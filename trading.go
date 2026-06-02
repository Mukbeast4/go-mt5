package gomt5

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

const (
	tradeRequestSymbolSlotBytes  = 64
	tradeRequestCommentSlotBytes = 64
	tradeRequestTotalBytes       = 232

	checkResultCommentSlotBytes = 200
	checkResultTotalBytes       = 252
	tradeResultCommentSlotBytes = 200
	tradeResultTotalBytes       = 260
)

func (c *Client) OrderSend(ctx context.Context, request TradeRequest) (*TradeResult, error) {
	w := protocol.NewWriter()
	w.WriteU32(uint32(request.Action))
	w.WriteI64(request.Magic)
	w.WriteI64(request.Order)
	w.WriteFixedString(request.Symbol, tradeRequestSymbolSlotBytes)
	w.WriteF64(request.Volume)
	w.WriteF64(request.Price)
	w.WriteF64(request.StopLimit)
	w.WriteF64(request.SL)
	w.WriteF64(request.TP)
	w.WriteU64(uint64(request.Deviation))
	w.WriteU32(uint32(request.Type))
	w.WriteU32(uint32(request.TypeFilling))
	w.WriteU32(uint32(request.TypeTime))
	w.WriteI64(request.Expiration)
	w.WriteFixedString(request.Comment, tradeRequestCommentSlotBytes)
	w.WriteI64(request.Position)
	w.WriteI64(request.PositionBy)

	data, err := c.SendRaw(ctx, protocol.CmdOrderSend, w.Bytes())
	if err != nil {
		return nil, err
	}

	if len(data) < tradeResultTotalBytes {
		return nil, fmt.Errorf("decode trade result: response too short (%d bytes, want %d)",
			len(data), tradeResultTotalBytes)
	}

	readF64 := func(off int) float64 {
		return math.Float64frombits(binary.LittleEndian.Uint64(data[off : off+8]))
	}
	result := &TradeResult{
		Retcode: binary.LittleEndian.Uint32(data[0:4]),
		Deal:    int64(binary.LittleEndian.Uint64(data[4:12])),
		Order:   int64(binary.LittleEndian.Uint64(data[12:20])),
		Volume:  readF64(20),
		Price:   readF64(28),
		Bid:     readF64(36),
		Ask:     readF64(44),
	}
	sr := protocol.NewReader(data[52 : 52+tradeResultCommentSlotBytes])
	result.Comment = sr.ReadFixedString(tradeResultCommentSlotBytes)
	if sr.Err() != nil {
		return nil, fmt.Errorf("decode trade result comment: %w", sr.Err())
	}
	result.RequestID = binary.LittleEndian.Uint32(data[252:256])
	result.RetcodeExt = int32(binary.LittleEndian.Uint32(data[256:260]))
	return result, nil
}

func (c *Client) OrderCheck(ctx context.Context, request TradeRequest) (*CheckResult, error) {
	w := protocol.NewWriter()
	w.WriteU32(uint32(request.Action))
	w.WriteI64(request.Magic)
	w.WriteI64(request.Order)
	w.WriteFixedString(request.Symbol, tradeRequestSymbolSlotBytes)
	w.WriteF64(request.Volume)
	w.WriteF64(request.Price)
	w.WriteF64(request.StopLimit)
	w.WriteF64(request.SL)
	w.WriteF64(request.TP)
	w.WriteU64(uint64(request.Deviation))
	w.WriteU32(uint32(request.Type))
	w.WriteU32(uint32(request.TypeFilling))
	w.WriteU32(uint32(request.TypeTime))
	w.WriteI64(request.Expiration)
	w.WriteFixedString(request.Comment, tradeRequestCommentSlotBytes)
	w.WriteI64(request.Position)
	w.WriteI64(request.PositionBy)

	data, err := c.SendRaw(ctx, protocol.CmdOrderCheck, w.Bytes())
	if err != nil {
		return nil, err
	}

	if len(data) < checkResultTotalBytes {
		return nil, fmt.Errorf("decode check result: response too short (%d bytes, want %d)",
			len(data), checkResultTotalBytes)
	}

	readF64 := func(off int) float64 {
		return math.Float64frombits(binary.LittleEndian.Uint64(data[off : off+8]))
	}
	result := &CheckResult{
		Retcode:     binary.LittleEndian.Uint32(data[0:4]),
		Balance:     readF64(4),
		Equity:      readF64(12),
		Profit:      readF64(20),
		Margin:      readF64(28),
		MarginFree:  readF64(36),
		MarginLevel: readF64(44),
	}
	sr := protocol.NewReader(data[52 : 52+checkResultCommentSlotBytes])
	result.Comment = sr.ReadFixedString(checkResultCommentSlotBytes)
	if sr.Err() != nil {
		return nil, fmt.Errorf("decode check result comment: %w", sr.Err())
	}
	return result, nil
}

