//! Versioned TCP framing and control messages for the MT5 bridge.
//!
//! The protocol deliberately has a small fixed frame header.  A result may
//! consist of arbitrarily many `ResponseChunk` frames with the same request
//! id; frame limits are allocation limits, not result limits.

use std::collections::BTreeMap;

use bytes::{Buf, BufMut, Bytes, BytesMut};
use prost::Message;
use thiserror::Error;
use tokio::io::{AsyncRead, AsyncReadExt, AsyncWrite, AsyncWriteExt};

pub const MAJOR_VERSION: u16 = 1;
pub const HEADER_AFTER_LENGTH: usize = 20;
pub const HEADER_LEN: usize = 4 + HEADER_AFTER_LENGTH;
pub const MAX_FRAME_LENGTH: usize = 1024 * 1024;
pub const MAX_METADATA_LENGTH: usize = 64 * 1024;
pub const DEFAULT_CHUNK_BYTES: usize = 256 * 1024;
pub const DEFAULT_CREDIT_BYTES: u64 = 1024 * 1024;
pub const CAPABILITY_RAW_RATE_V1: u64 = 1 << 0;
pub const CAPABILITY_RAW_TICK_V1: u64 = 1 << 1;
pub const CAPABILITY_RESPONSE_CREDIT: u64 = 1 << 2;
pub const CAPABILITY_REQUEST_CANCELLATION: u64 = 1 << 3;
pub const DEFAULT_CAPABILITIES: u64 = CAPABILITY_RAW_RATE_V1
    | CAPABILITY_RAW_TICK_V1
    | CAPABILITY_RESPONSE_CREDIT
    | CAPABILITY_REQUEST_CANCELLATION;

