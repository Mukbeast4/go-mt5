package transport

import "time"

const (
	DefaultAddress        = "127.0.0.1:15555"
	DefaultTimeout        = 30 * time.Second
	DefaultReconnectDelay = 2 * time.Second
	DefaultMaxReconnect   = 5
)

type Options struct {
	Address        string
	Timeout        time.Duration
	ReconnectDelay time.Duration
	MaxReconnect   int
}

func DefaultOptions() Options {
	return Options{
		Address:        DefaultAddress,
		Timeout:        DefaultTimeout,
		ReconnectDelay: DefaultReconnectDelay,
		MaxReconnect:   DefaultMaxReconnect,
	}
}
