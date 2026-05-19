package gomt5

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"sync"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

var symbolInfoTickDebugSeen sync.Map

func (c *Client) SymbolsTotal(ctx context.Context) (int, error) {
	resp, err := c.send(ctx, protocol.CmdSymbolsTotal, nil)
	if err != nil {
		return 0, err
	}
	r := protocol.NewReader(resp.Data)
	total := r.ReadU32()
	if r.Err() != nil {
		return 0, fmt.Errorf("decode symbols total: %w", r.Err())
	}
	return int(total), nil
}

func (c *Client) SymbolsGet(ctx context.Context, group string) ([]SymbolInfo, error) {
	cmdID := protocol.CmdSymbolsGet
	w := protocol.NewWriter()

	if group != "" {
		cmdID = protocol.CmdSymbolsGetByGroup
		w.WriteString(group)
	}

	data, err := c.SendRaw(ctx, cmdID, w.Bytes())
	if err != nil {
		return nil, err
	}

	r := protocol.NewReader(data)
	count := int(r.ReadU32())

	symbols := make([]SymbolInfo, 0, count)
	for i := 0; i < count; i++ {
		sym := decodeSymbolInfo(r)
		if r.Err() != nil {
			return nil, fmt.Errorf("decode symbol %d: %w", i, r.Err())
		}
		symbols = append(symbols, *sym)
	}
	return symbols, nil
}

func (c *Client) SymbolInfo(ctx context.Context, symbol string) (*SymbolInfo, error) {
	w := protocol.NewWriter()
	w.WriteString(symbol)

	resp, err := c.send(ctx, protocol.CmdSymbolInfo, w.Bytes())
	if err != nil {
		return nil, err
	}

	r := protocol.NewReader(resp.Data)
	info := decodeSymbolInfo(r)
	if r.Err() != nil {
		return nil, fmt.Errorf("decode symbol info: %w", r.Err())
	}
	return info, nil
}

func (c *Client) SymbolInfoTick(ctx context.Context, symbol string) (*Tick, error) {
	w := protocol.NewWriter()
	w.WriteString(symbol)

	sentParams := w.Bytes()
	if _, loaded := symbolInfoTickDebugSeen.LoadOrStore(symbol+":sent", struct{}{}); !loaded {
		log.Printf("[gomt5] SymbolInfoTick request debug: symbol=%s total=%d hex=%s",
			symbol, len(sentParams), hex.EncodeToString(sentParams))
	}

	resp, err := c.send(ctx, protocol.CmdSymbolInfoTick, sentParams)
	if err != nil {
		return nil, err
	}

	if _, loaded := symbolInfoTickDebugSeen.LoadOrStore(symbol+":recv", struct{}{}); !loaded {
		log.Printf("[gomt5] SymbolInfoTick response debug: symbol=%s total=%d hex=%s",
			symbol, len(resp.Data), hex.EncodeToString(resp.Data))
	}

	if len(resp.Data) == 0 {
		return nil, ErrNoTick
	}

	r := protocol.NewReader(resp.Data)
	tick := decodeTick(r)
	if r.Err() != nil {
		return nil, fmt.Errorf("decode tick: %w", r.Err())
	}
	return tick, nil
}

func (c *Client) SymbolSelect(ctx context.Context, symbol string, enable bool) error {
	w := protocol.NewWriter()
	w.WriteString(symbol)
	w.WriteBool(enable)

	_, err := c.SendRaw(ctx, protocol.CmdSymbolSelect, w.Bytes())
	return err
}

// Wire layout for SymbolInfo records (cmd 170 single, cmd 174 array entries).
//
// Validated against a captured EURUSD response from MT5 build 5684 (the test
// fixture at testdata/symbol_info_eurusd.bin replays exactly that capture).
// Total record size = 2993 bytes:
//   - 145 bytes int/bool (mixed widths: bool=1, u32=4, u64=8)
//   - 416 bytes float64 (52 fields)
//   - 2432 bytes strings (13 fixed-width null-padded UTF-16LE slots)
//
// All 96 fields decode exactly as MetaTrader5 Python presents them.
//
// IMPORTANT: the string field order on the wire is NOT the same as the
// MetaTrader5 Python namedtuple iteration order. SymbolName is the LAST slot
// on the wire, even though it sits at index 93 in the namedtuple (between
// ISIN and Page). Wire order is below.
const (
	slotBasis          = 64
	slotCategory       = 128
	slotCurrencyBase   = 32
	slotCurrencyProfit = 32
	slotCurrencyMargin = 32
	slotBank           = 512
	slotDescription    = 64
	slotExchange       = 64
	slotFormula        = 1024
	slotISIN           = 32
	slotPage           = 128
	slotPath           = 256
	slotSymbolName     = 64

	symbolInfoStringRegionBytes = slotBasis + slotCategory + slotCurrencyBase +
		slotCurrencyProfit + slotCurrencyMargin + slotBank + slotDescription +
		slotExchange + slotFormula + slotISIN + slotPage + slotPath + slotSymbolName
)

// Compile-time assertion that the string region matches the captured size.
// If a future MT5 build changes any slot width, one of these becomes a
// negative uint constant and the package fails to build.
const (
	_ uint = symbolInfoStringRegionBytes - 2432
	_ uint = 2432 - symbolInfoStringRegionBytes
)

