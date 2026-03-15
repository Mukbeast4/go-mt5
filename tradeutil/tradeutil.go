package tradeutil

import (
	"context"
	"fmt"

	gomt5 "github.com/mukbeast4/go-mt5"
)

func Buy(ctx context.Context, client *gomt5.Client, symbol string, volume float64, opts ...TradeOption) (*gomt5.TradeResult, error) {
	return marketOrder(ctx, client, symbol, volume, gomt5.OrderTypeBuy, opts...)
}

func Sell(ctx context.Context, client *gomt5.Client, symbol string, volume float64, opts ...TradeOption) (*gomt5.TradeResult, error) {
	return marketOrder(ctx, client, symbol, volume, gomt5.OrderTypeSell, opts...)
}

func BuyLimit(ctx context.Context, client *gomt5.Client, symbol string, volume, price float64, opts ...TradeOption) (*gomt5.TradeResult, error) {
	return pendingOrder(ctx, client, symbol, volume, price, gomt5.OrderTypeBuyLimit, opts...)
}

func SellLimit(ctx context.Context, client *gomt5.Client, symbol string, volume, price float64, opts ...TradeOption) (*gomt5.TradeResult, error) {
	return pendingOrder(ctx, client, symbol, volume, price, gomt5.OrderTypeSellLimit, opts...)
}

func BuyStop(ctx context.Context, client *gomt5.Client, symbol string, volume, price float64, opts ...TradeOption) (*gomt5.TradeResult, error) {
	return pendingOrder(ctx, client, symbol, volume, price, gomt5.OrderTypeBuyStop, opts...)
}

func SellStop(ctx context.Context, client *gomt5.Client, symbol string, volume, price float64, opts ...TradeOption) (*gomt5.TradeResult, error) {
	return pendingOrder(ctx, client, symbol, volume, price, gomt5.OrderTypeSellStop, opts...)
}

func ModifySLTP(ctx context.Context, client *gomt5.Client, ticket int64, sl, tp float64) (*gomt5.TradeResult, error) {
	positions, err := client.PositionsGet(ctx, &gomt5.PositionFilter{Ticket: ticket})
	if err != nil {
		return nil, fmt.Errorf("get position %d: %w", ticket, err)
	}
	if len(positions) == 0 {
		return nil, fmt.Errorf("position %d not found", ticket)
	}

	pos := positions[0]
	return client.OrderSend(ctx, gomt5.TradeRequest{
		Action:   gomt5.TradeActionSLTP,
		Symbol:   pos.Symbol,
		Position: ticket,
		SL:       sl,
		TP:       tp,
	})
}

func marketOrder(ctx context.Context, client *gomt5.Client, symbol string, volume float64, orderType gomt5.OrderType, opts ...TradeOption) (*gomt5.TradeResult, error) {
	o := applyOpts(opts)

	tick, err := client.SymbolInfoTick(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("get tick %s: %w", symbol, err)
	}

	price := tick.Ask
	if orderType == gomt5.OrderTypeSell {
		price = tick.Bid
	}

	filling, err := resolveFilling(ctx, client, symbol, o)
	if err != nil {
		return nil, err
	}

	return client.OrderSend(ctx, gomt5.TradeRequest{
		Action:      gomt5.TradeActionDeal,
		Symbol:      symbol,
		Volume:      volume,
		Type:        orderType,
		Price:       price,
		Deviation:   o.deviation,
		Magic:       o.magic,
		Comment:     o.comment,
		TypeFilling: filling,
		SL:          o.sl,
		TP:          o.tp,
	})
}

func pendingOrder(ctx context.Context, client *gomt5.Client, symbol string, volume, price float64, orderType gomt5.OrderType, opts ...TradeOption) (*gomt5.TradeResult, error) {
	o := applyOpts(opts)

	filling, err := resolveFilling(ctx, client, symbol, o)
	if err != nil {
		return nil, err
	}

	return client.OrderSend(ctx, gomt5.TradeRequest{
		Action:      gomt5.TradeActionPending,
		Symbol:      symbol,
		Volume:      volume,
		Type:        orderType,
		Price:       price,
		Magic:       o.magic,
		Comment:     o.comment,
		TypeFilling: filling,
		SL:          o.sl,
		TP:          o.tp,
	})
}

func resolveFilling(ctx context.Context, client *gomt5.Client, symbol string, o tradeOpts) (gomt5.OrderFilling, error) {
	if o.fillingSet {
		return o.filling, nil
	}

	info, err := client.SymbolInfo(ctx, symbol)
	if err != nil {
		return 0, fmt.Errorf("get symbol info %s: %w", symbol, err)
	}

	mode := info.FillingMode
	switch {
	case mode&1 != 0:
		return gomt5.OrderFillingFOK, nil
	case mode&2 != 0:
		return gomt5.OrderFillingIOC, nil
	default:
		return gomt5.OrderFillingReturn, nil
	}
}
