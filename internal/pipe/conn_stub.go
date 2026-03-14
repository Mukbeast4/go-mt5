//go:build !windows

package pipe

import (
	"fmt"
	"io"
	"time"
)

type Conn struct {
	io.ReadWriteCloser
	name string
}

func Open(name string, timeout time.Duration) (*Conn, error) {
	return nil, fmt.Errorf("mt5: named pipes are only supported on Windows")
}

func Discover() (string, error) {
	return "", fmt.Errorf("mt5: pipe discovery is only supported on Windows")
}

func FindTerminalPath() (string, error) {
	return "", fmt.Errorf("mt5: terminal discovery is only supported on Windows")
}

func NewFromReadWriteCloser(rwc io.ReadWriteCloser) *Conn {
	return &Conn{ReadWriteCloser: rwc, name: "mock"}
}
