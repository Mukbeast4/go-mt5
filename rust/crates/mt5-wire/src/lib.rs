//! Reverse-engineered MT5 pipe codec.
//!
//! This crate intentionally knows the MT5 binary protocol but knows nothing
//! about TCP, tokens, sessions, or trading policy.  Responses are consumed
//! incrementally, so the advertised MT5 response length is never used as an
//! allocation size.

use std::{collections::BTreeMap, fmt};

use bridge_protocol::{Operation, PayloadSchema, Value, value};
use bytes::{BufMut, BytesMut};
use thiserror::Error;
use tokio::io::{AsyncRead, AsyncReadExt, AsyncWrite, AsyncWriteExt};

pub const CMD_TERMINAL_INFO: u32 = 1;
pub const CMD_INITIALIZE: u32 = 4;
pub const CMD_COPY_TICKS_FROM: u32 = 104;
pub const CMD_COPY_TICKS_RANGE: u32 = 105;
pub const CMD_COPY_RATES_FROM: u32 = 106;
pub const CMD_COPY_RATES_RANGE: u32 = 107;
pub const CMD_COPY_RATES_FROM_POS: u32 = 108;
pub const CMD_POSITIONS_TOTAL: u32 = 120;
pub const CMD_POSITIONS_GET: u32 = 121;
pub const CMD_POSITIONS_GET_BY_SYMBOL: u32 = 122;
pub const CMD_POSITIONS_GET_BY_TICKET: u32 = 123;
pub const CMD_ORDERS_TOTAL: u32 = 130;
pub const CMD_ORDERS_GET: u32 = 131;
pub const CMD_ORDERS_GET_BY_SYMBOL: u32 = 132;
pub const CMD_ORDERS_GET_BY_TICKET: u32 = 133;
pub const CMD_HISTORY_ORDERS_TOTAL: u32 = 140;
pub const CMD_HISTORY_ORDERS_GET: u32 = 141;
pub const CMD_HISTORY_ORDERS_GET_SYMBOL: u32 = 142;
pub const CMD_HISTORY_ORDERS_GET_TICKET: u32 = 143;
pub const CMD_HISTORY_DEALS_TOTAL: u32 = 150;
pub const CMD_HISTORY_DEALS_GET: u32 = 151;
pub const CMD_HISTORY_DEALS_GET_SYMBOL: u32 = 152;
pub const CMD_HISTORY_DEALS_GET_TICKET: u32 = 153;
pub const CMD_ORDER_CHECK: u32 = 160;
pub const CMD_ORDER_SEND: u32 = 161;
pub const CMD_SYMBOL_INFO: u32 = 170;
pub const CMD_SYMBOL_SELECT: u32 = 171;
pub const CMD_SYMBOL_INFO_TICK: u32 = 172;
pub const CMD_SYMBOLS_TOTAL: u32 = 173;
pub const CMD_SYMBOLS_GET: u32 = 174;
pub const CMD_SYMBOLS_GET_BY_GROUP: u32 = 175;
pub const CMD_TERMINAL_INFO_FULL: u32 = 180;
pub const CMD_ACCOUNT_INFO: u32 = 190;
pub const CMD_MARKET_BOOK_ADD: u32 = 191;
pub const CMD_MARKET_BOOK_RELEASE: u32 = 192;
pub const CMD_MARKET_BOOK_GET: u32 = 193;
pub const CMD_ORDER_CALC_MARGIN: u32 = 202;
pub const CMD_ORDER_CALC_PROFIT: u32 = 203;

pub const RATE_RECORD_BYTES: usize = 60;
pub const TICK_RECORD_BYTES: usize = 60;
pub const SYMBOL_RECORD_BYTES: usize = 2993;
pub const POSITION_RECORD_BYTES: usize = 320;
pub const ORDER_RECORD_BYTES: usize = 340;
pub const DEAL_RECORD_BYTES: usize = 300;
pub const BOOK_RECORD_BYTES: usize = 32;