#[derive(Debug, Error)]
pub enum ProtocolError {
    #[error("I/O error: {0}")]
    Io(#[from] std::io::Error),
    #[error("invalid frame length {0}")]
    InvalidFrameLength(u32),
    #[error("invalid metadata length {metadata} for frame body {body}")]
    InvalidMetadataLength { metadata: u32, body: usize },
    #[error("unsupported protocol version {0}")]
    UnsupportedVersion(u16),
    #[error("unknown message type {0}")]
    UnknownMessageType(u16),
    #[error("nonzero frame flags are not supported in protocol v1: {0:#x}")]
    UnsupportedFlags(u32),
    #[error("metadata is too large: {0} bytes")]
    MetadataTooLarge(usize),
    #[error("protobuf decode failed: {0}")]
    Decode(#[from] prost::DecodeError),
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
#[repr(u16)]
pub enum MessageType {
    Hello = 1,
    HelloAck = 2,
    Request = 3,
    Response = 4,
    Error = 5,
    ResponseStart = 6,
    ResponseChunk = 7,
    ResponseEnd = 8,
    Cancel = 9,
    WindowUpdate = 10,
    Ping = 11,
    Pong = 12,
}

impl TryFrom<u16> for MessageType {
    type Error = ProtocolError;

    fn try_from(value: u16) -> Result<Self, ProtocolError> {
        use MessageType::*;
        match value {
            1 => Ok(Hello),
            2 => Ok(HelloAck),
            3 => Ok(Request),
            4 => Ok(Response),
            5 => Ok(Error),
            6 => Ok(ResponseStart),
            7 => Ok(ResponseChunk),
            8 => Ok(ResponseEnd),
            9 => Ok(Cancel),
            10 => Ok(WindowUpdate),
            11 => Ok(Ping),
            12 => Ok(Pong),
            _ => Err(ProtocolError::UnknownMessageType(value)),
        }
    }
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Frame {
    pub message_type: MessageType,
    pub flags: u32,
    pub request_id: u64,
    pub metadata: Bytes,
    pub payload: Bytes,
}

impl Frame {
    pub fn new(message_type: MessageType, request_id: u64, metadata: impl Into<Bytes>) -> Self {
        Self {
            message_type,
            flags: 0,
            request_id,
            metadata: metadata.into(),
            payload: Bytes::new(),
        }
    }

    pub fn with_payload(mut self, payload: impl Into<Bytes>) -> Self {
        self.payload = payload.into();
        self
    }

    pub async fn read_from<R>(reader: &mut R) -> Result<Self, ProtocolError>
    where
        R: AsyncRead + Unpin,
    {
        let frame_length = reader.read_u32_le().await?;
        let body_len = usize::try_from(frame_length)
            .map_err(|_| ProtocolError::InvalidFrameLength(frame_length))?;
        if !(HEADER_AFTER_LENGTH..=MAX_FRAME_LENGTH).contains(&body_len) {
            return Err(ProtocolError::InvalidFrameLength(frame_length));
        }

        let mut header = [0_u8; HEADER_AFTER_LENGTH];
        reader.read_exact(&mut header).await?;
        let mut header = &header[..];
        let version = header.get_u16_le();
        if version != MAJOR_VERSION {
            return Err(ProtocolError::UnsupportedVersion(version));
        }
        let message_type = MessageType::try_from(header.get_u16_le())?;
        let flags = header.get_u32_le();
        if flags != 0 {
            return Err(ProtocolError::UnsupportedFlags(flags));
        }
        let request_id = header.get_u64_le();
        let metadata_length = header.get_u32_le();
        let metadata_len =
            usize::try_from(metadata_length).map_err(|_| ProtocolError::InvalidMetadataLength {
                metadata: metadata_length,
                body: body_len,
            })?;
        let payload_len = body_len - HEADER_AFTER_LENGTH;
        if metadata_len > MAX_METADATA_LENGTH || metadata_len > payload_len {
            return Err(ProtocolError::InvalidMetadataLength {
                metadata: metadata_length,
                body: body_len,
            });
        }

        let mut body = vec![0_u8; payload_len];
        reader.read_exact(&mut body).await?;
        let (metadata, payload) = body.split_at(metadata_len);
        Ok(Self {
            message_type,
            flags,
            request_id,
            metadata: Bytes::copy_from_slice(metadata),
            payload: Bytes::copy_from_slice(payload),
        })
    }

    pub async fn write_to<W>(&self, writer: &mut W) -> Result<(), ProtocolError>
    where
        W: AsyncWrite + Unpin,
    {
        if self.metadata.len() > MAX_METADATA_LENGTH {
            return Err(ProtocolError::MetadataTooLarge(self.metadata.len()));
        }
        let body_len = HEADER_AFTER_LENGTH
            .checked_add(self.metadata.len())
            .and_then(|n| n.checked_add(self.payload.len()))
            .ok_or(ProtocolError::InvalidFrameLength(u32::MAX))?;
        if body_len > MAX_FRAME_LENGTH {
            return Err(ProtocolError::InvalidFrameLength(
                u32::try_from(body_len).unwrap_or(u32::MAX),
            ));
        }

        let mut encoded = BytesMut::with_capacity(4 + body_len);
        encoded.put_u32_le(
            u32::try_from(body_len).map_err(|_| ProtocolError::InvalidFrameLength(u32::MAX))?,
        );
        encoded.put_u16_le(MAJOR_VERSION);
        encoded.put_u16_le(self.message_type as u16);
        encoded.put_u32_le(self.flags);
        encoded.put_u64_le(self.request_id);
        encoded.put_u32_le(
            u32::try_from(self.metadata.len())
                .map_err(|_| ProtocolError::MetadataTooLarge(self.metadata.len()))?,
        );
        encoded.extend_from_slice(&self.metadata);
        encoded.extend_from_slice(&self.payload);
        writer.write_all(&encoded).await?;
        writer.flush().await?;
        Ok(())
    }

    pub fn encode_message<M: Message>(
        message_type: MessageType,
        request_id: u64,
        message: &M,
    ) -> Result<Self, ProtocolError> {
        let mut metadata = Vec::with_capacity(message.encoded_len());
        message
            .encode(&mut metadata)
            .expect("Vec has enough capacity");
        if metadata.len() > MAX_METADATA_LENGTH {
            return Err(ProtocolError::MetadataTooLarge(metadata.len()));
        }
        Ok(Self::new(message_type, request_id, metadata))
    }

    pub fn decode_message<M: Message + Default>(&self) -> Result<M, ProtocolError> {
        Ok(M::decode(self.metadata.as_ref())?)
    }
}

/// A protobuf value tree keeps the wire API typed without exposing MT5 command
/// ids or MT5 request byte layouts to backend clients.
#[derive(Clone, PartialEq, Message)]
pub struct Value {
    #[prost(oneof = "value::Kind", tags = "1, 2, 3, 4, 5, 6, 7, 8")]
    pub kind: Option<value::Kind>,
}

pub mod value {
    use super::{ValueList, ValueObject};
    use prost::Oneof;

    #[derive(Clone, PartialEq, Oneof)]
    pub enum Kind {
        #[prost(bool, tag = "1")]
        Bool(bool),
        #[prost(sint64, tag = "2")]
        I64(i64),
        #[prost(uint64, tag = "3")]
        U64(u64),
        #[prost(double, tag = "4")]
        F64(f64),
        #[prost(string, tag = "5")]
        String(String),
        #[prost(bytes, tag = "6")]
        Bytes(Vec<u8>),
        #[prost(message, tag = "7")]
        List(ValueList),
        #[prost(message, tag = "8")]
        Object(ValueObject),
    }
}

#[derive(Clone, PartialEq, Message)]
pub struct ValueList {
    #[prost(message, repeated, tag = "1")]
    pub values: Vec<Value>,
}

#[derive(Clone, PartialEq, Message)]
pub struct ValueField {
    #[prost(string, tag = "1")]
    pub name: String,
    #[prost(message, optional, tag = "2")]
    pub value: Option<Value>,
}

#[derive(Clone, PartialEq, Message)]
pub struct ValueObject {
    #[prost(message, repeated, tag = "1")]
    pub fields: Vec<ValueField>,
}

impl Value {
    pub fn object<I, S>(fields: I) -> Self
    where
        I: IntoIterator<Item = (S, Value)>,
        S: Into<String>,
    {
        Self {
            kind: Some(value::Kind::Object(ValueObject {
                fields: fields
                    .into_iter()
                    .map(|(name, value)| ValueField {
                        name: name.into(),
                        value: Some(value),
                    })
                    .collect(),
            })),
        }
    }

    pub fn string(value: impl Into<String>) -> Self {
        Self {
            kind: Some(value::Kind::String(value.into())),
        }
    }

    pub fn i64(value: i64) -> Self {
        Self {
            kind: Some(value::Kind::I64(value)),
        }
    }

    pub fn u64(value: u64) -> Self {
        Self {
            kind: Some(value::Kind::U64(value)),
        }
    }

    pub fn f64(value: f64) -> Self {
        Self {
            kind: Some(value::Kind::F64(value)),
        }
    }

    pub fn bool(value: bool) -> Self {
        Self {
            kind: Some(value::Kind::Bool(value)),
        }
    }

    pub fn list(values: Vec<Value>) -> Self {
        Self {
            kind: Some(value::Kind::List(ValueList { values })),
        }
    }

    pub fn as_object(&self) -> Option<BTreeMap<&str, &Value>> {
        let value::Kind::Object(object) = self.kind.as_ref()? else {
            return None;
        };
        Some(
            object
                .fields
                .iter()
                .filter_map(|f| f.value.as_ref().map(|v| (f.name.as_str(), v)))
                .collect(),
        )
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, prost::Enumeration)]
#[repr(i32)]
pub enum Operation {
    Unspecified = 0,
    BridgeStatus = 1,
    Version = 2,
    AccountInfo = 3,
    TerminalInfo = 4,
    SymbolsTotal = 5,
    SymbolsGet = 6,
    SymbolInfo = 7,
    SymbolInfoTick = 8,
    SymbolSelect = 9,
    CopyRatesFrom = 10,
    CopyRatesFromPos = 11,
    CopyRatesRange = 12,
    CopyTicksFrom = 13,
    CopyTicksRange = 14,
    PositionsTotal = 15,
    PositionsGet = 16,
    OrdersTotal = 17,
    OrdersGet = 18,
    HistoryOrdersTotal = 19,
    HistoryOrdersGet = 20,
    HistoryDealsTotal = 21,
    HistoryDealsGet = 22,
    OrderCheck = 23,
    OrderSend = 24,
    OrderCalcMargin = 25,
    OrderCalcProfit = 26,
    MarketBookSnapshot = 27,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, prost::Enumeration)]
#[repr(i32)]
pub enum PayloadSchema {
    Unspecified = 0,
    ProtoValues = 1,
    RateV1 = 2,
    TickV1 = 3,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, prost::Enumeration)]
#[repr(i32)]
pub enum ExecutionCertainty {
    NotDispatched = 0,
    ResultReceived = 1,
    OutcomeUnknown = 2,
}

#[derive(Clone, PartialEq, Message)]
pub struct Hello {
    #[prost(string, tag = "1")]
    pub client_id: String,
    #[prost(bytes, tag = "2")]
    pub token: Vec<u8>,
}

#[derive(Clone, PartialEq, Message)]
pub struct HelloAck {
    #[prost(bytes, tag = "1")]
    pub bridge_instance_id: Vec<u8>,
    #[prost(bytes, tag = "2")]
    pub session_id: Vec<u8>,
    #[prost(uint64, tag = "3")]
    pub terminal_epoch: u64,
    #[prost(string, tag = "4")]
    pub terminal_state: String,
    #[prost(uint32, tag = "5")]
    pub terminal_build: u32,
    #[prost(int64, tag = "6")]
    pub account_login: i64,
    #[prost(string, tag = "7")]
    pub account_server: String,
    #[prost(uint32, tag = "8")]
    pub max_frame_length: u32,
    #[prost(uint32, tag = "9")]
    pub max_metadata_length: u32,
    #[prost(uint32, tag = "10")]
    pub target_chunk_bytes: u32,
    #[prost(uint64, tag = "11")]
    pub initial_response_credit: u64,
    #[prost(uint64, tag = "12")]
    pub capabilities: u64,
}

#[derive(Clone, PartialEq, Message)]
pub struct Request {
    #[prost(enumeration = "Operation", tag = "1")]
    pub operation: i32,
    #[prost(uint64, tag = "2")]
    pub expected_terminal_epoch: u64,
    #[prost(message, optional, tag = "3")]
    pub params: Option<Value>,
    /// Zero means no caller-imposed deadline.
    #[prost(uint64, tag = "4")]
    pub deadline_ms: u64,
}

#[derive(Clone, PartialEq, Message)]
pub struct Response {
    #[prost(enumeration = "Operation", tag = "1")]
    pub operation: i32,
    #[prost(message, optional, tag = "2")]
    pub result: Option<Value>,
}

#[derive(Clone, PartialEq, Message)]
pub struct ResponseStart {
    #[prost(enumeration = "Operation", tag = "1")]
    pub operation: i32,
    #[prost(enumeration = "PayloadSchema", tag = "2")]
    pub schema: i32,
    #[prost(bool, tag = "3")]
    pub total_rows_known: bool,
    #[prost(uint64, tag = "4")]
    pub total_rows: u64,
}

#[derive(Clone, PartialEq, Message)]
pub struct ResponseChunk {
    #[prost(uint64, tag = "1")]
    pub sequence: u64,
    #[prost(uint64, tag = "2")]
    pub row_offset: u64,
    #[prost(uint64, tag = "3")]
    pub row_count: u64,
    #[prost(message, repeated, tag = "4")]
    pub records: Vec<Value>,
}

#[derive(Clone, PartialEq, Message)]
pub struct ResponseEnd {
    #[prost(bool, tag = "1")]
    pub success: bool,
    #[prost(uint64, tag = "2")]
    pub delivered_rows: u64,
    #[prost(enumeration = "ExecutionCertainty", tag = "3")]
    pub certainty: i32,
    #[prost(message, optional, tag = "4")]
    pub error: Option<ErrorMessage>,
}

#[derive(Clone, PartialEq, Message)]
pub struct ErrorMessage {
    #[prost(string, tag = "1")]
    pub origin: String,
    #[prost(string, tag = "2")]
    pub code: String,
    #[prost(string, tag = "3")]
    pub operation: String,
    #[prost(string, tag = "4")]
    pub message: String,
    #[prost(int64, tag = "5")]
    pub native_code: i64,
    #[prost(enumeration = "ExecutionCertainty", tag = "6")]
    pub certainty: i32,
}

#[derive(Clone, PartialEq, Message)]
pub struct Cancel {
    #[prost(string, tag = "1")]
    pub reason: String,
}

#[derive(Clone, PartialEq, Message)]
pub struct WindowUpdate {
    #[prost(uint64, tag = "1")]
    pub credit_bytes: u64,
}

#[derive(Clone, PartialEq, Message)]
pub struct Ping {
    #[prost(uint64, tag = "1")]
    pub nonce: u64,
}

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::io::AsyncWriteExt;

    #[tokio::test]
    async fn frame_round_trip_handles_fragmented_io() {
        let (mut left, mut right) = tokio::io::duplex(1024);
        let hello = Hello {
            client_id: "backend-a".into(),
            token: b"secret".to_vec(),
        };
        let frame = Frame::encode_message(MessageType::Hello, 0, &hello).unwrap();
        let expected = frame.clone();
        let writer = tokio::spawn(async move { frame.write_to(&mut left).await.unwrap() });
        let actual = Frame::read_from(&mut right).await.unwrap();
        writer.await.unwrap();
        assert_eq!(actual, expected);
        assert_eq!(actual.decode_message::<Hello>().unwrap(), hello);
    }

    #[tokio::test]
    async fn frame_rejects_large_metadata_without_writing() {
        let (mut left, _) = tokio::io::duplex(1024);
        let frame = Frame::new(MessageType::Request, 1, vec![0_u8; MAX_METADATA_LENGTH + 1]);
        assert!(matches!(
            frame.write_to(&mut left).await,
            Err(ProtocolError::MetadataTooLarge(_))
        ));
    }

    #[tokio::test]
    async fn frame_rejects_invalid_metadata_length() {
        let (mut left, mut right) = tokio::io::duplex(1024);
        tokio::spawn(async move {
            left.write_u32_le(HEADER_AFTER_LENGTH as u32).await.unwrap();
            left.write_u16_le(MAJOR_VERSION).await.unwrap();
            left.write_u16_le(MessageType::Request as u16)
                .await
                .unwrap();
            left.write_u32_le(0).await.unwrap();
            left.write_u64_le(1).await.unwrap();
            left.write_u32_le(1).await.unwrap();
            left.flush().await.unwrap();
        });
        assert!(matches!(
            Frame::read_from(&mut right).await,
            Err(ProtocolError::InvalidMetadataLength { .. })
        ));
    }

    #[tokio::test]
    async fn frame_rejects_reserved_flags_in_v1() {
        let (mut left, mut right) = tokio::io::duplex(128);
        tokio::spawn(async move {
            left.write_u32_le(HEADER_AFTER_LENGTH as u32).await.unwrap();
            left.write_u16_le(MAJOR_VERSION).await.unwrap();
            left.write_u16_le(MessageType::Ping as u16).await.unwrap();
            left.write_u32_le(1).await.unwrap();
            left.write_u64_le(0).await.unwrap();
            left.write_u32_le(0).await.unwrap();
            left.flush().await.unwrap();
        });
        assert!(matches!(
            Frame::read_from(&mut right).await,
            Err(ProtocolError::UnsupportedFlags(1))
        ));
    }

    #[test]
    fn object_value_round_trips_through_protobuf() {
        let value = Value::object([
            ("symbol", Value::string("EURUSD")),
            ("count", Value::u64(100)),
        ]);
        let mut bytes = Vec::new();
        value.encode(&mut bytes).unwrap();
        let decoded = Value::decode(bytes.as_slice()).unwrap();
        let fields = decoded.as_object().unwrap();
        assert!(matches!(fields["symbol"].kind, Some(value::Kind::String(ref s)) if s == "EURUSD"));
    }

    #[tokio::test]
    async fn frame_can_carry_large_logical_response_as_many_small_frames() {
        let (mut left, mut right) = tokio::io::duplex(2 * MAX_FRAME_LENGTH);
        let chunks = 20;
        let writer = tokio::spawn(async move {
            for n in 0..chunks {
                let metadata = ResponseChunk {
                    sequence: n,
                    row_offset: n * 10,
                    row_count: 10,
                    records: vec![],
                };
                let frame = Frame::encode_message(MessageType::ResponseChunk, 99, &metadata)
                    .unwrap()
                    .with_payload(vec![0_u8; DEFAULT_CHUNK_BYTES]);
                frame.write_to(&mut left).await.unwrap();
            }
        });
        let mut received = 0;
        while received < chunks {
            let frame = Frame::read_from(&mut right).await.unwrap();
            assert!(frame.payload.len() <= DEFAULT_CHUNK_BYTES);
            received += 1;
        }
        writer.await.unwrap();
    }
}
