package gomt5

import (
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func (c *Client) SymbolsTotal() (int, error) {
	resp, err := c.send(protocol.CmdSymbolsTotal, nil)
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

func (c *Client) SymbolInfo(symbol string) (*SymbolInfo, error) {
	w := protocol.NewWriter()
	w.WriteString(symbol)

	resp, err := c.send(protocol.CmdSymbolInfo, w.Bytes())
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

func (c *Client) SymbolInfoTick(symbol string) (*Tick, error) {
	w := protocol.NewWriter()
	w.WriteString(symbol)

	resp, err := c.send(protocol.CmdSymbolInfoTick, w.Bytes())
	if err != nil {
		return nil, err
	}

	r := protocol.NewReader(resp.Data)
	tick := decodeTick(r)
	if r.Err() != nil {
		return nil, fmt.Errorf("decode tick: %w", r.Err())
	}
	return tick, nil
}

func decodeSymbolInfo(r *protocol.Reader) *SymbolInfo {
	return &SymbolInfo{
		Custom:            r.ReadBool(),
		ChartMode:         r.ReadI64(),
		Select:            r.ReadBool(),
		Visible:           r.ReadBool(),
		SessionDeals:      r.ReadI64(),
		SessionBuyOrders:  r.ReadI64(),
		SessionSellOrders: r.ReadI64(),
		Volume:            r.ReadI64(),
		VolumeHigh:        r.ReadI64(),
		VolumeLow:         r.ReadI64(),
		Time:              r.ReadI64(),
		Digits:            r.ReadI64(),
		Spread:            r.ReadI64(),
		SpreadFloat:       r.ReadBool(),
		TicksBookDepth:    r.ReadI64(),
		TradeCalcMode:     r.ReadI64(),
		TradeMode:         r.ReadI64(),
		StartTime:         r.ReadI64(),
		ExpirationTime:    r.ReadI64(),
		TradeStopsLevel:   r.ReadI64(),
		TradeFreezeLevel:  r.ReadI64(),
		TradeExeMode:      r.ReadI64(),
		SwapMode:          r.ReadI64(),
		SwapRollover3Days: r.ReadI64(),
		MarginHedgedUseLeg: r.ReadBool(),
		ExpirationMode:    r.ReadI64(),
		FillingMode:       r.ReadI64(),
		OrderMode:         r.ReadI64(),
		OrderGTCMode:      r.ReadI64(),
		OptionMode:        r.ReadI64(),
		OptionRight:       r.ReadI64(),

		Bid:                    r.ReadF64(),
		BidHigh:                r.ReadF64(),
		BidLow:                 r.ReadF64(),
		Ask:                    r.ReadF64(),
		AskHigh:                r.ReadF64(),
		AskLow:                 r.ReadF64(),
		Last:                   r.ReadF64(),
		LastHigh:               r.ReadF64(),
		LastLow:                r.ReadF64(),
		VolumeReal:             r.ReadF64(),
		VolumeHighReal:         r.ReadF64(),
		VolumeLowReal:          r.ReadF64(),
		OptionStrike:           r.ReadF64(),
		Point:                  r.ReadF64(),
		TradeTickValue:         r.ReadF64(),
		TradeTickValueProfit:   r.ReadF64(),
		TradeTickValueLoss:     r.ReadF64(),
		TradeTickSize:          r.ReadF64(),
		TradeContractSize:      r.ReadF64(),
		TradeAccruedInterest:   r.ReadF64(),
		TradeFaceValue:         r.ReadF64(),
		TradeLiquidityRate:     r.ReadF64(),
		VolumeMin:              r.ReadF64(),
		VolumeMax:              r.ReadF64(),
		VolumeStep:             r.ReadF64(),
		VolumeLimit:            r.ReadF64(),
		SwapLong:               r.ReadF64(),
		SwapShort:              r.ReadF64(),
		MarginInitial:          r.ReadF64(),
		MarginMaintenance:      r.ReadF64(),
		SessionVolume:          r.ReadF64(),
		SessionTurnover:        r.ReadF64(),
		SessionInterest:        r.ReadF64(),
		SessionBuyOrdersVolume: r.ReadF64(),
		SessionSellOrdersVolume: r.ReadF64(),
		SessionOpen:            r.ReadF64(),
		SessionClose:           r.ReadF64(),
		SessionAW:              r.ReadF64(),
		SessionPriceSettlement: r.ReadF64(),
		SessionPriceLimitMin:   r.ReadF64(),
		SessionPriceLimitMax:   r.ReadF64(),
		MarginHedged:           r.ReadF64(),
		PriceChange:            r.ReadF64(),
		PriceVolatility:        r.ReadF64(),
		PriceTheoretical:       r.ReadF64(),
		PriceGreeksDelta:       r.ReadF64(),
		PriceGreeksTheta:       r.ReadF64(),
		PriceGreeksGamma:       r.ReadF64(),
		PriceGreeksVega:        r.ReadF64(),
		PriceGreeksRho:         r.ReadF64(),
		PriceGreeksOmega:       r.ReadF64(),
		PriceSensitivity:       r.ReadF64(),

		Basis:          r.ReadString(),
		Category:       r.ReadString(),
		CurrencyBase:   r.ReadString(),
		CurrencyProfit: r.ReadString(),
		CurrencyMargin: r.ReadString(),
		Bank:           r.ReadString(),
		Description:    r.ReadString(),
		Exchange:       r.ReadString(),
		Formula:        r.ReadString(),
		ISIN:           r.ReadString(),
		SymbolName:     r.ReadString(),
		Page:           r.ReadString(),
		Path:           r.ReadString(),
	}
}

func decodeTick(r *protocol.Reader) *Tick {
	return &Tick{
		Time:    r.ReadI64(),
		Bid:     r.ReadF64(),
		Ask:     r.ReadF64(),
		Last:    r.ReadF64(),
		Volume:  r.ReadU64(),
		TimeMsc: r.ReadI64(),
		Flags:   r.ReadU32(),
	}
}
