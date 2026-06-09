package gomt5_test

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func TestConcurrentRPCsRaceFree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in -short mode")
	}

	const (
		workers       = 32
		opsPerWorker  = 200
		stressTimeout = 15 * time.Second
	)

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		switch cmdID {
		case protocol.CmdInitialize:
			return true, writeU32(nil, 5836)
		case protocol.CmdSymbolsTotal:
			return true, writeU32(nil, 100)
		case protocol.CmdPositionsTotal:
			return true, writeU32(nil, 0)
		case protocol.CmdOrdersTotal:
			return true, writeU32(nil, 0)
		case protocol.CmdSymbolInfoTick:
			var d []byte
			d = writeI64(d, 1779289176)
			d = writeF64(d, 1.16003)
			d = writeF64(d, 1.16015)
			d = writeF64(d, 0)
			d = writeI64(d, 0)
			d = writeI64(d, 1779289176343)
			d = writeU32(d, 6)
			d = writeF64(d, 0)
			return true, d
		case protocol.CmdAccountInfo:
			d := make([]byte, 8+139+256+128+64+256)
			return true, d
		}
		return false, nil
	})
	defer mock.Close()

	ctx, cancel := context.WithTimeout(context.Background(), stressTimeout)
	defer cancel()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	var wg sync.WaitGroup
	var errCount atomic.Int64
	var okCount atomic.Int64

	for w := range workers {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			r := rand.New(rand.NewSource(seed))
			for i := range opsPerWorker {
				if ctx.Err() != nil {
					return
				}
				var err error
				switch r.Intn(4) {
				case 0:
					_, err = client.SymbolsTotal(ctx)
				case 1:
					_, err = client.SymbolInfoTick(ctx, "EURUSD")
				case 2:
					_, err = client.OrdersTotal(ctx)
				case 3:
					err = pingPositionsTotal(client, ctx)
				}
				if err != nil {
					if errors.Is(err, gomt5.ErrNotConnected) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
						errCount.Add(1)
						continue
					}
					t.Errorf("worker %d op %d: unexpected error: %v", seed, i, err)
					errCount.Add(1)
					return
				}
				okCount.Add(1)
			}
		}(int64(w))
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(stressTimeout + 5*time.Second):
		t.Fatalf("stress test hung: workers did not finish within %s", stressTimeout+5*time.Second)
	}

	t.Logf("concurrent stress: %d ok, %d non-fatal errs across %d workers x %d ops",
		okCount.Load(), errCount.Load(), workers, opsPerWorker)

	if okCount.Load() < int64(workers*opsPerWorker)/2 {
		t.Errorf("less than 50%% successful ops (%d/%d) — pipe may be misbehaving",
			okCount.Load(), workers*opsPerWorker)
	}
}

func pingPositionsTotal(client *gomt5.Client, ctx context.Context) error {
	_, err := client.PositionsTotal(ctx)
	return err
}

func TestConcurrentRPCsSurviveMidStreamClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in -short mode")
	}

	const workers = 16

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		switch cmdID {
		case protocol.CmdInitialize:
			return true, writeU32(nil, 5836)
		case protocol.CmdSymbolsTotal:
			return true, writeU32(nil, 100)
		}
		return false, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		mock.Close()
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	var wg sync.WaitGroup
	var fatalRaces atomic.Int64

	startBarrier := make(chan struct{})
	for range workers {
		wg.Go(func() {
			<-startBarrier
			for ctx.Err() == nil {
				_, err := client.SymbolsTotal(ctx)
				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) ||
						errors.Is(err, context.Canceled) ||
						errors.Is(err, gomt5.ErrNotConnected) {
						return
					}
					return
				}
			}
		})
	}

	close(startBarrier)
	time.Sleep(20 * time.Millisecond)
	mock.Close()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("workers did not exit within 5s after pipe close — possible deadlock")
	}

	if fatalRaces.Load() > 0 {
		t.Errorf("%d fatal races recorded", fatalRaces.Load())
	}
}

func TestPollQuotesRaceWithConcurrentRPCs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in -short mode")
	}

	raw, err := os.ReadFile("testdata/symbol_info_eurusd.bin")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	symbolsResp := writeU32(nil, 1)
	symbolsResp = append(symbolsResp, raw[8:]...)

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		switch cmdID {
		case protocol.CmdInitialize:
			return true, writeU32(nil, 5836)
		case protocol.CmdSymbolsTotal:
			return true, writeU32(nil, 100)
		case protocol.CmdSymbolsGetByGroup:
			return true, symbolsResp
		case protocol.CmdSymbolInfoTick:
			var d []byte
			d = writeI64(d, 1779289176)
			d = writeF64(d, 1.16003)
			d = writeF64(d, 1.16015)
			d = writeF64(d, 0)
			d = writeI64(d, 0)
			d = writeI64(d, 1779289176343)
			d = writeU32(d, 6)
			d = writeF64(d, 0)
			return true, d
		}
		return false, nil
	})
	defer mock.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	quotes, err := client.PollQuotes(ctx, []string{"EURUSD"}, time.Millisecond)
	if err != nil {
		t.Fatalf("poll quotes: %v", err)
	}

	drained := make(chan struct{})
	go func() {
		for range quotes {
		}
		close(drained)
	}()

	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for ctx.Err() == nil {
				if _, err := client.SymbolsTotal(ctx); err != nil {
					return
				}
				if _, err := client.SymbolInfoTick(ctx, "EURUSD"); err != nil {
					return
				}
			}
		})
	}

	time.Sleep(time.Second)
	client.Close()
	wg.Wait()

	select {
	case <-drained:
	case <-time.After(5 * time.Second):
		t.Fatal("quote channel did not close within 5s after client close")
	}
}
