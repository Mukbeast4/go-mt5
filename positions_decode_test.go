package gomt5_test

import (
	"context"
	"math"
	"os"
	"testing"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

// TestPositionsDecodeRealCapture decodes a real cmd PositionsGet payload
// captured from a demo MT5 terminal (4 EURUSD buy positions opened by an
// EA). The capture is the wire payload only — 4-byte count header followed
// by four 320-byte records. See issue #17.
func TestPositionsDecodeRealCapture(t *testing.T) {
	raw, err := os.ReadFile("testdata/positions_4buys_eurusd.bin")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	if len(raw) != 1284 {
		t.Fatalf("expected 1284 bytes capture (count + 4*320), got %d", len(raw))
	}

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		if cmdID == protocol.CmdPositionsGet {
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

	positions, err := client.PositionsGet(ctx, nil)
	if err != nil {
		t.Fatalf("positions get: %v", err)
	}
	if len(positions) != 4 {
		t.Fatalf("expected 4 positions, got %d", len(positions))
	}

	p0 := positions[0]
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"Ticket", p0.Ticket, int64(27822128)},
		{"Type", p0.Type, gomt5.PositionTypeBuy},
		{"Magic", p0.Magic, int64(0)},
		{"Identifier", p0.Identifier, int64(27822128)},
		{"Reason", p0.Reason, int64(3)},
		{"Volume", p0.Volume, 0.01},
		{"PriceOpen", p0.PriceOpen, 1.16218},
		{"PriceSL", p0.PriceSL, 0.0},
		{"PriceTP", p0.PriceTP, 0.0},
		{"Commission", p0.Commission, 0.0},
		{"Swap", p0.Swap, 0.0},
		{"Symbol", p0.Symbol, "EURUSD"},
		{"Comment", p0.Comment, "test-1"},
		{"ExternalID", p0.ExternalID, ""},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("p0.%s: got %v, want %v", c.name, c.got, c.want)
		}
	}

	if math.Abs(p0.PriceCurrent-1.1597) > 1e-6 {
		t.Errorf("p0.PriceCurrent: got %v, want ~1.1597", p0.PriceCurrent)
	}
	if math.Abs(p0.Profit-(-2.14)) > 1e-6 {
		t.Errorf("p0.Profit: got %v, want ~-2.14", p0.Profit)
	}

	tail := []struct {
		idx     int
		ticket  int64
		comment string
		profit  float64
	}{
		{1, 27822129, "test-2", -2.11},
		{2, 27822794, "test-3", -2.01},
		{3, 27822815, "test-4", -2.08},
	}
	for _, p := range tail {
		got := positions[p.idx]
		if got.Ticket != p.ticket {
			t.Errorf("p%d.Ticket: got %d, want %d", p.idx, got.Ticket, p.ticket)
		}
		if got.Symbol != "EURUSD" {
			t.Errorf("p%d.Symbol: got %q, want %q", p.idx, got.Symbol, "EURUSD")
		}
		if got.Comment != p.comment {
			t.Errorf("p%d.Comment: got %q, want %q", p.idx, got.Comment, p.comment)
		}
		if math.Abs(got.PriceCurrent-1.1597) > 1e-3 {
			t.Errorf("p%d.PriceCurrent: got %v, want ~1.1597", p.idx, got.PriceCurrent)
		}
		if math.Abs(got.Profit-p.profit) > 1e-2 {
			t.Errorf("p%d.Profit: got %v, want ~%v", p.idx, got.Profit, p.profit)
		}
	}
}

// TestPositionsGetDecodesArray builds a synthetic PositionsGet response (count
// + N copies of a real 320-byte record) and verifies the decoder advances
// exactly the right number of bytes per record. Parallel to
// TestSymbolsGetDecodesArray (issue #2 regression).
func TestPositionsGetDecodesArray(t *testing.T) {
	raw, err := os.ReadFile("testdata/positions_4buys_eurusd.bin")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	record := raw[4 : 4+320]
	if len(record) != 320 {
		t.Fatalf("expected 320-byte record, got %d", len(record))
	}

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		if cmdID == protocol.CmdPositionsGet {
			out := make([]byte, 0, 4+3*len(record))
			out = writeU32(out, 3)
			out = append(out, record...)
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

	positions, err := client.PositionsGet(ctx, nil)
	if err != nil {
		t.Fatalf("positions get: %v", err)
	}
	if len(positions) != 3 {
		t.Fatalf("expected 3 positions, got %d", len(positions))
	}
	for i, p := range positions {
		if p.Ticket != 27822128 {
			t.Errorf("position %d: Ticket=%d, want 27822128", i, p.Ticket)
		}
		if p.Symbol != "EURUSD" {
			t.Errorf("position %d: Symbol=%q, want EURUSD", i, p.Symbol)
		}
	}
}
