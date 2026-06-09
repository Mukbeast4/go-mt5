package gomt5_test

import (
	"context"
	"errors"
	"testing"
	"time"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func rateResponse(barTime int64) []byte {
	d := writeU32(nil, 1)
	d = writeI64(d, barTime)
	d = writeF64(d, 1.1)
	d = writeF64(d, 1.2)
	d = writeF64(d, 1.0)
	d = writeF64(d, 1.15)
	d = writeI64(d, 100)
	d = writeU32(d, 5)
	d = writeI64(d, 0)
	return d
}

func newClockSkewClient(t *testing.T, ratesData []byte, success bool) (*gomt5.Client, *[]byte) {
	t.Helper()
	var captured []byte
	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		switch cmdID {
		case protocol.CmdInitialize:
			return true, writeU32(nil, 5836)
		case protocol.CmdCopyRatesFromPos:
			captured = append(captured[:0], params...)
			return success, ratesData
		}
		return false, nil
	})
	t.Cleanup(func() { mock.Close() })

	client, err := gomt5.NewClientFromConn(context.Background(), mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client, &captured
}

func TestClockSkewRounds(t *testing.T) {
	cases := []struct {
		name   string
		offset time.Duration
		want   time.Duration
	}{
		{"plus2h", 2*time.Hour - 17*time.Second, 2 * time.Hour},
		{"plus3h floor bias", 3*time.Hour - 45*time.Second, 3 * time.Hour},
		{"minus5h", -5*time.Hour - 10*time.Second, -5 * time.Hour},
		{"half hour", 5*time.Hour + 30*time.Minute - 5*time.Second, 5*time.Hour + 30*time.Minute},
		{"zero", -12 * time.Second, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			barTime := time.Now().Add(tc.offset).UTC().Unix()
			client, captured := newClockSkewClient(t, rateResponse(barTime), true)

			skew, err := client.ClockSkew(context.Background(), "EURUSD")
			if err != nil {
				t.Fatalf("clock skew: %v", err)
			}
			if skew != tc.want {
				t.Errorf("skew: expected %v, got %v", tc.want, skew)
			}

			r := protocol.NewReader(*captured)
			if sym := r.ReadString(); sym != "EURUSD" {
				t.Errorf("symbol param: expected EURUSD, got %q", sym)
			}
			if tf := r.ReadU32(); tf != uint32(gomt5.TimeframeM1) {
				t.Errorf("timeframe param: expected M1 (%d), got %d", gomt5.TimeframeM1, tf)
			}
			if pos := r.ReadU32(); pos != 0 {
				t.Errorf("start pos param: expected 0, got %d", pos)
			}
			if count := r.ReadU32(); count != 1 {
				t.Errorf("count param: expected 1, got %d", count)
			}
			if r.Err() != nil {
				t.Fatalf("decode params: %v", r.Err())
			}
		})
	}
}

func TestClockSkewNoBars(t *testing.T) {
	client, _ := newClockSkewClient(t, writeU32(nil, 0), true)

	_, err := client.ClockSkew(context.Background(), "EURUSD")
	if !errors.Is(err, gomt5.ErrNoBars) {
		t.Fatalf("expected ErrNoBars, got: %v", err)
	}
}

func TestClockSkewStaleBar(t *testing.T) {
	cases := []struct {
		name   string
		offset time.Duration
	}{
		{"weekend stale within plausible offsets", -7*time.Hour - 10*time.Minute},
		{"residual ahead of half hour", 2*time.Hour + 5*time.Minute},
		{"gross stale negative", -40*time.Hour - 3*time.Second},
		{"gross stale positive", 15 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			barTime := time.Now().Add(tc.offset).UTC().Unix()
			client, _ := newClockSkewClient(t, rateResponse(barTime), true)

			_, err := client.ClockSkew(context.Background(), "EURUSD")
			if !errors.Is(err, gomt5.ErrStaleBar) {
				t.Fatalf("expected ErrStaleBar, got: %v", err)
			}
		})
	}
}

func TestClockSkewRPCError(t *testing.T) {
	client, _ := newClockSkewClient(t, nil, false)

	_, err := client.ClockSkew(context.Background(), "EURUSD")
	if !errors.Is(err, gomt5.ErrFailed) {
		t.Fatalf("expected ErrFailed, got: %v", err)
	}
}
