package mt5

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func (c *Client) PositionsTotal(ctx context.Context) (int, error) {
	resp, err := c.send(ctx, protocol.ActionPositionsTotal, nil)
	if err != nil {
		return 0, err
	}
	var total int
	if err := json.Unmarshal(resp.Data, &total); err != nil {
		return 0, fmt.Errorf("decode positions total: %w", err)
	}
	return total, nil
}

type positionsGetParams struct {
	Symbol string `json:"symbol,omitempty"`
	Group  string `json:"group,omitempty"`
	Ticket int64  `json:"ticket,omitempty"`
}

func (c *Client) PositionsGet(ctx context.Context, symbol string, group string, ticket int64) ([]Position, error) {
	params := &positionsGetParams{
		Symbol: symbol,
		Group:  group,
		Ticket: ticket,
	}
	resp, err := c.send(ctx, protocol.ActionPositionsGet, params)
	if err != nil {
		return nil, err
	}
	var positions []Position
	if err := json.Unmarshal(resp.Data, &positions); err != nil {
		return nil, fmt.Errorf("decode positions: %w", err)
	}
	return positions, nil
}
