package gomt5_test

import (
	"context"
	"encoding/binary"
	"os"
	"testing"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func TestSymbolsGetDecodeRealCapture(t *testing.T) {
	raw, err := os.ReadFile("testdata/symbols_all.bin")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	expectedCount := binary.LittleEndian.Uint32(raw[:4])
	const symbolRecordSize = 2993
	expectedBytes := 4 + int(expectedCount)*symbolRecordSize
	if len(raw) != expectedBytes {
		t.Fatalf("fixture size %d does not match 4 + %d * %d = %d",
			len(raw), expectedCount, symbolRecordSize, expectedBytes)
	}

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5836)
		}
		if cmdID == protocol.CmdSymbolsGet {
			return true, raw
		}
		return false, nil
	})
	defer mock.Close()

	ctx := context.Background()
	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	symbols, err := client.SymbolsGet(ctx, "")
	if err != nil {
		t.Fatalf("symbols get: %v", err)
	}
	if uint32(len(symbols)) != expectedCount {
		t.Fatalf("expected %d symbols, got %d", expectedCount, len(symbols))
	}

	seenEURUSD := false
	for i, s := range symbols {
		if s.SymbolName == "" {
			t.Errorf("symbol %d: SymbolName must be non-empty", i)
		}
		if s.Digits < 0 || s.Digits > 10 {
			t.Errorf("symbol %d (%s): Digits out of range, got %d", i, s.SymbolName, s.Digits)
		}
		if s.SymbolName == "EURUSD" {
			seenEURUSD = true
			if s.CurrencyBase != "EUR" {
				t.Errorf("EURUSD.CurrencyBase = %q, want EUR", s.CurrencyBase)
			}
			if s.CurrencyProfit != "USD" {
				t.Errorf("EURUSD.CurrencyProfit = %q, want USD", s.CurrencyProfit)
			}
		}
	}

	if !seenEURUSD {
		t.Error("EURUSD not found in symbols_all.bin capture (expected for a Forex broker)")
	}

	t.Logf("decoded %d symbols, first=%q last=%q",
		len(symbols), symbols[0].SymbolName, symbols[len(symbols)-1].SymbolName)
}
