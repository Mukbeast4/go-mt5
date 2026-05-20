package gomt5_test

import (
	"context"
	"math"
	"os"
	"testing"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func TestCopyRatesFromPosDecodeRealCapture(t *testing.T) {
	raw, err := os.ReadFile("testdata/rates_h1_50_eurusd.bin")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	if len(raw) != 3004 {
		t.Fatalf("expected 3004 bytes capture, got %d", len(raw))
	}

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5836)
		}
		if cmdID == protocol.CmdCopyRatesFromPos {
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

	rates, err := client.CopyRatesFromPos(ctx, "EURUSD", gomt5.TimeframeH1, 0, 50)
	if err != nil {
		t.Fatalf("copy rates: %v", err)
	}
	if len(rates) != 50 {
		t.Fatalf("expected 50 rates, got %d", len(rates))
	}

	for i, r := range rates {
		if r.Time <= 0 {
			t.Errorf("rate %d: Time must be positive, got %d", i, r.Time)
		}
		for _, p := range []struct {
			name string
			v    float64
		}{{"Open", r.Open}, {"High", r.High}, {"Low", r.Low}, {"Close", r.Close}} {
			if math.IsNaN(p.v) || math.IsInf(p.v, 0) || p.v <= 0 {
				t.Errorf("rate %d: %s must be finite positive, got %v", i, p.name, p.v)
			}
		}
		if r.High < r.Low {
			t.Errorf("rate %d: High (%.5f) < Low (%.5f)", i, r.High, r.Low)
		}
		if r.High < r.Open || r.High < r.Close {
			t.Errorf("rate %d: High (%.5f) not max of (O=%.5f C=%.5f)", i, r.High, r.Open, r.Close)
		}
		if r.Low > r.Open || r.Low > r.Close {
			t.Errorf("rate %d: Low (%.5f) not min of (O=%.5f C=%.5f)", i, r.Low, r.Open, r.Close)
		}
		if r.TickVolume <= 0 {
			t.Errorf("rate %d: TickVolume must be positive, got %d", i, r.TickVolume)
		}
		if i > 0 && rates[i-1].Time >= r.Time {
			t.Errorf("rate %d: Time (%d) not strictly greater than previous (%d)", i, r.Time, rates[i-1].Time)
		}
	}

	const h1 = int64(3600)
	for i := 1; i < len(rates); i++ {
		gap := rates[i].Time - rates[i-1].Time
		if gap != h1 {
			t.Logf("rate %d -> %d: gap %ds (expected %ds for H1, may indicate weekend)", i-1, i, gap, h1)
		}
	}

	t.Logf("Rates[0]: Time=%d O=%.5f H=%.5f L=%.5f C=%.5f TickVol=%d",
		rates[0].Time, rates[0].Open, rates[0].High, rates[0].Low, rates[0].Close, rates[0].TickVolume)
	t.Logf("Rates[%d]: Time=%d O=%.5f H=%.5f L=%.5f C=%.5f",
		len(rates)-1, rates[len(rates)-1].Time, rates[len(rates)-1].Open,
		rates[len(rates)-1].High, rates[len(rates)-1].Low, rates[len(rates)-1].Close)
}
