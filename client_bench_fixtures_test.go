package gomt5_test

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func BenchmarkAccountInfo_RealFixture(b *testing.B) {
	raw, err := os.ReadFile("testdata/account_info.bin")
	if err != nil {
		b.Skipf("fixture missing: %v", err)
	}
	ctx := context.Background()
	mock := newMockPipe(b, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5836)
		}
		if cmdID == protocol.CmdAccountInfo {
			return true, raw
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		b.Fatalf("new client: %v", err)
	}
	defer client.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := client.AccountInfo(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHistoryDealsGet_242Deals(b *testing.B) {
	raw, err := os.ReadFile("testdata/history_deals_5833_242deals.bin")
	if err != nil {
		b.Skipf("fixture missing: %v", err)
	}
	if len(raw) < 12 {
		b.Fatalf("fixture too short: %d", len(raw))
	}
	payload := raw[12:]

	ctx := context.Background()
	mock := newMockPipe(b, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5833)
		}
		if cmdID == protocol.CmdHistoryDealsGet {
			return true, payload
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		b.Fatalf("new client: %v", err)
	}
	defer client.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := client.HistoryDealsGet(ctx, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSymbolsGet_959Symbols(b *testing.B) {
	raw, err := os.ReadFile("testdata/symbols_all.bin")
	if err != nil {
		b.Skipf("fixture missing: %v", err)
	}

	ctx := context.Background()
	mock := newMockPipe(b, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5836)
		}
		if cmdID == protocol.CmdSymbolsGet {
			return true, raw
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		b.Fatalf("new client: %v", err)
	}
	defer client.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := client.SymbolsGet(ctx, ""); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCopyRatesFromPos_50Bars(b *testing.B) {
	raw, err := os.ReadFile("testdata/rates_h1_50_eurusd.bin")
	if err != nil {
		b.Skipf("fixture missing: %v", err)
	}
	ctx := context.Background()
	mock := newMockPipe(b, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5836)
		}
		if cmdID == protocol.CmdCopyRatesFromPos {
			return true, raw
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		b.Fatalf("new client: %v", err)
	}
	defer client.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := client.CopyRatesFromPos(ctx, "EURUSD", gomt5.TimeframeH1, 0, 50); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCopyTicksFrom_100Ticks(b *testing.B) {
	raw, err := os.ReadFile("testdata/ticks_100_eurusd.bin")
	if err != nil {
		b.Skipf("fixture missing: %v", err)
	}
	ctx := context.Background()
	mock := newMockPipe(b, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5836)
		}
		if cmdID == protocol.CmdCopyTicksFrom {
			return true, raw
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		b.Fatalf("new client: %v", err)
	}
	defer client.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := client.CopyTicksFrom(ctx, "EURUSD", 0, 100, gomt5.CopyTicksAll); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSymbolInfoTick_UnderContention(b *testing.B) {
	symbolsRaw, err := os.ReadFile("testdata/symbols_all.bin")
	if err != nil {
		b.Skipf("fixture missing: %v", err)
	}

	var tickData []byte
	tickData = writeI64(tickData, 1779289176)
	tickData = writeF64(tickData, 1.16003)
	tickData = writeF64(tickData, 1.16015)
	tickData = writeF64(tickData, 0)
	tickData = writeI64(tickData, 0)
	tickData = writeI64(tickData, 1779289176343)
	tickData = writeU32(tickData, 6)
	tickData = writeF64(tickData, 0)

	ctx := context.Background()
	mock := newMockPipe(b, func(cmdID uint32, params []byte) (bool, []byte) {
		switch cmdID {
		case protocol.CmdInitialize:
			return true, writeU32(nil, 5836)
		case protocol.CmdSymbolInfoTick:
			return true, tickData
		case protocol.CmdSymbolsGet:
			return true, symbolsRaw
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		b.Fatalf("new client: %v", err)
	}
	defer client.Close()

	var stop atomic.Bool
	contentionDone := make(chan struct{})
	go func() {
		defer close(contentionDone)
		for !stop.Load() {
			if _, err := client.SymbolsGet(ctx, ""); err != nil {
				return
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := client.SymbolInfoTick(ctx, "EURUSD"); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()

	stop.Store(true)
	<-contentionDone
}
