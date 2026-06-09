package gomt5

import (
	"context"
	"fmt"
	"maps"
	"math"
	"strings"
	"time"
)

type quotePoll struct {
	ch     chan map[string]Tick
	errCh  chan error
	cancel context.CancelFunc
}

// PollQuotes polls SymbolsGet every interval with the symbol list joined as
// an MT5 group expression and emits one map per poll containing only the
// symbols whose Bid, Ask or Last changed: one RPC per interval regardless of
// len(symbols). The first map contains all requested symbols. Updates that
// change only Volume or Time are never emitted, and the emitted Tick.TimeMsc
// is derived from SymbolInfo.Time with second precision, so consecutive
// ticks for a symbol may carry equal TimeMsc — do not deduplicate on it.
//
// Symbols must already be selected in Market Watch (SymbolSelect); unknown
// or unselected symbols make PollQuotes fail fast, a deselection while the
// stream runs silently freezes that symbol's quotes. Each poll decodes
// roughly 3KB per requested symbol (validated in production at 93 symbols,
// several hundred untested), so prefer intervals of 500ms or more for large
// lists.
//
// Emitted maps are owned by the receiver and never touched again by the
// client. The channel has capacity 1: when the consumer lags, changes
// coalesce into the next emitted map instead of being dropped. RPC errors
// mid-stream are retried at the next interval (PollQuotesWithErrors exposes
// them); the stream stops and the channel closes only when ctx is canceled
// or the client is closed, taking effect once any in-flight poll returns —
// there is no unsubscribe call.
func (c *Client) PollQuotes(ctx context.Context, symbols []string, interval time.Duration) (<-chan map[string]Tick, error) {
	ch, _, err := c.pollQuotes(ctx, symbols, interval, false)
	return ch, err
}

// PollQuotesWithErrors is PollQuotes with a second channel surfacing RPC
// errors from failed polls (capacity 1; while one error is pending unread,
// newer ones are dropped).
func (c *Client) PollQuotesWithErrors(ctx context.Context, symbols []string, interval time.Duration) (<-chan map[string]Tick, <-chan error, error) {
	return c.pollQuotes(ctx, symbols, interval, true)
}

func (c *Client) pollQuotes(ctx context.Context, symbols []string, interval time.Duration, withErrors bool) (<-chan map[string]Tick, <-chan error, error) {
	if len(symbols) == 0 {
		return nil, nil, fmt.Errorf("poll quotes: no symbols")
	}
	if interval <= 0 {
		return nil, nil, fmt.Errorf("poll quotes: interval must be positive, got %v", interval)
	}

	want := make(map[string]struct{}, len(symbols))
	deduped := make([]string, 0, len(symbols))
	for _, s := range symbols {
		if s == "" || strings.ContainsAny(s, ",*!") {
			return nil, nil, fmt.Errorf("poll quotes: invalid symbol name %q", s)
		}
		if _, ok := want[s]; ok {
			continue
		}
		want[s] = struct{}{}
		deduped = append(deduped, s)
	}

	c.subMu.Lock()
	alreadyClosed := c.closed
	c.subMu.Unlock()
	if alreadyClosed {
		return nil, nil, ErrNotConnected
	}

	group := strings.Join(deduped, ",")
	infos, err := c.SymbolsGet(ctx, group)
	if err != nil {
		return nil, nil, err
	}

	snapshot := make(map[string]Tick, len(deduped))
	var unselected []string
	for i := range infos {
		info := &infos[i]
		if _, ok := want[info.SymbolName]; !ok {
			continue
		}
		if !info.Select {
			unselected = append(unselected, info.SymbolName)
			continue
		}
		snapshot[info.SymbolName] = info.Tick()
	}
	if len(unselected) > 0 {
		return nil, nil, fmt.Errorf("poll quotes: not selected in Market Watch (call SymbolSelect first): %s", strings.Join(unselected, ", "))
	}
	if len(snapshot) != len(deduped) {
		missing := make([]string, 0, len(deduped)-len(snapshot))
		for _, s := range deduped {
			if _, ok := snapshot[s]; !ok {
				missing = append(missing, s)
			}
		}
		return nil, nil, fmt.Errorf("poll quotes: unknown symbols: %s", strings.Join(missing, ", "))
	}

	ch := make(chan map[string]Tick, 1)
	var errCh chan error
	if withErrors {
		errCh = make(chan error, 1)
	}

	pollCtx, cancel := context.WithCancel(ctx)
	poll := &quotePoll{ch: ch, errCh: errCh, cancel: cancel}

	c.subMu.Lock()
	if c.closed {
		c.subMu.Unlock()
		cancel()
		return nil, nil, ErrNotConnected
	}
	c.quotePolls[poll] = struct{}{}
	c.subMu.Unlock()

	prev := maps.Clone(snapshot)
	ch <- snapshot

	go c.runQuotePoll(pollCtx, poll, want, group, interval, prev)

	return ch, errCh, nil
}

type quoteChange struct {
	name string
	tick Tick
}

func (c *Client) runQuotePoll(ctx context.Context, poll *quotePoll, want map[string]struct{}, group string, interval time.Duration, prev map[string]Tick) {
	defer func() {
		c.subMu.Lock()
		delete(c.quotePolls, poll)
		c.subMu.Unlock()
		close(poll.ch)
		if poll.errCh != nil {
			close(poll.errCh)
		}
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var pending []quoteChange

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			infos, err := c.SymbolsGet(ctx, group)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				if poll.errCh != nil {
					select {
					case poll.errCh <- err:
					default:
					}
				}
				c.logger.Debug("quote poll error: %v", err)
				continue
			}

			if ctx.Err() != nil {
				return
			}

			latest := make(map[string]Tick, len(want))
			for i := range infos {
				info := &infos[i]
				if _, ok := want[info.SymbolName]; !ok {
					continue
				}
				latest[info.SymbolName] = info.Tick()
			}

			pending = pending[:0]
			for name, tick := range latest {
				if sameQuote(prev[name], tick) {
					continue
				}
				pending = append(pending, quoteChange{name, tick})
			}
			if len(pending) == 0 {
				continue
			}

			changed := make(map[string]Tick, len(pending))
			for _, p := range pending {
				changed[p.name] = p.tick
			}

			select {
			case poll.ch <- changed:
				for _, p := range pending {
					prev[p.name] = p.tick
				}
			default:
			}
		}
	}
}

func sameQuote(a, b Tick) bool {
	return math.Float64bits(a.Bid) == math.Float64bits(b.Bid) &&
		math.Float64bits(a.Ask) == math.Float64bits(b.Ask) &&
		math.Float64bits(a.Last) == math.Float64bits(b.Last)
}

// Tick returns the quote fields of s as a Tick. TimeMsc is derived from
// Time and has second precision; Flags is always zero.
func (s *SymbolInfo) Tick() Tick {
	return Tick{
		Time:       s.Time,
		Bid:        s.Bid,
		Ask:        s.Ask,
		Last:       s.Last,
		Volume:     uint64(s.Volume),
		TimeMsc:    s.Time * 1000,
		VolumeReal: s.VolumeReal,
	}
}
