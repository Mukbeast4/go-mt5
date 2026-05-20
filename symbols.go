package gomt5

import (
	"context"
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

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

	symbols := make([]SymbolInfo, count)
	for i := 0; i < count; i++ {
		decodeSymbolInfo(r, &symbols[i])
		if r.Err() != nil {
			return nil, fmt.Errorf("decode symbol %d: %w", i, r.Err())
		}
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
	info := &SymbolInfo{}
	decodeSymbolInfo(r, info)
	if r.Err() != nil {
		return nil, fmt.Errorf("decode symbol info: %w", r.Err())
	}
	return info, nil
}

func (c *Client) SymbolInfoTick(ctx context.Context, symbol string) (*Tick, error) {
	w := protocol.NewWriter()
	w.WriteString(symbol)

	resp, err := c.send(ctx, protocol.CmdSymbolInfoTick, w.Bytes())
	if err != nil {
		return nil, err
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

func decodeSymbolInfo(r *protocol.Reader, info *SymbolInfo) {
	info.Custom = r.ReadBool1()
	info.ChartMode = int64(r.ReadU32())
	info.Select = r.ReadBool1()
	info.Visible = r.ReadBool1()
	info.SessionDeals = r.ReadI64()
	info.SessionBuyOrders = r.ReadI64()
	info.SessionSellOrders = r.ReadI64()
	info.Volume = r.ReadI64()
	info.VolumeHigh = r.ReadI64()
	info.VolumeLow = r.ReadI64()
	info.Time = r.ReadI64()
	info.Digits = int64(r.ReadU32())
	info.Spread = int64(r.ReadU32())
	info.SpreadFloat = r.ReadBool1()
	info.TicksBookDepth = int64(r.ReadU32())
	info.TradeCalcMode = int64(r.ReadU32())
	info.TradeMode = int64(r.ReadU32())
	info.StartTime = r.ReadI64()
	info.ExpirationTime = r.ReadI64()
	info.TradeStopsLevel = int64(r.ReadU32())
	info.TradeFreezeLevel = int64(r.ReadU32())
	info.TradeExeMode = int64(r.ReadU32())
	info.SwapMode = int64(r.ReadU32())
	info.SwapRollover3Days = int64(r.ReadU32())
	info.MarginHedgedUseLeg = r.ReadBool1()
	info.ExpirationMode = int64(r.ReadU32())
	info.FillingMode = int64(r.ReadU32())
	info.OrderMode = int64(r.ReadU32())
	info.OrderGTCMode = int64(r.ReadU32())
	info.OptionMode = int64(r.ReadU32())
	info.OptionRight = int64(r.ReadU32())

	info.Bid = r.ReadF64()
	info.BidHigh = r.ReadF64()
	info.BidLow = r.ReadF64()
	info.Ask = r.ReadF64()
	info.AskHigh = r.ReadF64()
	info.AskLow = r.ReadF64()
	info.Last = r.ReadF64()
	info.LastHigh = r.ReadF64()
	info.LastLow = r.ReadF64()
	info.VolumeReal = r.ReadF64()
	info.VolumeHighReal = r.ReadF64()
	info.VolumeLowReal = r.ReadF64()
	info.OptionStrike = r.ReadF64()
	info.Point = r.ReadF64()
	info.TradeTickValue = r.ReadF64()
	info.TradeTickValueProfit = r.ReadF64()
	info.TradeTickValueLoss = r.ReadF64()
	info.TradeTickSize = r.ReadF64()
	info.TradeContractSize = r.ReadF64()
	info.TradeAccruedInterest = r.ReadF64()
	info.TradeFaceValue = r.ReadF64()
	info.TradeLiquidityRate = r.ReadF64()
	info.VolumeMin = r.ReadF64()
	info.VolumeMax = r.ReadF64()
	info.VolumeStep = r.ReadF64()
	info.VolumeLimit = r.ReadF64()
	info.SwapLong = r.ReadF64()
	info.SwapShort = r.ReadF64()
	info.MarginInitial = r.ReadF64()
	info.MarginMaintenance = r.ReadF64()
	info.SessionVolume = r.ReadF64()
	info.SessionTurnover = r.ReadF64()
	info.SessionInterest = r.ReadF64()
	info.SessionBuyOrdersVolume = r.ReadF64()
	info.SessionSellOrdersVolume = r.ReadF64()
	info.SessionOpen = r.ReadF64()
	info.SessionClose = r.ReadF64()
	info.SessionAW = r.ReadF64()
	info.SessionPriceSettlement = r.ReadF64()
	info.SessionPriceLimitMin = r.ReadF64()
	info.SessionPriceLimitMax = r.ReadF64()
	info.MarginHedged = r.ReadF64()
	info.PriceChange = r.ReadF64()
	info.PriceVolatility = r.ReadF64()
	info.PriceTheoretical = r.ReadF64()
	info.PriceGreeksDelta = r.ReadF64()
	info.PriceGreeksTheta = r.ReadF64()
	info.PriceGreeksGamma = r.ReadF64()
	info.PriceGreeksVega = r.ReadF64()
	info.PriceGreeksRho = r.ReadF64()
	info.PriceGreeksOmega = r.ReadF64()
	info.PriceSensitivity = r.ReadF64()

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
