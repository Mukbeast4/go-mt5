package mt5

import (
	"context"
	"fmt"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

type subscribeParams struct {
	Symbol string `json:"symbol"`
}

func (c *Client) SubscribeTicks(ctx context.Context, symbol string) (<-chan Tick, error) {
	c.subMu.Lock()
	if _, exists := c.tickSubs[symbol]; exists {
		c.subMu.Unlock()
		return nil, fmt.Errorf("already subscribed to %s", symbol)
	}
	ch := make(chan Tick, 256)
	c.tickSubs[symbol] = ch
	c.subMu.Unlock()

	params := &subscribeParams{Symbol: symbol}
	_, err := c.send(ctx, protocol.ActionSubscribeTicks, params)
	if err != nil {
		c.subMu.Lock()
		delete(c.tickSubs, symbol)
		close(ch)
		c.subMu.Unlock()
		return nil, err
	}

	return ch, nil
}

func (c *Client) UnsubscribeTicks(ctx context.Context, symbol string) error {
	params := &subscribeParams{Symbol: symbol}
	_, err := c.send(ctx, protocol.ActionUnsubscribeTicks, params)

	c.subMu.Lock()
	if ch, ok := c.tickSubs[symbol]; ok {
		close(ch)
		delete(c.tickSubs, symbol)
	}
	c.subMu.Unlock()

	return err
}
