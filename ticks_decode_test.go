package gomt5_test

import (
	"context"
	"math"
	"os"
	"testing"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func TestCopyTicksFromDecodeRealCapture(t *testing.T) {
	raw, err := os.ReadFile("testdata/ticks_100_eurusd.bin")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	if len(raw) != 6004 {
		t.Fatalf("expected 6004 bytes capture, got %d", len(raw))
	}

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5836)
		}
		if cmdID == protocol.CmdCopyTicksFrom {
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

	ticks, err := client.CopyTicksFrom(ctx, "EURUSD", 0, 100, gomt5.CopyTicksAll)
	if err != nil {
		t.Fatalf("copy ticks: %v", err)
	}
	if len(ticks) != 100 {
		t.Fatalf("expected 100 ticks (fixture is 4+100*60), got %d", len(ticks))
	}

	for i, tk := range ticks {
		if tk.Time <= 0 {
			t.Errorf("tick %d: Time must be positive, got %d", i, tk.Time)
		}
		if tk.TimeMsc <= 0 {
			t.Errorf("tick %d: TimeMsc must be positive, got %d", i, tk.TimeMsc)
		}
		if tk.TimeMsc/1000 != tk.Time {
			t.Errorf("tick %d: TimeMsc/1000 (%d) != Time (%d)", i, tk.TimeMsc/1000, tk.Time)
		}
		for _, p := range []struct {
			name string
			v    float64
		}{{"Bid", tk.Bid}, {"Ask", tk.Ask}} {
			if math.IsNaN(p.v) || math.IsInf(p.v, 0) || p.v <= 0 || p.v > 100 {
				t.Errorf("tick %d: %s must be a sane EURUSD price (0,100), got %v", i, p.name, p.v)
			}
		}
		if tk.Ask < tk.Bid {
			t.Errorf("tick %d: Ask (%.5f) < Bid (%.5f)", i, tk.Ask, tk.Bid)
		}
		if i > 0 && ticks[i-1].TimeMsc > tk.TimeMsc {
			t.Errorf("tick %d: TimeMsc (%d) earlier than previous (%d)", i, tk.TimeMsc, ticks[i-1].TimeMsc)
		}
	}

	t.Logf("Ticks[0]: Time=%d TimeMsc=%d Bid=%.5f Ask=%.5f Last=%.5f Vol=%d Flags=%d VolReal=%.5f",
		ticks[0].Time, ticks[0].TimeMsc, ticks[0].Bid, ticks[0].Ask, ticks[0].Last,
		ticks[0].Volume, ticks[0].Flags, ticks[0].VolumeReal)
	t.Logf("Ticks[99]: Time=%d TimeMsc=%d Bid=%.5f Ask=%.5f",
		ticks[99].Time, ticks[99].TimeMsc, ticks[99].Bid, ticks[99].Ask)
}
