package mt5

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mukbeast4/go-mt5/internal/protocol"
	"github.com/mukbeast4/go-mt5/internal/transport"
)

type Option func(*transport.Options)

func WithAddress(addr string) Option {
	return func(o *transport.Options) { o.Address = addr }
}

func WithTimeout(d time.Duration) Option {
	return func(o *transport.Options) { o.Timeout = d }
}

func WithReconnectDelay(d time.Duration) Option {
	return func(o *transport.Options) { o.ReconnectDelay = d }
}

func WithMaxReconnect(n int) Option {
	return func(o *transport.Options) { o.MaxReconnect = n }
}

type Client struct {
	conn      *transport.Connection
	opts      transport.Options
	pending   map[string]chan *protocol.Response
	tickSubs  map[string]chan Tick
	mu        sync.Mutex
	subMu     sync.RWMutex
	lastError error
}

func NewClient(opts ...Option) (*Client, error) {
	options := transport.DefaultOptions()
	for _, o := range opts {
		o(&options)
	}

	c := &Client{
		opts:     options,
		pending:  make(map[string]chan *protocol.Response),
		tickSubs: make(map[string]chan Tick),
	}

	c.conn = transport.NewConnection(options, c.handleMessage)
	if err := c.conn.Connect(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) Close() error {
	c.subMu.Lock()
	for sym, ch := range c.tickSubs {
		close(ch)
		delete(c.tickSubs, sym)
	}
	c.subMu.Unlock()

	return c.conn.Close()
}

func (c *Client) LastError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastError
}

func (c *Client) handleMessage(resp *protocol.Response) {
	if resp.Type == protocol.TypeEvent && resp.Event == "tick" {
		c.handleTickEvent(resp)
		return
	}

	c.mu.Lock()
	ch, ok := c.pending[resp.ID]
	if ok {
		delete(c.pending, resp.ID)
	}
	c.mu.Unlock()

	if ok {
		ch <- resp
	}
}

func (c *Client) handleTickEvent(resp *protocol.Response) {
	var tick Tick
	if err := json.Unmarshal(resp.Data, &tick); err != nil {
		return
	}

	c.subMu.RLock()
	ch, ok := c.tickSubs[tick.Symbol]
	c.subMu.RUnlock()

	if ok {
		select {
		case ch <- tick:
		default:
		}
	}
}

func (c *Client) send(ctx context.Context, action string, params interface{}) (*protocol.Response, error) {
	id := uuid.New().String()
	req := &protocol.Request{
		ID:     id,
		Action: action,
		Params: params,
	}

	ch := make(chan *protocol.Response, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	if err := c.conn.Send(req); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		c.setLastError(err)
		return nil, fmt.Errorf("send %s: %w", action, err)
	}

	select {
	case resp := <-ch:
		if !resp.Success && resp.Error != nil {
			err := &MT5Error{Code: resp.Error.Code, Message: resp.Error.Message}
			c.setLastError(err)
			return nil, err
		}
		return resp, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		c.setLastError(ErrTimeout)
		return nil, ctx.Err()
	}
}

func (c *Client) setLastError(err error) {
	c.mu.Lock()
	c.lastError = err
	c.mu.Unlock()
}

func (c *Client) Version(ctx context.Context) (*VersionInfo, error) {
	resp, err := c.send(ctx, protocol.ActionVersion, nil)
	if err != nil {
		return nil, err
	}
	var info VersionInfo
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		return nil, fmt.Errorf("decode version: %w", err)
	}
	return &info, nil
}
