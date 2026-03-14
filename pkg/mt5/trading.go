package mt5

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func (c *Client) OrderSend(ctx context.Context, request TradeRequest) (*TradeResult, error) {
	resp, err := c.send(ctx, protocol.ActionOrderSend, request)
	if err != nil {
		return nil, err
	}
	var result TradeResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("decode trade result: %w", err)
	}
	return &result, nil
}

func (c *Client) OrderCheck(ctx context.Context, request TradeRequest) (*CheckResult, error) {
	resp, err := c.send(ctx, protocol.ActionOrderCheck, request)
	if err != nil {
		return nil, err
	}
	var result CheckResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("decode check result: %w", err)
	}
	return &result, nil
}

type calcMarginParams struct {
	Action TradeAction `json:"action"`
	Symbol string      `json:"symbol"`
	Volume float64     `json:"volume"`
	Price  float64     `json:"price"`
}

func (c *Client) OrderCalcMargin(ctx context.Context, action TradeAction, symbol string, volume, price float64) (float64, error) {
	params := &calcMarginParams{
		Action: action,
		Symbol: symbol,
		Volume: volume,
		Price:  price,
	}
	resp, err := c.send(ctx, protocol.ActionOrderCalcMargin, params)
	if err != nil {
		return 0, err
	}
	var margin float64
	if err := json.Unmarshal(resp.Data, &margin); err != nil {
		return 0, fmt.Errorf("decode margin: %w", err)
	}
	return margin, nil
}

type calcProfitParams struct {
	Action    TradeAction `json:"action"`
	Symbol    string      `json:"symbol"`
	Volume    float64     `json:"volume"`
	PriceOpen float64     `json:"price_open"`
	PriceClose float64    `json:"price_close"`
}

func (c *Client) OrderCalcProfit(ctx context.Context, action TradeAction, symbol string, volume, priceOpen, priceClose float64) (float64, error) {
	params := &calcProfitParams{
		Action:     action,
		Symbol:     symbol,
		Volume:     volume,
		PriceOpen:  priceOpen,
		PriceClose: priceClose,
	}
	resp, err := c.send(ctx, protocol.ActionOrderCalcProfit, params)
	if err != nil {
		return 0, err
	}
	var profit float64
	if err := json.Unmarshal(resp.Data, &profit); err != nil {
		return 0, fmt.Errorf("decode profit: %w", err)
	}
	return profit, nil
}

func (c *Client) OrdersTotal(ctx context.Context) (int, error) {
	resp, err := c.send(ctx, protocol.ActionOrdersTotal, nil)
	if err != nil {
		return 0, err
	}
	var total int
	if err := json.Unmarshal(resp.Data, &total); err != nil {
		return 0, fmt.Errorf("decode orders total: %w", err)
	}
	return total, nil
}

type ordersGetParams struct {
	Symbol string `json:"symbol,omitempty"`
	Group  string `json:"group,omitempty"`
	Ticket int64  `json:"ticket,omitempty"`
}

func (c *Client) OrdersGet(ctx context.Context, symbol string, group string, ticket int64) ([]Order, error) {
	params := &ordersGetParams{
		Symbol: symbol,
		Group:  group,
		Ticket: ticket,
	}
	resp, err := c.send(ctx, protocol.ActionOrdersGet, params)
	if err != nil {
		return nil, err
	}
	var orders []Order
	if err := json.Unmarshal(resp.Data, &orders); err != nil {
		return nil, fmt.Errorf("decode orders: %w", err)
	}
	return orders, nil
}
