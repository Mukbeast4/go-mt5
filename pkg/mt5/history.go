package mt5

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

type historyParams struct {
	DateFrom int64  `json:"date_from"`
	DateTo   int64  `json:"date_to"`
	Group    string `json:"group,omitempty"`
	Ticket   int64  `json:"ticket,omitempty"`
	Position int64  `json:"position,omitempty"`
}

func (c *Client) HistoryOrdersTotal(ctx context.Context, dateFrom, dateTo int64) (int, error) {
	params := &historyParams{DateFrom: dateFrom, DateTo: dateTo}
	resp, err := c.send(ctx, protocol.ActionHistoryOrdersTotal, params)
	if err != nil {
		return 0, err
	}
	var total int
	if err := json.Unmarshal(resp.Data, &total); err != nil {
		return 0, fmt.Errorf("decode history orders total: %w", err)
	}
	return total, nil
}

func (c *Client) HistoryOrdersGet(ctx context.Context, dateFrom, dateTo int64, group string, ticket, position int64) ([]Order, error) {
	params := &historyParams{
		DateFrom: dateFrom,
		DateTo:   dateTo,
		Group:    group,
		Ticket:   ticket,
		Position: position,
	}
	resp, err := c.send(ctx, protocol.ActionHistoryOrdersGet, params)
	if err != nil {
		return nil, err
	}
	var orders []Order
	if err := json.Unmarshal(resp.Data, &orders); err != nil {
		return nil, fmt.Errorf("decode history orders: %w", err)
	}
	return orders, nil
}

func (c *Client) HistoryDealsTotal(ctx context.Context, dateFrom, dateTo int64) (int, error) {
	params := &historyParams{DateFrom: dateFrom, DateTo: dateTo}
	resp, err := c.send(ctx, protocol.ActionHistoryDealsTotal, params)
	if err != nil {
		return 0, err
	}
	var total int
	if err := json.Unmarshal(resp.Data, &total); err != nil {
		return 0, fmt.Errorf("decode history deals total: %w", err)
	}
	return total, nil
}

func (c *Client) HistoryDealsGet(ctx context.Context, dateFrom, dateTo int64, group string, ticket, position int64) ([]Deal, error) {
	params := &historyParams{
		DateFrom: dateFrom,
		DateTo:   dateTo,
		Group:    group,
		Ticket:   ticket,
		Position: position,
	}
	resp, err := c.send(ctx, protocol.ActionHistoryDealsGet, params)
	if err != nil {
		return nil, err
	}
	var deals []Deal
	if err := json.Unmarshal(resp.Data, &deals); err != nil {
		return nil, fmt.Errorf("decode history deals: %w", err)
	}
	return deals, nil
}
