package gomt5_test

import (
	"context"
	"sync"
	"testing"
	"time"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func TestCloseAbortsInFlightRPC(t *testing.T) {
	ctx := context.Background()

	proceedHandler := make(chan struct{})
	handlerEntered := make(chan struct{})
	var enteredOnce sync.Once

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		enteredOnce.Do(func() { close(handlerEntered) })
		<-proceedHandler
		return false, nil
	})
	defer mock.Close()
	defer close(proceedHandler)

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	rpcErr := make(chan error, 1)
	go func() {
		_, err := client.SymbolsTotal(ctx)
		rpcErr <- err
	}()

	select {
	case <-handlerEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("RPC did not reach the server-side handler")
	}

	closeDone := make(chan error, 1)
	go func() {
		closeDone <- client.Close()
	}()

	select {
	case <-closeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Close hung — deadlock with in-flight RPC (c.mu held by send blocked in ReadResponse)")
	}

	select {
	case e := <-rpcErr:
		if e == nil {
			t.Error("expected error from aborted RPC, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight RPC never returned after Close")
	}
}

func TestCloseIdempotent(t *testing.T) {
	ctx := context.Background()
	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Errorf("second Close (must be no-op): %v", err)
	}
}
