package gomt5

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

var (
	orderCheckSentDebugOnce bool
	orderCheckRecvDebugOnce bool
	orderSendSentDebugOnce  bool
	orderSendRecvDebugOnce  bool
)

func (c *Client) OrderSend(ctx context.Context, request TradeRequest) (*TradeResult, error) {
	w := protocol.NewWriter()
	w.WriteU32(uint32(request.Action))
	w.WriteI64(request.Magic)
	w.WriteI64(request.Order)
	w.WriteString(request.Symbol)
	w.WriteF64(request.Volume)
	w.WriteF64(request.Price)
	w.WriteF64(request.StopLimit)
	w.WriteF64(request.SL)
	w.WriteF64(request.TP)
	w.WriteU32(uint32(request.Deviation))
	w.WriteU32(uint32(request.Type))
	w.WriteU32(uint32(request.TypeFilling))
	w.WriteU32(uint32(request.TypeTime))
	w.WriteI64(request.Expiration)
	w.WriteString(request.Comment)
	w.WriteI64(request.Position)
	w.WriteI64(request.PositionBy)

	sentParams := w.Bytes()
	if !orderSendSentDebugOnce {
		orderSendSentDebugOnce = true
		log.Printf("[gomt5] OrderSend request debug: total=%d hex=%s",
			len(sentParams), hex.EncodeToString(sentParams))
	}

	data, err := c.SendRaw(ctx, protocol.CmdOrderSend, sentParams)
	if err != nil {
		return nil, err
	}

	if !orderSendRecvDebugOnce {
		orderSendRecvDebugOnce = true
		log.Printf("[gomt5] OrderSend response debug: total=%d hex=%s",
			len(data), hex.EncodeToString(data))
	}

	r := protocol.NewReader(data)
	result := &TradeResult{
		Retcode:    r.ReadU32(),
		Deal:       r.ReadI64(),
		Order:      r.ReadI64(),
		Volume:     r.ReadF64(),
		Price:      r.ReadF64(),
		Bid:        r.ReadF64(),
		Ask:        r.ReadF64(),
		Comment:    r.ReadString(),
		RequestID:  r.ReadU32(),
		RetcodeExt: r.ReadI32(),
	}
	if r.Err() != nil {
		return nil, fmt.Errorf("decode trade result: %w", r.Err())
	}
	return result, nil
}

func (c *Client) OrderCheck(ctx context.Context, request TradeRequest) (*CheckResult, error) {
	w := protocol.NewWriter()
	w.WriteU32(uint32(request.Action))
	w.WriteString(request.Symbol)
	w.WriteF64(request.Volume)
	w.WriteF64(request.Price)
	w.WriteF64(request.SL)
	w.WriteF64(request.TP)
	w.WriteU32(uint32(request.Deviation))
	w.WriteU32(uint32(request.Type))
	w.WriteU32(uint32(request.TypeFilling))

	sentParams := w.Bytes()
	if !orderCheckSentDebugOnce {
		orderCheckSentDebugOnce = true
		log.Printf("[gomt5] OrderCheck request debug: total=%d hex=%s",
			len(sentParams), hex.EncodeToString(sentParams))
	}

	data, err := c.SendRaw(ctx, protocol.CmdOrderCheck, sentParams)
	if err != nil {
		return nil, err
	}

	if !orderCheckRecvDebugOnce {
		orderCheckRecvDebugOnce = true
		log.Printf("[gomt5] OrderCheck response debug: total=%d hex=%s",
			len(data), hex.EncodeToString(data))
	}

	r := protocol.NewReader(data)
	result := &CheckResult{
		Retcode:     r.ReadU32(),
		Balance:     r.ReadF64(),
		Equity:      r.ReadF64(),
		Profit:      r.ReadF64(),
		Margin:      r.ReadF64(),
		MarginFree:  r.ReadF64(),
		MarginLevel: r.ReadF64(),
		Comment:     r.ReadString(),
	}
	if r.Err() != nil {
		return nil, fmt.Errorf("decode check result: %w", r.Err())
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
	for i := 0; i < count; i++ {
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
