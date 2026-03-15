package gomt5

import (
	"context"
	"time"
)

const defaultTickBufferSize = 256
const defaultPollInterval = 100 * time.Millisecond

func WithTickBufferSize(size int) Option {
	return func(o *options) { o.tickBufferSize = size }
}

func WithTickPollInterval(d time.Duration) Option {
	return func(o *options) { o.tickPollInterval = d }
}

type tickSub struct {
	ch     chan Tick
	errCh  chan error
	cancel context.CancelFunc
}

func (c *Client) SubscribeTicks(ctx context.Context, symbol string) (<-chan Tick, error) {
	ch, _, err := c.subscribeTicks(ctx, symbol, false)
	return ch, err
}

func (c *Client) SubscribeTicksWithErrors(ctx context.Context, symbol string) (<-chan Tick, <-chan error, error) {
	return c.subscribeTicks(ctx, symbol, true)
}

func (c *Client) subscribeTicks(ctx context.Context, symbol string, withErrors bool) (<-chan Tick, <-chan error, error) {
	if _, err := c.SymbolInfoTick(ctx, symbol); err != nil {
		return nil, nil, err
	}

	bufSize := c.tickBufferSize
	if bufSize <= 0 {
		bufSize = defaultTickBufferSize
	}

	pollInterval := c.tickPollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}

	ch := make(chan Tick, bufSize)
	var errCh chan error
	if withErrors {
		errCh = make(chan error, bufSize)
	}

	subCtx, cancel := context.WithCancel(ctx)
	sub := &tickSub{ch: ch, errCh: errCh, cancel: cancel}

	c.subMu.Lock()
	if existing, ok := c.subs[symbol]; ok {
		existing.cancel()
	}
	c.subs[symbol] = sub
	c.subMu.Unlock()

	go c.pollTicks(subCtx, symbol, sub, pollInterval)

	return ch, errCh, nil
}

func (c *Client) UnsubscribeTicks(symbol string) error {
	c.subMu.Lock()
	sub, ok := c.subs[symbol]
	if ok {
		delete(c.subs, symbol)
	}
	c.subMu.Unlock()

	if ok {
		sub.cancel()
	}
	return nil
}

func (c *Client) pollTicks(ctx context.Context, symbol string, sub *tickSub, interval time.Duration) {
	defer close(sub.ch)
	if sub.errCh != nil {
		defer close(sub.errCh)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastMsc int64

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick, err := c.SymbolInfoTick(ctx, symbol)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if sub.errCh != nil {
					select {
					case sub.errCh <- err:
					default:
					}
				}
				c.logger.Debug("tick poll %s error: %v", symbol, err)
				continue
			}

			if tick.TimeMsc <= lastMsc {
				continue
			}
			lastMsc = tick.TimeMsc

			select {
			case sub.ch <- *tick:
			default:
				<-sub.ch
				sub.ch <- *tick
				c.logger.Debug("tick buffer full for %s, dropped oldest", symbol)
			}
		}
	}
}