func (c *Client) OrderCalcMargin(ctx context.Context, action TradeAction, symbol string, volume, price float64) (float64, error) {
	w := protocol.NewWriter()
	w.WriteU32(uint32(action))
	w.WriteString(symbol)
	w.WriteF64(volume)
	w.WriteF64(price)

	data, err := c.SendRaw(ctx, protocol.CmdOrderCalcMargin, w.Bytes())
	if err != nil {
		return 0, err
	}

	r := protocol.NewReader(data)
	margin := r.ReadF64()
	if r.Err() != nil {
		return 0, fmt.Errorf("decode margin: %w", r.Err())
	}
	return margin, nil
}

func (c *Client) OrderCalcProfit(ctx context.Context, action TradeAction, symbol string, volume, priceOpen, priceClose float64) (float64, error) {
	w := protocol.NewWriter()
	w.WriteU32(uint32(action))
	w.WriteString(symbol)
	w.WriteF64(volume)
	w.WriteF64(priceOpen)
	w.WriteF64(priceClose)

	data, err := c.SendRaw(ctx, protocol.CmdOrderCalcProfit, w.Bytes())
	if err != nil {
		return 0, err
	}

	r := protocol.NewReader(data)
	profit := r.ReadF64()
	if r.Err() != nil {
		return 0, fmt.Errorf("decode profit: %w", r.Err())
	}
	return profit, nil
}

func (c *Client) OrdersTotal(ctx context.Context) (int, error) {
	resp, err := c.send(ctx, protocol.CmdOrdersTotal, nil)
	if err != nil {
		return 0, err
	}
	r := protocol.NewReader(resp.Data)
	total := r.ReadU32()
	if r.Err() != nil {
		return 0, fmt.Errorf("decode orders total: %w", r.Err())
	}
	return int(total), nil
}

func (c *Client) OrdersGet(ctx context.Context, filter *OrderFilter) ([]Order, error) {
	var cmdID uint32
	w := protocol.NewWriter()

	switch {
	case filter == nil:
		cmdID = protocol.CmdOrdersGet
	case filter.Ticket != 0:
		cmdID = protocol.CmdOrdersGetByTicket
		w.WriteI64(filter.Ticket)
	case filter.Symbol != "":
		cmdID = protocol.CmdOrdersGetBySymbol
		w.WriteString(filter.Symbol)
	default:
		cmdID = protocol.CmdOrdersGet
	}

	data, err := c.SendRaw(ctx, cmdID, w.Bytes())
	if err != nil {
		return nil, err
	}

	orders, err := decodeOrders(data)
	if err != nil {
		return nil, err
	}

	if filter != nil && filter.Group != "" {
		orders = filterByGroup(orders, filter.Group, func(o Order) string { return o.Symbol })
	}

	return orders, nil
}

func decodeOrders(data []byte) ([]Order, error) {
	r := protocol.NewReader(data)
	count := int(r.ReadU32())

	orders := make([]Order, 0, count)
	for i := range count {
		order := Order{
			Ticket:         r.ReadI64(),
			TimeSetup:      r.ReadI64(),
			TimeSetupMsc:   r.ReadI64(),
			TimeDone:       r.ReadI64(),
			TimeDoneMsc:    r.ReadI64(),
			TimeExpiration: r.ReadI64(),
			Type:           OrderType(r.ReadU32()),
			TypeTime:       OrderTime(r.ReadU32()),
			TypeFilling:    OrderFilling(r.ReadU32()),
			State:          int64(r.ReadU32()),
			Magic:          r.ReadI64(),
			PositionID:     r.ReadI64(),
			PositionByID:   r.ReadI64(),
			Reason:         int64(r.ReadU32()),
			VolumeInitial:  r.ReadF64(),
			VolumeCurrent:  r.ReadF64(),
			PriceOpen:      r.ReadF64(),
			PriceCurrent:   r.ReadF64(),
			PriceSL:        r.ReadF64(),
			PriceTP:        r.ReadF64(),
			PriceStopLimit: r.ReadF64(),
			Symbol:         r.ReadFixedString(64),
			Comment:        r.ReadFixedString(64),
			ExternalID:     r.ReadFixedString(64),
		}
		if r.Err() != nil {
			return nil, fmt.Errorf("decode order %d: %w", i, r.Err())
		}
		orders = append(orders, order)
	}
	return orders, nil
}