#[derive(Debug, Error)]
pub enum WireError {
    #[error("I/O error: {0}")]
    Io(#[from] std::io::Error),
    #[error("invalid MT5 response length {0}")]
    InvalidResponseLength(u32),
    #[error("MT5 command echo mismatch: requested {expected}, received {actual}")]
    CommandEcho { expected: u32, actual: u32 },
    #[error("MT5 response failed ({code}): {message}")]
    Remote { code: i32, message: String },
    #[error("invalid MT5 response: {0}")]
    InvalidResponse(String),
    #[error("invalid request parameter {0}")]
    InvalidParameter(String),
    #[error("the active pipe response was cancelled")]
    Cancelled,
    #[error("the caller-imposed request deadline expired")]
    DeadlineExceeded,
    #[error("unsupported bridge operation {0:?}")]
    Unsupported(Operation),
    #[error("missing request parameter {0}")]
    MissingParameter(&'static str),
    #[error("request parameter {0} has the wrong type")]
    ParameterType(&'static str),
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum ResponseKind {
    Unit,
    Small,
    Rates,
    Ticks,
    Symbols,
    Positions,
    Orders,
    Deals,
    Books,
}

impl ResponseKind {
    pub fn record_layout(self) -> Option<(PayloadSchema, usize)> {
        match self {
            Self::Rates => Some((PayloadSchema::RateV1, RATE_RECORD_BYTES)),
            Self::Ticks => Some((PayloadSchema::TickV1, TICK_RECORD_BYTES)),
            _ => None,
        }
    }

    pub fn value_record_bytes(self) -> Option<usize> {
        match self {
            Self::Symbols => Some(SYMBOL_RECORD_BYTES),
            Self::Positions => Some(POSITION_RECORD_BYTES),
            Self::Orders => Some(ORDER_RECORD_BYTES),
            Self::Deals => Some(DEAL_RECORD_BYTES),
            Self::Books => Some(BOOK_RECORD_BYTES),
            _ => None,
        }
    }
}

#[derive(Clone, Debug)]
pub struct WireRequest {
    pub command: u32,
    pub params: Vec<u8>,
    pub response_kind: ResponseKind,
    pub mutation: bool,
}

pub fn initialize_request() -> WireRequest {
    let mut writer = Writer::default();
    writer.u32(3);
    writer.string("Go");
    WireRequest {
        command: CMD_INITIALIZE,
        params: writer.finish(),
        response_kind: ResponseKind::Small,
        mutation: false,
    }
}

pub fn build_request(
    operation: Operation,
    params: Option<&Value>,
) -> Result<WireRequest, WireError> {
    let params = Params::new(params)?;
    let mut w = Writer::default();
    let request = match operation {
        Operation::Version => request(CMD_TERMINAL_INFO, w.finish(), ResponseKind::Small, false),
        Operation::AccountInfo => request(CMD_ACCOUNT_INFO, w.finish(), ResponseKind::Small, false),
        Operation::TerminalInfo => request(
            CMD_TERMINAL_INFO_FULL,
            w.finish(),
            ResponseKind::Small,
            false,
        ),
        Operation::SymbolsTotal => {
            request(CMD_SYMBOLS_TOTAL, w.finish(), ResponseKind::Small, false)
        }
        Operation::SymbolsGet => {
            if let Some(group) = params.optional_string("group")? {
                w.string(group);
                request(
                    CMD_SYMBOLS_GET_BY_GROUP,
                    w.finish(),
                    ResponseKind::Symbols,
                    false,
                )
            } else {
                request(CMD_SYMBOLS_GET, w.finish(), ResponseKind::Symbols, false)
            }
        }
        Operation::SymbolInfo => {
            w.string(params.string("symbol")?);
            request(CMD_SYMBOL_INFO, w.finish(), ResponseKind::Small, false)
        }
        Operation::SymbolInfoTick => {
            w.string(params.string("symbol")?);
            request(CMD_SYMBOL_INFO_TICK, w.finish(), ResponseKind::Small, false)
        }
        Operation::SymbolSelect => {
            w.string(params.string("symbol")?);
            w.bool1(params.bool("enable")?);
            request(CMD_SYMBOL_SELECT, w.finish(), ResponseKind::Unit, true)
        }
        Operation::CopyRatesFrom => {
            w.string(params.string("symbol")?);
            w.u32(params.u32("timeframe")?);
            w.i64(params.i64("date_from")?);
            w.u32(params.u32("count")?);
            request(CMD_COPY_RATES_FROM, w.finish(), ResponseKind::Rates, false)
        }
        Operation::CopyRatesFromPos => {
            w.string(params.string("symbol")?);
            w.u32(params.u32("timeframe")?);
            w.u32(params.u32("start_pos")?);
            w.u32(params.u32("count")?);
            request(
                CMD_COPY_RATES_FROM_POS,
                w.finish(),
                ResponseKind::Rates,
                false,
            )
        }
        Operation::CopyRatesRange => {
            w.string(params.string("symbol")?);
            w.u32(params.u32("timeframe")?);
            w.i64(params.i64("date_from")?);
            w.i64(params.i64("date_to")?);
            request(CMD_COPY_RATES_RANGE, w.finish(), ResponseKind::Rates, false)
        }
        Operation::CopyTicksFrom => {
            w.string(params.string("symbol")?);
            w.i64(params.i64("date_from")?);
            w.u32(params.u32("count")?);
            w.u32(params.u32("flags")?);
            request(CMD_COPY_TICKS_FROM, w.finish(), ResponseKind::Ticks, false)
        }
        Operation::CopyTicksRange => {
            w.string(params.string("symbol")?);
            w.i64(params.i64("date_from")?);
            w.i64(params.i64("date_to")?);
            w.u32(params.u32("flags")?);
            request(CMD_COPY_TICKS_RANGE, w.finish(), ResponseKind::Ticks, false)
        }
        Operation::PositionsTotal => {
            request(CMD_POSITIONS_TOTAL, w.finish(), ResponseKind::Small, false)
        }
        Operation::PositionsGet => filtered_request(
            &params,
            &mut w,
            CMD_POSITIONS_GET,
            CMD_POSITIONS_GET_BY_SYMBOL,
            CMD_POSITIONS_GET_BY_TICKET,
            ResponseKind::Positions,
        )?,
        Operation::OrdersTotal => request(CMD_ORDERS_TOTAL, w.finish(), ResponseKind::Small, false),
        Operation::OrdersGet => filtered_request(
            &params,
            &mut w,
            CMD_ORDERS_GET,
            CMD_ORDERS_GET_BY_SYMBOL,
            CMD_ORDERS_GET_BY_TICKET,
            ResponseKind::Orders,
        )?,
        Operation::HistoryOrdersTotal => {
            w.i64(params.i64("date_from")?);
            w.i64(params.i64("date_to")?);
            request(
                CMD_HISTORY_ORDERS_TOTAL,
                w.finish(),
                ResponseKind::Small,
                false,
            )
        }
        Operation::HistoryDealsTotal => {
            w.i64(params.i64("date_from")?);
            w.i64(params.i64("date_to")?);
            request(
                CMD_HISTORY_DEALS_TOTAL,
                w.finish(),
                ResponseKind::Small,
                false,
            )
        }
        Operation::HistoryOrdersGet => history_request(
            &params,
            &mut w,
            CMD_HISTORY_ORDERS_GET,
            CMD_HISTORY_ORDERS_GET_SYMBOL,
            CMD_HISTORY_ORDERS_GET_TICKET,
            ResponseKind::Orders,
        )?,
        Operation::HistoryDealsGet => history_request(
            &params,
            &mut w,
            CMD_HISTORY_DEALS_GET,
            CMD_HISTORY_DEALS_GET_SYMBOL,
            CMD_HISTORY_DEALS_GET_TICKET,
            ResponseKind::Deals,
        )?,
        Operation::OrderCheck => {
            encode_trade(&params, &mut w)?;
            request(CMD_ORDER_CHECK, w.finish(), ResponseKind::Small, false)
        }
        Operation::OrderSend => {
            encode_trade(&params, &mut w)?;
            request(CMD_ORDER_SEND, w.finish(), ResponseKind::Small, true)
        }
        Operation::OrderCalcMargin => {
            w.u32(params.u32("action")?);
            w.string(params.string("symbol")?);
            w.f64(params.f64("volume")?);
            w.f64(params.f64("price")?);
            request(
                CMD_ORDER_CALC_MARGIN,
                w.finish(),
                ResponseKind::Small,
                false,
            )
        }
        Operation::OrderCalcProfit => {
            w.u32(params.u32("action")?);
            w.string(params.string("symbol")?);
            w.f64(params.f64("volume")?);
            w.f64(params.f64("price_open")?);
            w.f64(params.f64("price_close")?);
            request(
                CMD_ORDER_CALC_PROFIT,
                w.finish(),
                ResponseKind::Small,
                false,
            )
        }
        // The runtime executes add/get/release as one finite request.
        Operation::MarketBookSnapshot => {
            w.string(params.string("symbol")?);
            request(CMD_MARKET_BOOK_GET, w.finish(), ResponseKind::Books, false)
        }
        Operation::BridgeStatus | Operation::Unspecified => {
            return Err(WireError::Unsupported(operation));
        }
    };
    Ok(request)
}

fn request(
    command: u32,
    params: Vec<u8>,
    response_kind: ResponseKind,
    mutation: bool,
) -> WireRequest {
    WireRequest {
        command,
        params,
        response_kind,
        mutation,
    }
}

fn filtered_request(
    params: &Params<'_>,
    w: &mut Writer,
    all: u32,
    by_symbol: u32,
    by_ticket: u32,
    response_kind: ResponseKind,
) -> Result<WireRequest, WireError> {
    if let Some(ticket) = params.optional_i64("ticket")? {
        w.i64(ticket);
        Ok(request(by_ticket, w.finish(), response_kind, false))
    } else if let Some(symbol) = params.optional_string("symbol")? {
        w.string(symbol);
        Ok(request(by_symbol, w.finish(), response_kind, false))
    } else {
        Ok(request(all, w.finish(), response_kind, false))
    }
}

fn history_request(
    params: &Params<'_>,
    w: &mut Writer,
    range: u32,
    by_symbol: u32,
    by_ticket: u32,
    response_kind: ResponseKind,
) -> Result<WireRequest, WireError> {
    if let Some(ticket) = params.optional_i64("ticket")? {
        w.i64(ticket);
        Ok(request(by_ticket, w.finish(), response_kind, false))
    } else {
        let from = params.i64_or("date_from", 0)?;
        let to = params.i64_or("date_to", 0)?;
        if let Some(symbol) = params.optional_string("symbol")? {
            w.i64(from);
            w.i64(to);
            w.string(symbol);
            Ok(request(by_symbol, w.finish(), response_kind, false))
        } else {
            w.i64(from);
            w.i64(to);
            Ok(request(range, w.finish(), response_kind, false))
        }
    }
}

fn encode_trade(params: &Params<'_>, w: &mut Writer) -> Result<(), WireError> {
    w.u32(params.u32("action")?);
    w.i64(params.i64_or("magic", 0)?);
    w.i64(params.i64_or("order", 0)?);
    w.fixed_string(params.string_or("symbol", "")?, 64)?;
    w.f64(params.f64_or("volume", 0.0)?);
    w.f64(params.f64_or("price", 0.0)?);
    w.f64(params.f64_or("stop_limit", 0.0)?);
    w.f64(params.f64_or("sl", 0.0)?);
    w.f64(params.f64_or("tp", 0.0)?);
    w.u64(params.u64_or("deviation", 0)?);
    w.u32(params.u32_or("type", 0)?);
    w.u32(params.u32_or("type_filling", 0)?);
    w.u32(params.u32_or("type_time", 0)?);
    w.i64(params.i64_or("expiration", 0)?);
    w.fixed_string(params.string_or("comment", "")?, 64)?;
    w.i64(params.i64_or("position", 0)?);
    w.i64(params.i64_or("position_by", 0)?);
    Ok(())
}

/// A serial, cancellable pipe protocol implementation.  `begin` writes one
/// complete request then reads only the response prefix; callers consume the
/// body through `read_chunk` and can therefore keep memory bounded.
pub struct Pipe<S> {
    io: S,
}

impl<S> Pipe<S> {
    pub fn new(io: S) -> Self {
        Self { io }
    }
    pub fn into_inner(self) -> S {
        self.io
    }
}

#[derive(Debug)]
pub struct ResponseHead {
    pub command: u32,
    pub success: bool,
    pub remaining: u64,
}

impl<S> Pipe<S>
where
    S: AsyncRead + AsyncWrite + Unpin,
{
    pub async fn begin(&mut self, request: &WireRequest) -> Result<ResponseHead, WireError> {
        let request_payload = 4_usize
            .checked_add(request.params.len())
            .ok_or_else(|| WireError::InvalidParameter("request too large".into()))?;
        let request_payload_u32 = u32::try_from(request_payload)
            .map_err(|_| WireError::InvalidParameter("request too large".into()))?;
        self.io.write_u32_le(request_payload_u32).await?;
        self.io.write_u32_le(request.command).await?;
        self.io.write_all(&request.params).await?;
        self.io.flush().await?;

        let payload_len = self.io.read_u32_le().await?;
        if payload_len < 8 {
            return Err(WireError::InvalidResponseLength(payload_len));
        }
        let command = self.io.read_u32_le().await?;
        let success = self.io.read_u32_le().await? != 0;
        if command != request.command {
            return Err(WireError::CommandEcho {
                expected: request.command,
                actual: command,
            });
        }
        Ok(ResponseHead {
            command,
            success,
            remaining: u64::from(payload_len - 8),
        })
    }

    pub async fn read_chunk(
        &mut self,
        head: &mut ResponseHead,
        max_bytes: usize,
    ) -> Result<Vec<u8>, WireError> {
        let n = usize::try_from(head.remaining.min(max_bytes as u64)).expect("bounded by usize");
        let mut bytes = vec![0_u8; n];
        self.io.read_exact(&mut bytes).await?;
        head.remaining -= n as u64;
        Ok(bytes)
    }

    pub async fn read_exact_body(
        &mut self,
        head: &mut ResponseHead,
        bytes: &mut [u8],
    ) -> Result<(), WireError> {
        if head.remaining < bytes.len() as u64 {
            return Err(WireError::InvalidResponse("truncated response body".into()));
        }
        self.io.read_exact(bytes).await?;
        head.remaining -= bytes.len() as u64;
        Ok(())
    }

    pub async fn discard(&mut self, head: &mut ResponseHead) -> Result<(), WireError> {
        while head.remaining > 0 {
            let _ = self.read_chunk(head, 16 * 1024).await?;
        }
        Ok(())
    }

    pub async fn read_small(
        &mut self,
        head: &mut ResponseHead,
        limit: usize,
    ) -> Result<Vec<u8>, WireError> {
        if head.remaining > limit as u64 {
            self.discard(head).await?;
            return Err(WireError::InvalidResponse(format!(
                "small response exceeded {limit} byte metadata limit"
            )));
        }
        self.read_chunk(head, limit).await
    }

    pub async fn read_remote_error(
        &mut self,
        head: &mut ResponseHead,
    ) -> Result<WireError, WireError> {
        let bytes = self.read_small(head, 64 * 1024).await?;
        let mut reader = Reader::new(&bytes);
        let code = reader.i32()?;
        let message = if reader.remaining() > 0 {
            reader.string()?
        } else {
            "request failed".to_owned()
        };
        reader.finish()?;
        Ok(WireError::Remote { code, message })
    }
}

pub fn array_count(
    head_remaining: u64,
    first_four_bytes: [u8; 4],
    record_bytes: usize,
) -> Result<u64, WireError> {
    let count = u64::from(u32::from_le_bytes(first_four_bytes));
    let expected = count
        .checked_mul(record_bytes as u64)
        .ok_or_else(|| WireError::InvalidResponse("record byte count overflow".into()))?;
    if head_remaining != expected {
        return Err(WireError::InvalidResponse(format!(
            "array count {count} with {record_bytes}-byte records does not match {head_remaining} response bytes"
        )));
    }
    Ok(count)
}

pub fn decode_small(operation: Operation, bytes: &[u8]) -> Result<Value, WireError> {
    let mut r = Reader::new(bytes);
    let result = match operation {
        Operation::Version => Value::object([
            ("version", Value::i64(r.i64()?)),
            ("build", Value::i64(r.i64()?)),
            ("build_date", Value::string(r.string()?)),
        ]),
        Operation::AccountInfo => decode_account(&mut r)?,
        Operation::TerminalInfo => decode_terminal(&mut r)?,
        Operation::SymbolsTotal
        | Operation::PositionsTotal
        | Operation::OrdersTotal
        | Operation::HistoryOrdersTotal
        | Operation::HistoryDealsTotal => {
            Value::object([("total", Value::u64(u64::from(r.u32()?)))])
        }
        Operation::SymbolInfo => decode_symbol(&mut r)?,
        Operation::SymbolInfoTick => {
            if bytes.is_empty() {
                return Ok(Value::object([("available", Value::bool(false))]));
            }
            decode_tick(&mut r)?
        }
        Operation::OrderCheck => decode_check_result(&mut r)?,
        Operation::OrderSend => decode_trade_result(&mut r)?,
        Operation::OrderCalcMargin => Value::object([("margin", Value::f64(r.f64()?))]),
        Operation::OrderCalcProfit => Value::object([("profit", Value::f64(r.f64()?))]),
        Operation::SymbolSelect => Value::object(Vec::<(String, Value)>::new()),
        _ => return Err(WireError::Unsupported(operation)),
    };
    r.finish()?;
    Ok(result)
}

pub fn decode_value_record(kind: ResponseKind, bytes: &[u8]) -> Result<Value, WireError> {
    let mut r = Reader::new(bytes);
    let result = match kind {
        ResponseKind::Symbols => decode_symbol(&mut r)?,
        ResponseKind::Positions => decode_position(&mut r)?,
        ResponseKind::Orders => decode_order(&mut r)?,
        ResponseKind::Deals => decode_deal(&mut r)?,
        ResponseKind::Books => decode_book(&mut r)?,
        _ => {
            return Err(WireError::InvalidResponse(
                "not a value-record response".into(),
            ));
        }
    };
    r.finish()?;
    Ok(result)
}

fn decode_account(r: &mut Reader<'_>) -> Result<Value, WireError> {
    let login = r.i64()?;
    let names = ["trade_mode", "leverage", "limit_orders", "margin_so_mode"];
    let mut fields: Vec<(String, Value)> = vec![("login".into(), Value::i64(login))];
    for name in names {
        fields.push((name.into(), Value::i64(i64::from(r.i32()?))));
    }
    for name in ["trade_allowed", "trade_expert"] {
        fields.push((name.into(), Value::bool(r.bool1()?)));
    }
    for name in ["margin_mode", "currency_digits"] {
        fields.push((name.into(), Value::i64(i64::from(r.i32()?))));
    }
    fields.push(("fifo_close".into(), Value::bool(r.bool1()?)));
    for name in [
        "balance",
        "credit",
        "profit",
        "equity",
        "margin",
        "free_margin",
        "margin_level",
        "margin_so_call",
        "margin_so_so",
        "margin_initial",
        "margin_maintenance",
        "assets",
        "liabilities",
        "commission_blocked",
    ] {
        fields.push((name.into(), Value::f64(r.f64()?)));
    }
    fields.push(("name".into(), Value::string(r.fixed_string(256)?)));
    fields.push(("server".into(), Value::string(r.fixed_string(128)?)));
    fields.push(("currency".into(), Value::string(r.fixed_string(64)?)));
    fields.push(("company".into(), Value::string(r.fixed_string(256)?)));
    Ok(object(fields))
}

fn decode_terminal(r: &mut Reader<'_>) -> Result<Value, WireError> {
    let mut fields: Vec<(String, Value)> = Vec::new();
    for name in [
        "community_account",
        "community_connection",
        "connected",
        "dlls_allowed",
        "trade_allowed",
        "tradeapi_disabled",
        "email_enabled",
        "ftp_enabled",
        "notifications_enabled",
        "mqid",
    ] {
        fields.push((name.into(), Value::bool(r.bool64()?)));
    }
    for name in ["build", "max_bars", "code_page", "ping_last"] {
        fields.push((name.into(), Value::i64(r.i64()?)));
    }
    for name in ["community_balance", "retransmission"] {
        fields.push((name.into(), Value::f64(r.f64()?)));
    }
    for name in [
        "company",
        "name",
        "language",
        "path",
        "data_path",
        "common_data_path",
    ] {
        fields.push((name.into(), Value::string(r.string()?)));
    }
    Ok(object(fields))
}

fn decode_symbol(r: &mut Reader<'_>) -> Result<Value, WireError> {
    let mut fields: Vec<(String, Value)> = Vec::new();
    fields.push(("custom".into(), Value::bool(r.bool1()?)));
    fields.push(("chart_mode".into(), Value::i64(i64::from(r.u32()?))));
    for name in ["select", "visible"] {
        fields.push((name.into(), Value::bool(r.bool1()?)));
    }
    for name in [
        "session_deals",
        "session_buy_orders",
        "session_sell_orders",
        "volume",
        "volume_high",
        "volume_low",
        "time",
    ] {
        fields.push((name.into(), Value::i64(r.i64()?)));
    }
    for name in ["digits", "spread"] {
        fields.push((name.into(), Value::i64(i64::from(r.u32()?))));
    }
    fields.push(("spread_float".into(), Value::bool(r.bool1()?)));
    for name in ["ticks_book_depth", "trade_calc_mode", "trade_mode"] {
        fields.push((name.into(), Value::i64(i64::from(r.u32()?))));
    }
    for name in ["start_time", "expiration_time"] {
        fields.push((name.into(), Value::i64(r.i64()?)));
    }
    for name in [
        "trade_stops_level",
        "trade_freeze_level",
        "trade_exe_mode",
        "swap_mode",
        "swap_rollover_3days",
    ] {
        fields.push((name.into(), Value::i64(i64::from(r.u32()?))));
    }
    fields.push(("margin_hedged_use_leg".into(), Value::bool(r.bool1()?)));
    for name in [
        "expiration_mode",
        "filling_mode",
        "order_mode",
        "order_gtc_mode",
        "option_mode",
        "option_right",
    ] {
        fields.push((name.into(), Value::i64(i64::from(r.u32()?))));
    }
    for name in [
        "bid",
        "bid_high",
        "bid_low",
        "ask",
        "ask_high",
        "ask_low",
        "last",
        "last_high",
        "last_low",
        "volume_real",
        "volume_high_real",
        "volume_low_real",
        "option_strike",
        "point",
        "trade_tick_value",
        "trade_tick_value_profit",
        "trade_tick_value_loss",
        "trade_tick_size",
        "trade_contract_size",
        "trade_accrued_interest",
        "trade_face_value",
        "trade_liquidity_rate",
        "volume_min",
        "volume_max",
        "volume_step",
        "volume_limit",
        "swap_long",
        "swap_short",
        "margin_initial",
        "margin_maintenance",
        "session_volume",
        "session_turnover",
        "session_interest",
        "session_buy_orders_volume",
        "session_sell_orders_volume",
        "session_open",
        "session_close",
        "session_aw",
        "session_price_settlement",
        "session_price_limit_min",
        "session_price_limit_max",
        "margin_hedged",
        "price_change",
        "price_volatility",
        "price_theoretical",
        "price_greeks_delta",
        "price_greeks_theta",
        "price_greeks_gamma",
        "price_greeks_vega",
        "price_greeks_rho",
        "price_greeks_omega",
        "price_sensitivity",
    ] {
        fields.push((name.into(), Value::f64(r.f64()?)));
    }
    for (name, width) in [
        ("basis", 64),
        ("category", 128),
        ("currency_base", 32),
        ("currency_profit", 32),
        ("currency_margin", 32),
        ("bank", 512),
        ("description", 64),
        ("exchange", 64),
        ("formula", 1024),
        ("isin", 32),
        ("page", 128),
        ("path", 256),
        ("symbol", 64),
    ] {
        fields.push((name.into(), Value::string(r.fixed_string(width)?)));
    }
    Ok(object(fields))
}

fn decode_tick(r: &mut Reader<'_>) -> Result<Value, WireError> {
    Ok(object([
        ("time", Value::i64(r.i64()?)),
        ("bid", Value::f64(r.f64()?)),
        ("ask", Value::f64(r.f64()?)),
        ("last", Value::f64(r.f64()?)),
        ("volume", Value::u64(r.u64()?)),
        ("time_msc", Value::i64(r.i64()?)),
        ("flags", Value::u64(u64::from(r.u32()?))),
        ("volume_real", Value::f64(r.f64()?)),
    ]))
}

fn decode_position(r: &mut Reader<'_>) -> Result<Value, WireError> {
    let mut fields: Vec<(String, Value)> = Vec::new();
    for name in [
        "ticket",
        "time",
        "time_msc",
        "time_update",
        "time_update_msc",
    ] {
        fields.push((name.into(), Value::i64(r.i64()?)));
    }
    fields.push(("type".into(), Value::u64(u64::from(r.u32()?))));
    for name in ["magic", "identifier"] {
        fields.push((name.into(), Value::i64(r.i64()?)));
    }
    fields.push(("reason".into(), Value::u64(u64::from(r.u32()?))));
    for name in [
        "volume",
        "price_open",
        "sl",
        "tp",
        "price_current",
        "commission",
        "swap",
        "profit",
    ] {
        fields.push((name.into(), Value::f64(r.f64()?)));
    }
    for name in ["symbol", "comment", "external_id"] {
        fields.push((name.into(), Value::string(r.fixed_string(64)?)));
    }
    Ok(object(fields))
}

fn decode_order(r: &mut Reader<'_>) -> Result<Value, WireError> {
    let mut fields: Vec<(String, Value)> = Vec::new();
    for name in [
        "ticket",
        "time_setup",
        "time_setup_msc",
        "time_done",
        "time_done_msc",
        "time_expiration",
    ] {
        fields.push((name.into(), Value::i64(r.i64()?)));
    }
    for name in ["type", "type_time", "type_filling", "state"] {
        fields.push((name.into(), Value::u64(u64::from(r.u32()?))));
    }
    for name in ["magic", "position_id", "position_by_id"] {
        fields.push((name.into(), Value::i64(r.i64()?)));
    }
    fields.push(("reason".into(), Value::u64(u64::from(r.u32()?))));
    for name in [
        "volume_initial",
        "volume_current",
        "price_open",
        "price_current",
        "sl",
        "tp",
        "price_stop_limit",
    ] {
        fields.push((name.into(), Value::f64(r.f64()?)));
    }
    for name in ["symbol", "comment", "external_id"] {
        fields.push((name.into(), Value::string(r.fixed_string(64)?)));
    }
    Ok(object(fields))
}

fn decode_deal(r: &mut Reader<'_>) -> Result<Value, WireError> {
    let mut fields: Vec<(String, Value)> = Vec::new();
    for name in ["ticket", "order", "time", "time_msc"] {
        fields.push((name.into(), Value::i64(r.i64()?)));
    }
    for name in ["type", "entry"] {
        fields.push((name.into(), Value::u64(u64::from(r.u32()?))));
    }
    for name in ["magic", "position_id"] {
        fields.push((name.into(), Value::i64(r.i64()?)));
    }
    fields.push(("reason".into(), Value::u64(u64::from(r.u32()?))));
    for name in ["volume", "price", "commission", "swap", "profit", "fee"] {
        fields.push((name.into(), Value::f64(r.f64()?)));
    }
    for name in ["symbol", "comment", "external_id"] {
        fields.push((name.into(), Value::string(r.fixed_string(64)?)));
    }
    Ok(object(fields))
}

fn decode_book(r: &mut Reader<'_>) -> Result<Value, WireError> {
    Ok(object([
        ("type", Value::i64(r.i64()?)),
        ("price", Value::f64(r.f64()?)),
        ("volume", Value::i64(r.i64()?)),
        ("volume_real", Value::f64(r.f64()?)),
    ]))
}

fn decode_check_result(r: &mut Reader<'_>) -> Result<Value, WireError> {
    let mut fields: Vec<(String, Value)> = Vec::new();
    fields.push(("retcode".into(), Value::u64(u64::from(r.u32()?))));
    for name in [
        "balance",
        "equity",
        "profit",
        "margin",
        "margin_free",
        "margin_level",
    ] {
        fields.push((name.into(), Value::f64(r.f64()?)));
    }
    fields.push(("comment".into(), Value::string(r.fixed_string(200)?)));
    Ok(object(fields))
}

fn decode_trade_result(r: &mut Reader<'_>) -> Result<Value, WireError> {
    let mut fields: Vec<(String, Value)> = Vec::new();
    fields.push(("retcode".into(), Value::u64(u64::from(r.u32()?))));
    fields.push(("deal".into(), Value::i64(r.i64()?)));
    fields.push(("order".into(), Value::i64(r.i64()?)));
    for name in ["volume", "price", "bid", "ask"] {
        fields.push((name.into(), Value::f64(r.f64()?)));
    }
    fields.push(("comment".into(), Value::string(r.fixed_string(200)?)));
    fields.push(("request_id".into(), Value::u64(u64::from(r.u32()?))));
    fields.push(("retcode_external".into(), Value::i64(i64::from(r.i32()?))));
    Ok(object(fields))
}

fn object<I, S>(fields: I) -> Value
where
    I: IntoIterator<Item = (S, Value)>,
    S: Into<String>,
{
    Value::object(fields)
}

#[derive(Default)]
struct Writer {
    bytes: BytesMut,
}

impl Writer {
    fn u32(&mut self, value: u32) {
        self.bytes.put_u32_le(value);
    }
    fn u64(&mut self, value: u64) {
        self.bytes.put_u64_le(value);
    }
    fn i64(&mut self, value: i64) {
        self.bytes.put_i64_le(value);
    }
    fn f64(&mut self, value: f64) {
        self.bytes.put_u64_le(value.to_bits());
    }
    fn bool1(&mut self, value: bool) {
        self.bytes.put_u8(u8::from(value));
    }
    fn string(&mut self, value: &str) {
        let utf16: Vec<u16> = value.encode_utf16().collect();
        self.u32(u32::try_from(utf16.len()).expect("string length is bounded by memory"));
        for unit in utf16 {
            self.bytes.put_u16_le(unit);
        }
    }
    fn fixed_string(&mut self, value: &str, byte_width: usize) -> Result<(), WireError> {
        let utf16: Vec<u16> = value.encode_utf16().collect();
        let max_units = byte_width
            .checked_div(2)
            .and_then(|n| n.checked_sub(1))
            .ok_or_else(|| WireError::InvalidParameter("invalid fixed string width".into()))?;
        if utf16.len() > max_units {
            return Err(WireError::InvalidParameter(format!(
                "UTF-16 string exceeds {max_units} code units"
            )));
        }
        let start = self.bytes.len();
        self.bytes.resize(start + byte_width, 0);
        for (index, unit) in utf16.iter().enumerate() {
            self.bytes[start + index * 2..start + index * 2 + 2]
                .copy_from_slice(&unit.to_le_bytes());
        }
        Ok(())
    }
    fn finish(&mut self) -> Vec<u8> {
        std::mem::take(&mut self.bytes).to_vec()
    }
}

struct Reader<'a> {
    bytes: &'a [u8],
    position: usize,
}

impl<'a> Reader<'a> {
    fn new(bytes: &'a [u8]) -> Self {
        Self { bytes, position: 0 }
    }
    fn remaining(&self) -> usize {
        self.bytes.len().saturating_sub(self.position)
    }
    fn take(&mut self, count: usize) -> Result<&'a [u8], WireError> {
        let end = self
            .position
            .checked_add(count)
            .ok_or_else(|| WireError::InvalidResponse("cursor overflow".into()))?;
        let result = self
            .bytes
            .get(self.position..end)
            .ok_or_else(|| WireError::InvalidResponse("unexpected end of response".into()))?;
        self.position = end;
        Ok(result)
    }
    fn bool1(&mut self) -> Result<bool, WireError> {
        Ok(self.take(1)?[0] != 0)
    }
    fn bool64(&mut self) -> Result<bool, WireError> {
        Ok(self.i64()? != 0)
    }
    fn u32(&mut self) -> Result<u32, WireError> {
        Ok(u32::from_le_bytes(self.take(4)?.try_into().unwrap()))
    }
    fn i32(&mut self) -> Result<i32, WireError> {
        Ok(self.u32()? as i32)
    }
    fn u64(&mut self) -> Result<u64, WireError> {
        Ok(u64::from_le_bytes(self.take(8)?.try_into().unwrap()))
    }
    fn i64(&mut self) -> Result<i64, WireError> {
        Ok(self.u64()? as i64)
    }
    fn f64(&mut self) -> Result<f64, WireError> {
        Ok(f64::from_bits(self.u64()?))
    }
    fn string(&mut self) -> Result<String, WireError> {
        let units = usize::try_from(self.u32()?)
            .map_err(|_| WireError::InvalidResponse("string length overflow".into()))?;
        let raw = self.take(
            units
                .checked_mul(2)
                .ok_or_else(|| WireError::InvalidResponse("string length overflow".into()))?,
        )?;
        Ok(decode_utf16(raw))
    }
    fn fixed_string(&mut self, width: usize) -> Result<String, WireError> {
        if !width.is_multiple_of(2) {
            return Err(WireError::InvalidResponse("odd fixed string width".into()));
        }
        let raw = self.take(width)?;
        let end = raw
            .chunks_exact(2)
            .position(|u| u == [0, 0])
            .map(|n| n * 2)
            .unwrap_or(width);
        Ok(decode_utf16(&raw[..end]))
    }
    fn finish(&self) -> Result<(), WireError> {
        if self.remaining() == 0 {
            Ok(())
        } else {
            Err(WireError::InvalidResponse(format!(
                "{} trailing response bytes",
                self.remaining()
            )))
        }
    }
}

fn decode_utf16(bytes: &[u8]) -> String {
    let units: Vec<u16> = bytes
        .chunks_exact(2)
        .map(|unit| u16::from_le_bytes([unit[0], unit[1]]))
        .collect();
    String::from_utf16_lossy(&units)
}

struct Params<'a> {
    fields: BTreeMap<&'a str, &'a Value>,
}

impl<'a> Params<'a> {
    fn new(value: Option<&'a Value>) -> Result<Self, WireError> {
        match value {
            None => Ok(Self {
                fields: BTreeMap::new(),
            }),
            Some(value) => Ok(Self {
                fields: value
                    .as_object()
                    .ok_or(WireError::ParameterType("params"))?,
            }),
        }
    }
    fn value(&self, name: &'static str) -> Result<&Value, WireError> {
        self.fields
            .get(name)
            .copied()
            .ok_or(WireError::MissingParameter(name))
    }
    fn optional(&self, name: &'static str) -> Option<&Value> {
        self.fields.get(name).copied()
    }
    fn string(&self, name: &'static str) -> Result<&str, WireError> {
        match self.value(name)?.kind.as_ref() {
            Some(value::Kind::String(value)) => Ok(value),
            _ => Err(WireError::ParameterType(name)),
        }
    }
    fn string_or(&self, name: &'static str, default: &'static str) -> Result<&str, WireError> {
        self.optional_string(name).map(|v| v.unwrap_or(default))
    }
    fn optional_string(&self, name: &'static str) -> Result<Option<&str>, WireError> {
        self.optional(name)
            .map(|v| match v.kind.as_ref() {
                Some(value::Kind::String(value)) => Ok(value.as_str()),
                _ => Err(WireError::ParameterType(name)),
            })
            .transpose()
    }
    fn bool(&self, name: &'static str) -> Result<bool, WireError> {
        match self.value(name)?.kind {
            Some(value::Kind::Bool(value)) => Ok(value),
            _ => Err(WireError::ParameterType(name)),
        }
    }
    fn i64(&self, name: &'static str) -> Result<i64, WireError> {
        numeric_i64(self.value(name)?, name)
    }
    fn i64_or(&self, name: &'static str, default: i64) -> Result<i64, WireError> {
        self.optional(name)
            .map(|v| numeric_i64(v, name))
            .transpose()
            .map(|v| v.unwrap_or(default))
    }
    fn optional_i64(&self, name: &'static str) -> Result<Option<i64>, WireError> {
        self.optional(name)
            .map(|v| numeric_i64(v, name))
            .transpose()
    }
    fn u64_or(&self, name: &'static str, default: u64) -> Result<u64, WireError> {
        self.optional(name)
            .map(|v| numeric_u64(v, name))
            .transpose()
            .map(|v| v.unwrap_or(default))
    }
    fn u32(&self, name: &'static str) -> Result<u32, WireError> {
        u32::try_from(numeric_u64(self.value(name)?, name)?)
            .map_err(|_| WireError::InvalidParameter(name.into()))
    }
    fn u32_or(&self, name: &'static str, default: u32) -> Result<u32, WireError> {
        u32::try_from(self.u64_or(name, u64::from(default))?)
            .map_err(|_| WireError::InvalidParameter(name.into()))
    }
    fn f64(&self, name: &'static str) -> Result<f64, WireError> {
        numeric_f64(self.value(name)?, name)
    }
    fn f64_or(&self, name: &'static str, default: f64) -> Result<f64, WireError> {
        self.optional(name)
            .map(|v| numeric_f64(v, name))
            .transpose()
            .map(|v| v.unwrap_or(default))
    }
}

fn numeric_i64(value: &Value, name: &'static str) -> Result<i64, WireError> {
    match value.kind {
        Some(value::Kind::I64(v)) => Ok(v),
        Some(value::Kind::U64(v)) => {
            i64::try_from(v).map_err(|_| WireError::InvalidParameter(name.into()))
        }
        _ => Err(WireError::ParameterType(name)),
    }
}
fn numeric_u64(value: &Value, name: &'static str) -> Result<u64, WireError> {
    match value.kind {
        Some(value::Kind::U64(v)) => Ok(v),
        Some(value::Kind::I64(v)) if v >= 0 => Ok(v as u64),
        _ => Err(WireError::ParameterType(name)),
    }
}
fn numeric_f64(value: &Value, name: &'static str) -> Result<f64, WireError> {
    match value.kind {
        Some(value::Kind::F64(v)) if v.is_finite() => Ok(v),
        _ => Err(WireError::ParameterType(name)),
    }
}

impl fmt::Display for ResponseKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{self:?}")
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::PathBuf;

