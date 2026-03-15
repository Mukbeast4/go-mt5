package gomt5

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/mukbeast4/go-mt5/internal/pipe"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

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
	conn              io.ReadWriteCloser
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
	pipeName          string
	pipeTimeout       time.Duration
	subMu             sync.Mutex
	subs              map[string]*tickSub
	tickBufferSize    int
	tickPollInterval  time.Duration
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
	return &Client{
		conn:              conn,
		logger:            o.logger,
		onRequest:         o.onRequest,
		reconnectCfg:      o.reconnect,
		onDisconnect:      o.onDisconnect,
		onReconnect:       o.onReconnect,
		heartbeatInterval: o.heartbeatInterval,
		pipeName:          pipeName,
		pipeTimeout:       o.timeout,
		subs:              make(map[string]*tickSub),
		tickBufferSize:    o.tickBufferSize,
		tickPollInterval:  o.tickPollInterval,
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopCh != nil {
		select {
		case <-c.stopCh:
		default:
			close(c.stopCh)
		}
	}

	c.subMu.Lock()
	for symbol, sub := range c.subs {
		sub.cancel()
		delete(c.subs, symbol)
	}
	c.subMu.Unlock()

	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	c.setState(StateDisconnected)
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

	if c.conn == nil {
		return nil, ErrNotConnected
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	start := time.Now()

	if err := protocol.WriteRequest(c.conn, cmdID, params); err != nil {
		c.lastError = err
		c.logger.Debug("send cmd=%d error=%v", cmdID, err)
		if c.onRequest != nil {
			c.onRequest(cmdID, time.Since(start), err)
		}
		return nil, fmt.Errorf("send cmd %d: %w", cmdID, err)
	}

	resp, err := protocol.ReadResponse(c.conn)
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

func (c *Client) SendRaw(ctx context.Context, cmdID uint32, params []byte) ([]byte, error) {
	resp, err := c.send(ctx, cmdID, params)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}
