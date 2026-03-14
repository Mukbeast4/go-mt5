package transport

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/mukbeast4/go-mt5/internal/protocol"
)

type MessageHandler func(*protocol.Response)

type Connection struct {
	opts    Options
	conn    net.Conn
	handler MessageHandler
	mu      sync.Mutex
	done    chan struct{}
	closed  bool
}

func NewConnection(opts Options, handler MessageHandler) *Connection {
	return &Connection{
		opts:    opts,
		handler: handler,
		done:    make(chan struct{}),
	}
}

func (c *Connection) Connect() error {
	conn, err := net.DialTimeout("tcp", c.opts.Address, c.opts.Timeout)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", c.opts.Address, err)
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	go c.readLoop()
	return nil
}

func (c *Connection) Send(req *protocol.Request) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	if err := conn.SetWriteDeadline(time.Now().Add(c.opts.Timeout)); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}

	return protocol.Encode(conn, req)
}

func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	close(c.done)

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Connection) readLoop() {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()

		if conn == nil {
			return
		}

		resp, err := protocol.Decode(conn)
		if err != nil {
			select {
			case <-c.done:
				return
			default:
			}
			log.Printf("read error: %v", err)
			c.tryReconnect()
			return
		}

		if c.handler != nil {
			c.handler(resp)
		}
	}
}

func (c *Connection) tryReconnect() {
	for i := 0; i < c.opts.MaxReconnect; i++ {
		select {
		case <-c.done:
			return
		default:
		}

		log.Printf("reconnecting (%d/%d)...", i+1, c.opts.MaxReconnect)
		time.Sleep(c.opts.ReconnectDelay)

		conn, err := net.DialTimeout("tcp", c.opts.Address, c.opts.Timeout)
		if err != nil {
			log.Printf("reconnect failed: %v", err)
			continue
		}

		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.conn = conn
		c.mu.Unlock()

		log.Printf("reconnected successfully")
		go c.readLoop()
		return
	}

	log.Printf("max reconnection attempts reached")
}