    fn fixture(name: &str) -> Vec<u8> {
        let root = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../..");
        std::fs::read(root.join("testdata").join(name)).unwrap()
    }

    #[test]
    fn initialize_matches_go_reference() {
        let request = initialize_request();
        assert_eq!(request.command, CMD_INITIALIZE);
        assert_eq!(&request.params[..4], &3_u32.to_le_bytes());
        assert_eq!(
            u32::from_le_bytes(request.params[4..8].try_into().unwrap()),
            2
        );
        assert_eq!(&request.params[8..], b"G\0o\0");
    }

    #[test]
    fn rate_fixture_has_exact_native_layout() {
        let bytes = fixture("rates_h1_50_eurusd.bin");
        let count = u32::from_le_bytes(bytes[..4].try_into().unwrap());
        assert_eq!(count, 50);
        assert_eq!(bytes.len() - 4, count as usize * RATE_RECORD_BYTES);
    }

    #[test]
    fn tick_fixture_has_exact_native_layout() {
        let bytes = fixture("ticks_100_eurusd.bin");
        let count = u32::from_le_bytes(bytes[..4].try_into().unwrap());
        assert_eq!(count, 100);
        assert_eq!(bytes.len() - 4, count as usize * TICK_RECORD_BYTES);
    }

    #[test]
    fn symbol_fixture_decodes_every_field_and_consumes_record() {
        let bytes = fixture("symbol_info_eurusd.bin");
        let value = decode_small(Operation::SymbolInfo, &bytes[8..]).unwrap();
        let fields = value.as_object().unwrap();
        assert!(fields.contains_key("symbol"));
        assert!(fields.contains_key("price_greeks_omega"));
        assert!(matches!(fields["symbol"].kind, Some(value::Kind::String(ref s)) if s == "EURUSD"));
    }

    #[test]
    fn trade_request_rejects_fixed_string_truncation() {
        let value = Value::object([
            ("action", Value::u64(1)),
            ("symbol", Value::string("x".repeat(32))),
        ]);
        assert!(matches!(
            build_request(Operation::OrderSend, Some(&value)),
            Err(WireError::InvalidParameter(_))
        ));
    }

    #[test]
    fn invalid_array_count_is_rejected_without_allocating_records() {
        assert!(array_count(10, 1_u32.to_le_bytes(), RATE_RECORD_BYTES).is_err());
    }
}
