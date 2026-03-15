package gomt5

import (
	"log"
	"time"
)

type Logger interface {
	Debug(msg string, args ...any)
}

type defaultLogger struct{}

func (l *defaultLogger) Debug(msg string, args ...any) {
	log.Printf("[gomt5] "+msg, args...)
}

type nopLogger struct{}

func (l *nopLogger) Debug(string, ...any) {}

type RequestHook func(cmdID uint32, duration time.Duration, err error)

func WithLogger(l Logger) Option {
	return func(o *options) { o.logger = l }
}

func WithDebug(enabled bool) Option {
	return func(o *options) {
		if enabled {
			o.logger = &defaultLogger{}
		}
	}
}

func WithOnRequest(hook RequestHook) Option {
	return func(o *options) { o.onRequest = hook }
}
