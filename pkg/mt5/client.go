package mt5

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/mukbeast4/go-mt5/internal/pipe"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

type Option func(*options)

type options struct {
	pipeName string
	timeout  time.Duration
}

func WithPipeName(name string) Option {
	return func(o *options) { o.pipeName = name }
}

func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d }
}

type Client struct {
	conn      io.ReadWriteCloser
	mu        sync.Mutex
	build     int
	lastError error
}

func NewClient(opts ...Option) (*Client, error) {
	o := &options{
		timeout: 60 * time.Second,
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

	c := &Client{conn: conn}
	if err := c.initialize(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("initialize: %w", err)
	}

	return c, nil
}

func NewClientFromConn(conn io.ReadWriteCloser) (*Client, error) {
	c := &Client{conn: conn}
	if err := c.initialize(); err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}
	return c, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
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

func (c *Client) initialize() error {
	w := protocol.NewWriter()
	w.WriteU32(3)
	w.WriteString("Go")

	resp, err := c.send(protocol.CmdInitialize, w.Bytes())
	if err != nil {
		return err
	}

	if len(resp.Data) >= 4 {
		r := protocol.NewReader(resp.Data)
		c.build = int(r.ReadU32())
	}

	return nil
}

func (c *Client) send(cmdID uint32, params []byte) (*protocol.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, ErrNotConnected
	}

	if err := protocol.WriteRequest(c.conn, cmdID, params); err != nil {
		c.lastError = err
		return nil, fmt.Errorf("send cmd %d: %w", cmdID, err)
	}

	resp, err := protocol.ReadResponse(c.conn)
	if err != nil {
		c.lastError = err
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
		return nil, err
	}

	return resp, nil
}

func (c *Client) SendRaw(cmdID uint32, params []byte) ([]byte, error) {
	resp, err := c.send(cmdID, params)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}
