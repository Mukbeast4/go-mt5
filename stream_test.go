package gomt5_test

import (
	"context"
	"errors"
	"testing"
	"time"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func newTickStreamClient(t *testing.T, opts ...gomt5.Option) *gomt5.Client {
	t.Helper()
	var timeMsc int64 = 1779289176000
	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		switch cmdID {
		case protocol.CmdInitialize:
			return true, writeU32(nil, 5836)
		case protocol.CmdSymbolInfoTick:
			timeMsc++
			var d []byte
			d = writeI64(d, timeMsc/1000)
			d = writeF64(d, 1.16003)
			d = writeF64(d, 1.16015)
			d = writeF64(d, 0)
			d = writeI64(d, 0)
			d = writeI64(d, timeMsc)
			d = writeU32(d, 6)
			d = writeF64(d, 0)
			return true, d
		}
		return false, nil
	})
	t.Cleanup(func() { mock.Close() })

	client, err := gomt5.NewClientFromConn(context.Background(), mock, opts...)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func TestSubscribeTicksAfterCloseFails(t *testing.T) {
	client := newTickStreamClient(t)
	client.Close()

	_, err := client.SubscribeTicks(context.Background(), "EURUSD")
	if !errors.Is(err, gomt5.ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected after Close, got: %v", err)
	}
}

func TestSubscribeTicksSlowConsumerBufferOne(t *testing.T) {
	client := newTickStreamClient(t,
		gomt5.WithTickBufferSize(1),
		gomt5.WithTickPollInterval(time.Millisecond),
	)

	ch, err := client.SubscribeTicks(context.Background(), "EURUSD")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	deadline := time.After(2 * time.Second)
	received := 0
	var lastMsc int64
	for received < 50 {
		select {
		case tick, ok := <-ch:
			if !ok {
				t.Fatal("channel closed prematurely")
			}
			if tick.TimeMsc <= lastMsc {
				t.Fatalf("ticks not monotonic: %d after %d", tick.TimeMsc, lastMsc)
			}
			lastMsc = tick.TimeMsc
			received++
			time.Sleep(time.Millisecond)
		case <-deadline:
			t.Fatalf("stalled after %d ticks — poller likely deadlocked in eviction", received)
		}
	}

	if err := client.UnsubscribeTicks("EURUSD"); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	waitClosed(t, ch)
}