func decodeSymbolInfo(r *protocol.Reader) *SymbolInfo {
	info := &SymbolInfo{
		Custom:             r.ReadBool1(),
		ChartMode:          int64(r.ReadU32()),
		Select:             r.ReadBool1(),
		Visible:            r.ReadBool1(),
		SessionDeals:       r.ReadI64(),
		SessionBuyOrders:   r.ReadI64(),
		SessionSellOrders:  r.ReadI64(),
		Volume:             r.ReadI64(),
		VolumeHigh:         r.ReadI64(),
		VolumeLow:          r.ReadI64(),
		Time:               r.ReadI64(),
		Digits:             int64(r.ReadU32()),
		Spread:             int64(r.ReadU32()),
		SpreadFloat:        r.ReadBool1(),
		TicksBookDepth:     int64(r.ReadU32()),
		TradeCalcMode:      int64(r.ReadU32()),
		TradeMode:          int64(r.ReadU32()),
		StartTime:          r.ReadI64(),
		ExpirationTime:     r.ReadI64(),
		TradeStopsLevel:    int64(r.ReadU32()),
		TradeFreezeLevel:   int64(r.ReadU32()),
		TradeExeMode:       int64(r.ReadU32()),
		SwapMode:           int64(r.ReadU32()),
		SwapRollover3Days:  int64(r.ReadU32()),
		MarginHedgedUseLeg: r.ReadBool1(),
		ExpirationMode:     int64(r.ReadU32()),
		FillingMode:        int64(r.ReadU32()),
		OrderMode:          int64(r.ReadU32()),
		OrderGTCMode:       int64(r.ReadU32()),
		OptionMode:         int64(r.ReadU32()),
		OptionRight:        int64(r.ReadU32()),

		Bid:                     r.ReadF64(),
		BidHigh:                 r.ReadF64(),
		BidLow:                  r.ReadF64(),
		Ask:                     r.ReadF64(),
		AskHigh:                 r.ReadF64(),
		AskLow:                  r.ReadF64(),
		Last:                    r.ReadF64(),
		LastHigh:                r.ReadF64(),
		LastLow:                 r.ReadF64(),
		VolumeReal:              r.ReadF64(),
		VolumeHighReal:          r.ReadF64(),
		VolumeLowReal:           r.ReadF64(),
		OptionStrike:            r.ReadF64(),
		Point:                   r.ReadF64(),
		TradeTickValue:          r.ReadF64(),
		TradeTickValueProfit:    r.ReadF64(),
		TradeTickValueLoss:      r.ReadF64(),
		TradeTickSize:           r.ReadF64(),
		TradeContractSize:       r.ReadF64(),
		TradeAccruedInterest:    r.ReadF64(),
		TradeFaceValue:          r.ReadF64(),
		TradeLiquidityRate:      r.ReadF64(),
		VolumeMin:               r.ReadF64(),
		VolumeMax:               r.ReadF64(),
		VolumeStep:              r.ReadF64(),
		VolumeLimit:             r.ReadF64(),
		SwapLong:                r.ReadF64(),
		SwapShort:               r.ReadF64(),
		MarginInitial:           r.ReadF64(),
		MarginMaintenance:       r.ReadF64(),
		SessionVolume:           r.ReadF64(),
		SessionTurnover:         r.ReadF64(),
		SessionInterest:         r.ReadF64(),
		SessionBuyOrdersVolume:  r.ReadF64(),
		SessionSellOrdersVolume: r.ReadF64(),
		SessionOpen:             r.ReadF64(),
		SessionClose:            r.ReadF64(),
		SessionAW:               r.ReadF64(),
		SessionPriceSettlement:  r.ReadF64(),
		SessionPriceLimitMin:    r.ReadF64(),
		SessionPriceLimitMax:    r.ReadF64(),
		MarginHedged:            r.ReadF64(),
		PriceChange:             r.ReadF64(),
		PriceVolatility:         r.ReadF64(),
		PriceTheoretical:        r.ReadF64(),
		PriceGreeksDelta:        r.ReadF64(),
		PriceGreeksTheta:        r.ReadF64(),
		PriceGreeksGamma:        r.ReadF64(),
		PriceGreeksVega:         r.ReadF64(),
		PriceGreeksRho:          r.ReadF64(),
		PriceGreeksOmega:        r.ReadF64(),
		PriceSensitivity:        r.ReadF64(),
	}

	info.Basis = r.ReadFixedString(slotBasis)
	info.Category = r.ReadFixedString(slotCategory)
	info.CurrencyBase = r.ReadFixedString(slotCurrencyBase)
	info.CurrencyProfit = r.ReadFixedString(slotCurrencyProfit)
	info.CurrencyMargin = r.ReadFixedString(slotCurrencyMargin)
	info.Bank = r.ReadFixedString(slotBank)
	info.Description = r.ReadFixedString(slotDescription)
	info.Exchange = r.ReadFixedString(slotExchange)
	info.Formula = r.ReadFixedString(slotFormula)
	info.ISIN = r.ReadFixedString(slotISIN)
	info.Page = r.ReadFixedString(slotPage)
	info.Path = r.ReadFixedString(slotPath)
	info.SymbolName = r.ReadFixedString(slotSymbolName)

	return info
}

func decodeTick(r *protocol.Reader) *Tick {
	return &Tick{
		Time:       r.ReadI64(),
		Bid:        r.ReadF64(),
		Ask:        r.ReadF64(),
		Last:       r.ReadF64(),
		Volume:     r.ReadU64(),
		TimeMsc:    r.ReadI64(),
		Flags:      r.ReadU32(),
		VolumeReal: r.ReadF64(),
	}
}
