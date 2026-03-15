package tradeutil

import (
	"context"
	"fmt"

	gomt5 "github.com/mukbeast4/go-mt5"
)

func ClosePosition(ctx context.Context, client *gomt5.Client, ticket int64, opts ...TradeOption) (*gomt5.TradeResult, error) {
	positions, err := client.PositionsGet(ctx, &gomt5.PositionFilter{Ticket: ticket})
	if err != nil {
		return nil, fmt.Errorf("get position %d: %w", ticket, err)
	}
	if len(positions) == 0 {
		return nil, fmt.Errorf("position %d not found", ticket)
	}

	pos := positions[0]
	return closePosition(ctx, client, pos, opts...)
}

func CloseAll(ctx context.Context, client *gomt5.Client, opts ...TradeOption) ([]*gomt5.TradeResult, error) {
	positions, err := client.PositionsGet(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("get positions: %w", err)
	}

	results := make([]*gomt5.TradeResult, 0, len(positions))
	for _, pos := range positions {
		result, err := closePosition(ctx, client, pos, opts...)
		if err != nil {
			return results, fmt.Errorf("close position %d: %w", pos.Ticket, err)
		}
		results = append(results, result)
	}
	return results, nil
}

func CloseBySymbol(ctx context.Context, client *gomt5.Client, symbol string, opts ...TradeOption) ([]*gomt5.TradeResult, error) {
	positions, err := client.PositionsGet(ctx, &gomt5.PositionFilter{Symbol: symbol})
	if err != nil {
		return nil, fmt.Errorf("get positions for %s: %w", symbol, err)
	}

	results := make([]*gomt5.TradeResult, 0, len(positions))
	for _, pos := range positions {
		result, err := closePosition(ctx, client, pos, opts...)
		if err != nil {
			return results, fmt.Errorf("close position %d: %w", pos.Ticket, err)
		}
		results = append(results, result)
	}
	return results, nil
}

func closePosition(ctx context.Context, client *gomt5.Client, pos gomt5.Position, opts ...TradeOption) (*gomt5.TradeResult, error) {
	o := applyOpts(opts)

	tick, err := client.SymbolInfoTick(ctx, pos.Symbol)
	if err != nil {
		return nil, fmt.Errorf("get tick %s: %w", pos.Symbol, err)
	}

	closeType := gomt5.OrderTypeSell
	price := tick.Bid
	if pos.Type == gomt5.PositionTypeSell {
		closeType = gomt5.OrderTypeBuy
		price = tick.Ask
	}

	filling, err := resolveFilling(ctx, client, pos.Symbol, o)
	if err != nil {
		return nil, err
	}

	return client.OrderSend(ctx, gomt5.TradeRequest{
		Action:      gomt5.TradeActionDeal,
		Symbol:      pos.Symbol,
		Volume:      pos.Volume,
		Type:        closeType,
		Price:       price,
		Deviation:   o.deviation,
		Magic:       o.magic,
		Comment:     o.comment,
		TypeFilling: filling,
		Position:    pos.Ticket,
	})
}
