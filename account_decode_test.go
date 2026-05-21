package gomt5_test

import (
	"context"
	"math"
	"os"
	"testing"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

func TestAccountInfoDecodeRealCapture(t *testing.T) {
	raw, err := os.ReadFile("testdata/account_info.bin")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	if len(raw) != 851 {
		t.Fatalf("expected 851 bytes capture, got %d", len(raw))
	}

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5836)
		}
		if cmdID == protocol.CmdAccountInfo {
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

	info, err := client.AccountInfo(ctx)
	if err != nil {
		t.Fatalf("account info: %v", err)
	}

	if info.Login <= 0 {
		t.Errorf("Login must be positive, got %d", info.Login)
	}
	if info.Leverage <= 0 {
		t.Errorf("Leverage must be positive, got %d", info.Leverage)
	}
	if l := len(info.Currency); l != 3 {
		t.Errorf("Currency length must be 3 (ISO 4217), got %d (%q)", l, info.Currency)
	}
	if info.Name == "" {
		t.Error("Name must be non-empty")
	}
	if info.Server == "" {
		t.Error("Server must be non-empty")
	}
	if info.Company == "" {
		t.Error("Company must be non-empty")
	}

	for _, f := range []struct {
		name string
		v    float64
	}{
		{"Balance", info.Balance},
		{"Credit", info.Credit},
		{"Profit", info.Profit},
		{"Equity", info.Equity},
		{"Margin", info.Margin},
		{"FreeMargin", info.FreeMargin},
		{"MarginLevel", info.MarginLevel},
		{"MarginSOCall", info.MarginSOCall},
		{"MarginSOSO", info.MarginSOSO},
		{"MarginInitial", info.MarginInitial},
		{"MarginMaintenance", info.MarginMaintenance},
		{"Assets", info.Assets},
		{"Liabilities", info.Liabilities},
		{"CommissionBlocked", info.CommissionBlocked},
	} {
		if math.IsNaN(f.v) || math.IsInf(f.v, 0) {
			t.Errorf("%s must be finite, got %v", f.name, f.v)
		}
	}

	if math.Abs(info.Equity-(info.Balance+info.Credit+info.Profit)) > 0.01 {
		t.Errorf("Equity (%.2f) != Balance + Credit + Profit (%.2f + %.2f + %.2f = %.2f)",
			info.Equity, info.Balance, info.Credit, info.Profit,
			info.Balance+info.Credit+info.Profit)
	}

	golden := []struct {
		name string
		got  any
		want any
	}{
		{"Login", info.Login, int64(7395945)},
		{"Leverage", info.Leverage, int64(500)},
		{"Currency", info.Currency, "EUR"},
		{"Server", info.Server, "FPTradingLLC-Demo"},
		{"Company", info.Company, "FP Trading LLC"},
		{"Balance", info.Balance, 103000.0},
		{"Equity", info.Equity, 103000.0},
		{"Credit", info.Credit, 0.0},
		{"Profit", info.Profit, 0.0},
	}
	for _, c := range golden {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}
