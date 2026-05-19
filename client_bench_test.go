package gomt5_test

import (
	"context"
	"testing"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func BenchmarkSymbolInfoTick(b *testing.B) {
	ctx := context.Background()
	mock := newMockPipe(b, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		if cmdID == protocol.CmdSymbolInfoTick {
			var d []byte
			d = writeI64(d, 1710000000)
			d = writeF64(d, 1.08500)
			d = writeF64(d, 1.08520)
			d = writeF64(d, 0)
			d = writeI64(d, 0)
			d = writeI64(d, 1710000000123)
			d = writeU32(d, 6)
			d = writeF64(d, 0)
			return true, d
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		b.Fatalf("new client: %v", err)
	}
	defer client.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.SymbolInfoTick(ctx, "EURUSD")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCopyRatesFromPos(b *testing.B) {
	ctx := context.Background()
	mock := newMockPipe(b, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		if cmdID == protocol.CmdCopyRatesFromPos {
			var d []byte
			d = writeU32(d, 10)
			for i := 0; i < 10; i++ {
				d = writeI64(d, int64(1710000000+i*3600))
				d = writeF64(d, 1.0850)
				d = writeF64(d, 1.0860)
				d = writeF64(d, 1.0840)
				d = writeF64(d, 1.0855)
				d = writeI64(d, 100)
				d = writeU32(d, 5)
				d = writeI64(d, 0)
			}
			return true, d
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		b.Fatalf("new client: %v", err)
	}
	defer client.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.CopyRatesFromPos(ctx, "EURUSD", gomt5.TimeframeH1, 0, 10)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodePositions(b *testing.B) {
	ctx := context.Background()

	var posData []byte
	posData = writeU32(posData, 5)
	for i := 0; i < 5; i++ {
		posData = append(posData, mockPosition(int64(100+i), "EURUSD")...)
	}

	mock := newMockPipe(b, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		if cmdID == protocol.CmdPositionsGet {
			return true, posData
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		b.Fatalf("new client: %v", err)
	}
	defer client.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.PositionsGet(ctx, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtocolRoundtrip(b *testing.B) {
	ctx := context.Background()
	mock := newMockPipe(b, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		return true, writeU32(nil, 42)
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		b.Fatalf("new client: %v", err)
	}
	defer client.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.SymbolsTotal(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
