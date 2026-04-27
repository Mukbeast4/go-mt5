package gomt5_test

import (
	"context"
	"encoding/binary"
	"os"
	"testing"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

// TestSymbolInfoDecodeRealCapture decodes a real cmd 170 (SymbolInfo) response
// captured from MT5 build 5684 for EURUSD. Expected values come from the
// MetaTrader5 Python namedtuple captured at the same moment.
func TestSymbolInfoDecodeRealCapture(t *testing.T) {
	raw, err := os.ReadFile("testdata/symbol_info_eurusd.bin")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	if len(raw) != 3001 {
		t.Fatalf("expected 3001 bytes capture, got %d", len(raw))
	}

	cmdEcho := binary.LittleEndian.Uint32(raw[0:4])
	success := binary.LittleEndian.Uint32(raw[4:8])
	if cmdEcho != protocol.CmdSymbolInfo {
		t.Fatalf("cmd echo: expected %d, got %d", protocol.CmdSymbolInfo, cmdEcho)
	}
	if success != 1 {
		t.Fatalf("success: expected 1, got %d", success)
	}

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		if cmdID == protocol.CmdSymbolInfo {
			return true, raw[8:]
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

	info, err := client.SymbolInfo(ctx, "EURUSD")
	if err != nil {
		t.Fatalf("symbol info: %v", err)
	}

	checks := []struct {
		name string
		got  any
		want any
	}{
		// Booleans
		{"Custom", info.Custom, false},
		{"Select", info.Select, true},
		{"Visible", info.Visible, true},
		{"SpreadFloat", info.SpreadFloat, true},
		{"MarginHedgedUseLeg", info.MarginHedgedUseLeg, false},
		// Ints
		{"ChartMode", info.ChartMode, int64(0)},
		{"Time", info.Time, int64(1773442795)},
		{"Digits", info.Digits, int64(5)},
		{"Spread", info.Spread, int64(32)},
		{"TradeMode", info.TradeMode, int64(4)},
		{"TradeExeMode", info.TradeExeMode, int64(2)},
		{"SwapMode", info.SwapMode, int64(1)},
		{"SwapRollover3Days", info.SwapRollover3Days, int64(3)},
		{"ExpirationMode", info.ExpirationMode, int64(15)},
		{"FillingMode", info.FillingMode, int64(3)},
		{"OrderMode", info.OrderMode, int64(127)},
		// Floats
		{"Bid", info.Bid, 1.14152},
		{"Ask", info.Ask, 1.14184},
		{"Point", info.Point, 1e-05},
		{"TradeContractSize", info.TradeContractSize, 100000.0},
		{"VolumeMin", info.VolumeMin, 0.01},
		{"VolumeMax", info.VolumeMax, 100.0},
		{"VolumeStep", info.VolumeStep, 0.01},
		// Strings
		{"CurrencyBase", info.CurrencyBase, "EUR"},
		{"CurrencyProfit", info.CurrencyProfit, "USD"},
		{"CurrencyMargin", info.CurrencyMargin, "EUR"},
		{"Description", info.Description, "Euro vs US Dollar"},
		{"SymbolName", info.SymbolName, "EURUSD"},
		{"Path", info.Path, "Forex\\EURUSD"},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}

// TestSymbolsGetDecodesArray builds a synthetic cmd 174 response (count + 2
// SymbolInfo records) using the same fixed layout and verifies the decoder
// advances exactly the right number of bytes per record. This is the
// regression test for issue #2 ("decode symbol 1: unexpected EOF").
func TestSymbolsGetDecodesArray(t *testing.T) {
	raw, err := os.ReadFile("testdata/symbol_info_eurusd.bin")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	// One record = payload bytes after the 8-byte cmd_echo+success header.
	record := raw[8:]
	if len(record) != 2993 {
		t.Fatalf("expected 2993-byte record, got %d", len(record))
	}

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		if cmdID == protocol.CmdSymbolsGet {
			out := make([]byte, 0, 4+2*len(record))
			out = writeU32(out, 2)
			out = append(out, record...)
			out = append(out, record...)
			return true, out
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

	syms, err := client.SymbolsGet(ctx, "")
	if err != nil {
		t.Fatalf("symbols get: %v", err)
	}
	if len(syms) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(syms))
	}
	for i, s := range syms {
		if s.SymbolName != "EURUSD" {
			t.Errorf("symbol %d: SymbolName=%q, want EURUSD", i, s.SymbolName)
		}
		if s.Bid != 1.14152 {
			t.Errorf("symbol %d: Bid=%v, want 1.14152", i, s.Bid)
		}
	}
}
