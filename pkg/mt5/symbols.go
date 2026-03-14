package mt5

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func (c *Client) SymbolsTotal(ctx context.Context) (int, error) {
	resp, err := c.send(ctx, protocol.ActionSymbolsTotal, nil)
	if err != nil {
		return 0, err
	}
	var total int
	if err := json.Unmarshal(resp.Data, &total); err != nil {
		return 0, fmt.Errorf("decode symbols total: %w", err)
	}
	return total, nil
}

type symbolsGetParams struct {
	Group string `json:"group,omitempty"`
}

func (c *Client) SymbolsGet(ctx context.Context, group string) ([]SymbolInfo, error) {
	params := &symbolsGetParams{Group: group}
	resp, err := c.send(ctx, protocol.ActionSymbolsGet, params)
	if err != nil {
		return nil, err
	}
	var symbols []SymbolInfo
	if err := json.Unmarshal(resp.Data, &symbols); err != nil {
		return nil, fmt.Errorf("decode symbols: %w", err)
	}
	return symbols, nil
}

type symbolInfoParams struct {
	Symbol string `json:"symbol"`
}

func (c *Client) SymbolInfo(ctx context.Context, symbol string) (*SymbolInfo, error) {
	params := &symbolInfoParams{Symbol: symbol}
	resp, err := c.send(ctx, protocol.ActionSymbolInfo, params)
	if err != nil {
		return nil, err
	}
	var info SymbolInfo
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		return nil, fmt.Errorf("decode symbol info: %w", err)
	}
	return &info, nil
}

func (c *Client) SymbolInfoTick(ctx context.Context, symbol string) (*Tick, error) {
	params := &symbolInfoParams{Symbol: symbol}
	resp, err := c.send(ctx, protocol.ActionSymbolInfoTick, params)
	if err != nil {
		return nil, err
	}
	var tick Tick
	if err := json.Unmarshal(resp.Data, &tick); err != nil {
		return nil, fmt.Errorf("decode tick: %w", err)
	}
	return &tick, nil
}

type symbolSelectParams struct {
	Symbol string `json:"symbol"`
	Enable bool   `json:"enable"`
}

func (c *Client) SymbolSelect(ctx context.Context, symbol string, enable bool) error {
	params := &symbolSelectParams{Symbol: symbol, Enable: enable}
	_, err := c.send(ctx, protocol.ActionSymbolSelect, params)
	return err
}
