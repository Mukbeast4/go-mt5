package gomt5

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mukbeast4/go-mt5/internal/pipe"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

type connBox struct {
	conn io.ReadWriteCloser
}

type Option func(*options)

type options struct {
	pipeName          string
	timeout           time.Duration
	logger            Logger
	onRequest         RequestHook
	reconnect         ReconnectConfig
	onDisconnect      DisconnectHook
	onReconnect       ReconnectHook
	heartbeatInterval time.Duration
	tickBufferSize    int
	tickPollInterval  time.Duration
}

func WithPipeName(name string) Option {
	return func(o *options) { o.pipeName = name }
}

func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d }
}

type Client struct {
	conn              atomic.Pointer[connBox]
	mu                sync.Mutex
	build             int
	lastError         error
	logger            Logger
	onRequest         RequestHook
	state             int32
	reconnectCfg      ReconnectConfig
	onDisconnect      DisconnectHook
	onReconnect       ReconnectHook
	heartbeatInterval time.Duration
	stopCh            chan struct{}
	stopOnce          sync.Once
	pipeName          string
	pipeTimeout       time.Duration
	subMu             sync.Mutex
	subs              map[string]*tickSub
	quotePolls        map[*quotePoll]struct{}
	closed            bool
	tickBufferSize    int
	tickPollInterval  time.Duration
}

func (c *Client) loadConn() io.ReadWriteCloser {
	b := c.conn.Load()
	if b == nil {
		return nil
	}
	return b.conn
}

func (c *Client) storeConn(conn io.ReadWriteCloser) {
	if conn == nil {
		c.conn.Store(nil)
		return
	}
	c.conn.Store(&connBox{conn: conn})
}

func (c *Client) swapConn(conn io.ReadWriteCloser) io.ReadWriteCloser {
	var b *connBox
	if conn != nil {
		b = &connBox{conn: conn}
	}
	old := c.conn.Swap(b)
	if old == nil {
		return nil
	}
	return old.conn
}

func NewClient(ctx context.Context, opts ...Option) (*Client, error) {
	o := &options{
		timeout: 60 * time.Second,
		logger:  &nopLogger{},
	}
	for _, opt := range opts {
		opt(o)
	}

	pipeName := o.pipeName
	if pipeName == "" {
		discovered, err := pipe.Discover()
		if err != nil {
			return nil, fmt.Errorf("discover MT5 pipe: %w", err)
		}
		pipeName = discovered
	}

	conn, err := pipe.Open(pipeName, o.timeout)
	if err != nil {
		return nil, err
	}

	c := newClientFromOptions(conn, pipeName, o)
	if err := c.initialize(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("initialize: %w", err)
	}

	c.setState(StateConnected)
	if o.reconnect.Enabled && o.heartbeatInterval > 0 {
		c.startHeartbeat(ctx)
	}

	return c, nil
}

func NewClientFromConn(ctx context.Context, conn io.ReadWriteCloser, opts ...Option) (*Client, error) {
	o := &options{
		logger: &nopLogger{},
	}
	for _, opt := range opts {
		opt(o)
	}

	c := newClientFromOptions(conn, "", o)
	if err := c.initialize(ctx); err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}

	c.setState(StateConnected)
	return c, nil
}

func newClientFromOptions(conn io.ReadWriteCloser, pipeName string, o *options) *Client {
	c := &Client{
		logger:            o.logger,
		onRequest:         o.onRequest,
		reconnectCfg:      o.reconnect,
		onDisconnect:      o.onDisconnect,
		onReconnect:       o.onReconnect,
		heartbeatInterval: o.heartbeatInterval,
		pipeName:          pipeName,
		pipeTimeout:       o.timeout,
		subs:              make(map[string]*tickSub),
		quotePolls:        make(map[*quotePoll]struct{}),
		tickBufferSize:    o.tickBufferSize,
		tickPollInterval:  o.tickPollInterval,
	}
	c.storeConn(conn)
	return c
}

func (c *Client) Close() error {
	if c.stopCh != nil {
		c.stopOnce.Do(func() { close(c.stopCh) })
	}

	c.subMu.Lock()
	c.closed = true
	for symbol, sub := range c.subs {
		sub.cancel()
		delete(c.subs, symbol)
	}
	for poll := range c.quotePolls {
		poll.cancel()
		delete(c.quotePolls, poll)
	}
	c.subMu.Unlock()

	conn := c.swapConn(nil)
	var err error
	if conn != nil {
		err = conn.Close()
	}

	c.mu.Lock()
	c.setState(StateDisconnected)
	c.mu.Unlock()
	return err
}

func (c *Client) LastError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastError
}

func (c *Client) Build() int {
	return c.build
}

func (c *Client) initialize(ctx context.Context) error {
	w := protocol.NewWriter()
	w.WriteU32(3)
	w.WriteString("Go")

	resp, err := c.send(ctx, protocol.CmdInitialize, w.Bytes())
	if err != nil {
		return err
	}

	if len(resp.Data) >= 4 {
		r := protocol.NewReader(resp.Data)
		c.build = int(r.ReadU32())
	}

	return nil
}

func (c *Client) send(ctx context.Context, cmdID uint32, params []byte) (*protocol.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn := c.loadConn()
	if conn == nil {
		return nil, ErrNotConnected
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	start := time.Now()

	if err := protocol.WriteRequest(conn, cmdID, params); err != nil {
		c.lastError = err
		c.logger.Debug("send cmd=%d error=%v", cmdID, err)
		if c.onRequest != nil {
			c.onRequest(cmdID, time.Since(start), err)
		}
		return nil, fmt.Errorf("send cmd %d: %w", cmdID, err)
	}

	resp, err := protocol.ReadResponse(conn)
	duration := time.Since(start)
	if err != nil {
		c.lastError = err
		c.logger.Debug("recv cmd=%d error=%v duration=%v", cmdID, err, duration)
		if c.onRequest != nil {
			c.onRequest(cmdID, duration, err)
		}
		return nil, fmt.Errorf("recv cmd %d: %w", cmdID, err)
	}

	if !resp.Success {
		err := ErrFailed
		if len(resp.Data) >= 4 {
			r := protocol.NewReader(resp.Data)
			code := r.ReadI32()
			msg := "request failed"
			if r.Remaining() > 0 {
				msg = r.ReadString()
			}
			err = &MT5Error{Code: int(code), Message: msg}
		}
		c.lastError = err
		c.logger.Debug("cmd=%d failed duration=%v error=%v", cmdID, duration, err)
		if c.onRequest != nil {
			c.onRequest(cmdID, duration, err)
		}
		return nil, err
	}

	c.logger.Debug("cmd=%d ok duration=%v bytes=%d", cmdID, duration, len(resp.Data))
	if c.onRequest != nil {
		c.onRequest(cmdID, duration, nil)
	}

	return resp, nil
}

// SendRaw issues a raw command and returns the response payload bytes.
//
// The returned slice is owned by the caller for its full lifetime: it is not
// pooled, recycled, or aliased by the client. Callers may retain or mutate it
// freely. This contract is part of the public API — any future allocation
// optimization in the protocol layer must preserve this ownership semantics.
func (c *Client) SendRaw(ctx context.Context, cmdID uint32, params []byte) ([]byte, error) {
	resp, err := c.send(ctx, cmdID, params)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}
