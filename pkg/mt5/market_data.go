package mt5

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

type copyRatesFromParams struct {
	Symbol    string    `json:"symbol"`
	Timeframe Timeframe `json:"timeframe"`
	DateFrom  int64     `json:"date_from"`
	Count     int       `json:"count"`
}

func (c *Client) CopyRatesFrom(ctx context.Context, symbol string, timeframe Timeframe, dateFrom int64, count int) ([]Rate, error) {
	params := &copyRatesFromParams{
		Symbol:    symbol,
		Timeframe: timeframe,
		DateFrom:  dateFrom,
		Count:     count,
	}
	resp, err := c.send(ctx, protocol.ActionCopyRatesFrom, params)
	if err != nil {
		return nil, err
	}
	var rates []Rate
	if err := json.Unmarshal(resp.Data, &rates); err != nil {
		return nil, fmt.Errorf("decode rates: %w", err)
	}
	return rates, nil
}

type copyRatesFromPosParams struct {
	Symbol    string    `json:"symbol"`
	Timeframe Timeframe `json:"timeframe"`
	StartPos  int       `json:"start_pos"`
	Count     int       `json:"count"`
}

func (c *Client) CopyRatesFromPos(ctx context.Context, symbol string, timeframe Timeframe, startPos int, count int) ([]Rate, error) {
	params := &copyRatesFromPosParams{
		Symbol:    symbol,
		Timeframe: timeframe,
		StartPos:  startPos,
		Count:     count,
	}
	resp, err := c.send(ctx, protocol.ActionCopyRatesFromPos, params)
	if err != nil {
		return nil, err
	}
	var rates []Rate
	if err := json.Unmarshal(resp.Data, &rates); err != nil {
		return nil, fmt.Errorf("decode rates: %w", err)
	}
	return rates, nil
}

type copyRatesRangeParams struct {
	Symbol    string    `json:"symbol"`
	Timeframe Timeframe `json:"timeframe"`
	DateFrom  int64     `json:"date_from"`
	DateTo    int64     `json:"date_to"`
}

func (c *Client) CopyRatesRange(ctx context.Context, symbol string, timeframe Timeframe, dateFrom, dateTo int64) ([]Rate, error) {
	params := &copyRatesRangeParams{
		Symbol:    symbol,
		Timeframe: timeframe,
		DateFrom:  dateFrom,
		DateTo:    dateTo,
	}
	resp, err := c.send(ctx, protocol.ActionCopyRatesRange, params)
	if err != nil {
		return nil, err
	}
	var rates []Rate
	if err := json.Unmarshal(resp.Data, &rates); err != nil {
		return nil, fmt.Errorf("decode rates: %w", err)
	}
	return rates, nil
}

type copyTicksFromParams struct {
	Symbol   string        `json:"symbol"`
	DateFrom int64         `json:"date_from"`
	Count    int           `json:"count"`
	Flags    CopyTicksFlag `json:"flags"`
}

func (c *Client) CopyTicksFrom(ctx context.Context, symbol string, dateFrom int64, count int, flags CopyTicksFlag) ([]Tick, error) {
	params := &copyTicksFromParams{
		Symbol:   symbol,
		DateFrom: dateFrom,
		Count:    count,
		Flags:    flags,
	}
	resp, err := c.send(ctx, protocol.ActionCopyTicksFrom, params)
	if err != nil {
		return nil, err
	}
	var ticks []Tick
	if err := json.Unmarshal(resp.Data, &ticks); err != nil {
		return nil, fmt.Errorf("decode ticks: %w", err)
	}
	return ticks, nil
}

type copyTicksRangeParams struct {
	Symbol   string        `json:"symbol"`
	DateFrom int64         `json:"date_from"`
	DateTo   int64         `json:"date_to"`
	Flags    CopyTicksFlag `json:"flags"`
}

func (c *Client) CopyTicksRange(ctx context.Context, symbol string, dateFrom, dateTo int64, flags CopyTicksFlag) ([]Tick, error) {
	params := &copyTicksRangeParams{
		Symbol:   symbol,
		DateFrom: dateFrom,
		DateTo:   dateTo,
		Flags:    flags,
	}
	resp, err := c.send(ctx, protocol.ActionCopyTicksRange, params)
	if err != nil {
		return nil, err
	}
	var ticks []Tick
	if err := json.Unmarshal(resp.Data, &ticks); err != nil {
		return nil, fmt.Errorf("decode ticks: %w", err)
	}
	return ticks, nil
}

func (c *Client) MarketBookAdd(ctx context.Context, symbol string) error {
	params := &symbolInfoParams{Symbol: symbol}
	_, err := c.send(ctx, protocol.ActionMarketBookAdd, params)
	return err
}

func (c *Client) MarketBookGet(ctx context.Context, symbol string) ([]BookEntry, error) {
	params := &symbolInfoParams{Symbol: symbol}
	resp, err := c.send(ctx, protocol.ActionMarketBookGet, params)
	if err != nil {
		return nil, err
	}
	var entries []BookEntry
	if err := json.Unmarshal(resp.Data, &entries); err != nil {
		return nil, fmt.Errorf("decode book: %w", err)
	}
	return entries, nil
}

func (c *Client) MarketBookRelease(ctx context.Context, symbol string) error {
	params := &symbolInfoParams{Symbol: symbol}
	_, err := c.send(ctx, protocol.ActionMarketBookRelease, params)
	return err
}
