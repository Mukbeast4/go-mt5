package gomt5_test

import (
	"context"
	"math"
	"os"
	"testing"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func TestHistoryDealsDecodeRealCapture_242Deals(t *testing.T) {
	raw, err := os.ReadFile("testdata/history_deals_5833_242deals.bin")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	if len(raw) < 12 {
		t.Fatalf("capture too short: %d bytes", len(raw))
	}

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5833)
		}
		if cmdID == protocol.CmdHistoryDealsGet {
			return true, raw[12:]
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

	deals, err := client.HistoryDealsGet(ctx, nil)
	if err != nil {
		t.Fatalf("history deals: %v", err)
	}
	if len(deals) != 242 {
		t.Fatalf("expected 242 deals, got %d", len(deals))
	}

	for i, d := range deals {
		if d.Ticket <= 0 {
			t.Errorf("deal %d: Ticket must be positive, got %d", i, d.Ticket)
		}
		if d.Time <= 0 {
			t.Errorf("deal %d: Time must be positive, got %d", i, d.Time)
		}
		if d.TimeMsc < d.Time*1000 || d.TimeMsc >= (d.Time+1)*1000 {
			t.Errorf("deal %d: TimeMsc (%d) inconsistent with Time (%d)", i, d.TimeMsc, d.Time)
		}
		for _, p := range []struct {
			name string
			v    float64
		}{{"Volume", d.Volume}, {"Price", d.Price}, {"Commission", d.Commission}, {"Swap", d.Swap}, {"Profit", d.Profit}, {"Fee", d.Fee}} {
			if math.IsNaN(p.v) || math.IsInf(p.v, 0) {
				t.Errorf("deal %d: %s must be finite, got %v", i, p.name, p.v)
			}
		}
	}

	d0 := deals[0]
	t.Logf("Deal[0]: Ticket=%d Order=%d Time=%d Symbol=%q Volume=%.2f Price=%.5f Profit=%.2f Comment=%q",
		d0.Ticket, d0.Order, d0.Time, d0.Symbol, d0.Volume, d0.Price, d0.Profit, d0.Comment)
}

func TestHistoryDealsDecodeRealCapture_30d(t *testing.T) {
	raw, err := os.ReadFile("testdata/history_deals_30d.bin")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	if len(raw) != 904 {
		t.Fatalf("expected 904 bytes capture, got %d", len(raw))
	}

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5836)
		}
		if cmdID == protocol.CmdHistoryDealsGet {
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

	deals, err := client.HistoryDealsGet(ctx, nil)
	if err != nil {
		t.Fatalf("history deals: %v", err)
	}
	t.Logf("decoded %d deals from %d bytes", len(deals), len(raw))
	for i, d := range deals {
		t.Logf("Deal[%d]: Ticket=%d Type=%d Entry=%d Symbol=%q Volume=%.2f Price=%.5f Profit=%.2f",
			i, d.Ticket, d.Type, d.Entry, d.Symbol, d.Volume, d.Price, d.Profit)
	}
}
