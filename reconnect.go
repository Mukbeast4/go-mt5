package gomt5

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/mukbeast4/go-mt5/internal/pipe"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

type ConnState int32

const (
	StateConnected    ConnState = 0
	StateDisconnected ConnState = 1
	StateReconnecting ConnState = 2
)

func (s ConnState) String() string {
	switch s {
	case StateConnected:
		return "Connected"
	case StateDisconnected:
		return "Disconnected"
	case StateReconnecting:
		return "Reconnecting"
	default:
		return "Unknown"
	}
}

type ReconnectConfig struct {
	Enabled      bool
	InitialDelay time.Duration
	MaxDelay     time.Duration
	MaxRetries   int
}

type DisconnectHook func(err error)
type ReconnectHook func()

func WithAutoReconnect(enabled bool) Option {
	return func(o *options) {
		o.reconnect.Enabled = enabled
	}
}

func WithReconnectConfig(initial, max time.Duration, maxRetries int) Option {
	return func(o *options) {
		o.reconnect.Enabled = true
		o.reconnect.InitialDelay = initial
		o.reconnect.MaxDelay = max
		o.reconnect.MaxRetries = maxRetries
	}
}

func WithOnDisconnect(hook DisconnectHook) Option {
	return func(o *options) { o.onDisconnect = hook }
}

func WithOnReconnect(hook ReconnectHook) Option {
	return func(o *options) { o.onReconnect = hook }
}

func WithHeartbeatInterval(d time.Duration) Option {
	return func(o *options) { o.heartbeatInterval = d }
}

func (c *Client) Connected() bool {
	return ConnState(atomic.LoadInt32(&c.state)) == StateConnected
}

func (c *Client) State() ConnState {
	return ConnState(atomic.LoadInt32(&c.state))
}

func (c *Client) setState(s ConnState) {
	atomic.StoreInt32(&c.state, int32(s))
}

func (c *Client) startHeartbeat(ctx context.Context) {
	if c.heartbeatInterval <= 0 {
		return
	}

	c.stopCh = make(chan struct{})
	go func() {
		ticker := time.NewTicker(c.heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			case <-ticker.C:
				if !c.Connected() {
					continue
				}
				_, err := c.Version(ctx)
				if err != nil {
					c.logger.Debug("heartbeat failed: %v", err)
					if c.reconnectCfg.Enabled {
						go c.reconnect(ctx)
					}
				}
			}
		}
	}()
}

func (c *Client) reconnect(ctx context.Context) {
	if !atomic.CompareAndSwapInt32(&c.state, int32(StateConnected), int32(StateReconnecting)) {
		if ConnState(atomic.LoadInt32(&c.state)) == StateReconnecting {
			return
		}
		atomic.StoreInt32(&c.state, int32(StateReconnecting))
	}

	if c.onDisconnect != nil {
		c.onDisconnect(ErrDisconnected)
	}

	if old := c.swapConn(nil); old != nil {
		old.Close()
	}

	delay := c.reconnectCfg.InitialDelay
	if delay == 0 {
		delay = time.Second
	}
	maxDelay := c.reconnectCfg.MaxDelay
	if maxDelay == 0 {
		maxDelay = 30 * time.Second
	}
	maxRetries := c.reconnectCfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = -1
	}

	for attempt := 0; maxRetries < 0 || attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			c.setState(StateDisconnected)
			return
		default:
		}

		if c.stopCh != nil {
			select {
			case <-c.stopCh:
				c.setState(StateDisconnected)
				return
			default:
			}
		}

		c.logger.Debug("reconnect attempt %d, delay %v", attempt+1, delay)

		pipeName := c.pipeName
		if pipeName == "" {
			discovered, err := pipe.Discover()
			if err != nil {
				c.logger.Debug("discover pipe: %v", err)
				time.Sleep(delay)
				delay = min(delay*2, maxDelay)
				continue
			}
			pipeName = discovered
		}

		conn, err := pipe.Open(pipeName, c.pipeTimeout)
		if err != nil {
			c.logger.Debug("open pipe: %v", err)
			time.Sleep(delay)
			delay = min(delay*2, maxDelay)
			continue
		}

		c.storeConn(conn)

		w := protocol.NewWriter()
		w.WriteU32(3)
		w.WriteString("Go")
		if _, err := c.send(ctx, protocol.CmdInitialize, w.Bytes()); err != nil {
			c.logger.Debug("reinitialize: %v", err)
			if old := c.swapConn(nil); old != nil {
				old.Close()
			}
			time.Sleep(delay)
			delay = min(delay*2, maxDelay)
			continue
		}

		c.setState(StateConnected)
		c.logger.Debug("reconnected after %d attempts", attempt+1)
		if c.onReconnect != nil {
			c.onReconnect()
		}
		return
	}

	c.setState(StateDisconnected)
	c.logger.Debug("reconnect exhausted after %d retries", maxRetries)
}
