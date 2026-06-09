package gomt5_test

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

const (
	symbolRecordOffSelect = 5
	symbolRecordOffTime   = 55
	symbolRecordOffBid    = 145
	symbolRecordOffAsk    = 169
	symbolRecordOffLast   = 193
	symbolRecordOffName   = 2929
)

func loadSymbolRecord(t testing.TB) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/symbol_info_eurusd.bin")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	if len(raw) != 3001 {
		t.Fatalf("expected 3001-byte capture, got %d", len(raw))
	}
	return raw[8:]
}

func symbolRecord(t testing.TB, base []byte, name string, tsec int64, bid, ask, last float64) []byte {
	t.Helper()
	rec := make([]byte, len(base))
	copy(rec, base)
	binary.LittleEndian.PutUint64(rec[symbolRecordOffTime:], uint64(tsec))
	binary.LittleEndian.PutUint64(rec[symbolRecordOffBid:], math.Float64bits(bid))
	binary.LittleEndian.PutUint64(rec[symbolRecordOffAsk:], math.Float64bits(ask))
	binary.LittleEndian.PutUint64(rec[symbolRecordOffLast:], math.Float64bits(last))
	clear(rec[symbolRecordOffName : symbolRecordOffName+64])
	for i, r := range utf16.Encode([]rune(name)) {
		binary.LittleEndian.PutUint16(rec[symbolRecordOffName+i*2:], r)
	}
	return rec
}

func symbolsResponse(records ...[]byte) []byte {
	out := writeU32(nil, uint32(len(records)))
	for _, r := range records {
		out = append(out, r...)
	}
	return out
}

func newQuoteTestClient(t *testing.T, payloads chan []byte) (*gomt5.Client, *[]byte) {
	t.Helper()
	var captured []byte
	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		switch cmdID {
		case protocol.CmdInitialize:
			return true, writeU32(nil, 5836)
		case protocol.CmdSymbolsGetByGroup:
			if payloads == nil {
				t.Errorf("unexpected SymbolsGetByGroup RPC")
				return false, nil
			}
			captured = append(captured[:0], params...)
			p := <-payloads
			if p == nil {
				return false, nil
			}
			return true, p
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

func push(t *testing.T, payloads chan []byte, p []byte) {
	t.Helper()
	select {
	case payloads <- p:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout pushing poll payload — poller not consuming")
	}
}

func prePush(payloads chan []byte, p []byte) {
	go func() { payloads <- p }()
}

func recvQuotes(t *testing.T, ch <-chan map[string]gomt5.Tick) map[string]gomt5.Tick {
	t.Helper()
	select {
	case m, ok := <-ch:
		if !ok {
			t.Fatal("quote channel closed unexpectedly")
		}
		return m
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for quotes")
	}
	return nil
}

func waitClosed[T any](t *testing.T, ch <-chan T) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("channel did not close within 5s")
		}
	}
}

func TestPollQuotesInitialSnapshot(t *testing.T) {
	base := loadSymbolRecord(t)
	payloads := make(chan []byte)
	client, _ := newQuoteTestClient(t, payloads)

	prePush(payloads, symbolsResponse(
		symbolRecord(t, base, "EURUSD", 1700000000, 1.10000, 1.10010, 1.10005),
		symbolRecord(t, base, "GBPUSD", 1700000001, 2.20000, 2.20020, 0),
	))

	quotes, err := client.PollQuotes(context.Background(), []string{"EURUSD", "GBPUSD"}, time.Millisecond)
	if err != nil {
		t.Fatalf("poll quotes: %v", err)
	}

	snap := recvQuotes(t, quotes)
	if len(snap) != 2 {
		t.Fatalf("snapshot size: expected 2, got %d", len(snap))
	}
	eur, ok := snap["EURUSD"]
	if !ok {
		t.Fatal("snapshot missing EURUSD")
	}
	if eur.Bid != 1.10000 || eur.Ask != 1.10010 || eur.Last != 1.10005 {
		t.Errorf("EURUSD quote: got Bid=%v Ask=%v Last=%v", eur.Bid, eur.Ask, eur.Last)
	}
	if eur.Time != 1700000000 {
		t.Errorf("EURUSD time: expected 1700000000, got %d", eur.Time)
	}
	if eur.TimeMsc != 1700000000000 {
		t.Errorf("EURUSD time_msc: expected Time*1000, got %d", eur.TimeMsc)
	}
	if eur.Flags != 0 {
		t.Errorf("EURUSD flags: expected 0, got %d", eur.Flags)
	}
	gbp, ok := snap["GBPUSD"]
	if !ok {
		t.Fatal("snapshot missing GBPUSD")
	}
	if gbp.Bid != 2.20000 {
		t.Errorf("GBPUSD bid: expected 2.20000, got %v", gbp.Bid)
	}
}

func TestPollQuotesSendsGroupRequest(t *testing.T) {
	base := loadSymbolRecord(t)
	resp := symbolsResponse(
		symbolRecord(t, base, "EURUSD", 1700000000, 1.1, 1.2, 0),
		symbolRecord(t, base, "GBPUSD", 1700000000, 2.1, 2.2, 0),
	)

	cases := []struct {
		name    string
		symbols []string
		want    string
	}{
		{"joined", []string{"EURUSD", "GBPUSD"}, "EURUSD,GBPUSD"},
		{"deduped", []string{"EURUSD", "EURUSD"}, "EURUSD"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payloads := make(chan []byte)
			client, captured := newQuoteTestClient(t, payloads)

			prePush(payloads, resp)
			if _, err := client.PollQuotes(context.Background(), tc.symbols, time.Hour); err != nil {
				t.Fatalf("poll quotes: %v", err)
			}

			r := protocol.NewReader(*captured)
			got := r.ReadString()
			if r.Err() != nil {
				t.Fatalf("decode group param: %v", r.Err())
			}
			if got != tc.want {
				t.Errorf("group param: expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestPollQuotesEmitsOnlyChanged(t *testing.T) {
	base := loadSymbolRecord(t)
	payloads := make(chan []byte)
	client, _ := newQuoteTestClient(t, payloads)

	prePush(payloads, symbolsResponse(
		symbolRecord(t, base, "EURUSD", 1700000000, 1.10000, 1.10010, 0),
		symbolRecord(t, base, "GBPUSD", 1700000000, 2.20000, 2.20020, 0),
	))
	quotes, err := client.PollQuotes(context.Background(), []string{"EURUSD", "GBPUSD"}, time.Millisecond)
	if err != nil {
		t.Fatalf("poll quotes: %v", err)
	}
	recvQuotes(t, quotes)

	push(t, payloads, symbolsResponse(
		symbolRecord(t, base, "EURUSD", 1700000005, 1.10050, 1.10060, 0),
		symbolRecord(t, base, "GBPUSD", 1700000000, 2.20000, 2.20020, 0),
	))

	changed := recvQuotes(t, quotes)
	if len(changed) != 1 {
		t.Fatalf("changed size: expected 1, got %d (%v)", len(changed), changed)
	}
	eur, ok := changed["EURUSD"]
	if !ok {
		t.Fatal("changed map missing EURUSD")
	}
	if eur.Bid != 1.10050 {
		t.Errorf("EURUSD bid: expected 1.10050, got %v", eur.Bid)
	}
}

func TestPollQuotesCoalescesForSlowConsumer(t *testing.T) {
	base := loadSymbolRecord(t)
	payloads := make(chan []byte)
	client, _ := newQuoteTestClient(t, payloads)

	rec := func(bid float64) []byte {
		return symbolRecord(t, base, "EURUSD", 1700000000, bid, bid+0.0001, 0)
	}

	prePush(payloads, symbolsResponse(rec(1.10000)))
	quotes, err := client.PollQuotes(context.Background(), []string{"EURUSD"}, time.Millisecond)
	if err != nil {
		t.Fatalf("poll quotes: %v", err)
	}

	push(t, payloads, symbolsResponse(rec(1.20000)))
	push(t, payloads, symbolsResponse(rec(1.30000)))
	push(t, payloads, symbolsResponse(rec(1.30000)))

	snap := recvQuotes(t, quotes)
	if snap["EURUSD"].Bid != 1.10000 {
		t.Fatalf("snapshot bid: expected 1.10000, got %v", snap["EURUSD"].Bid)
	}

	push(t, payloads, symbolsResponse(rec(1.30000)))

	coalesced := recvQuotes(t, quotes)
	got := coalesced["EURUSD"].Bid
	if got == 1.20000 {
		t.Fatal("observed intermediate bid 1.20000 — coalescing should skip it")
	}
	if got != 1.30000 {
		t.Fatalf("coalesced bid: expected 1.30000, got %v", got)
	}
}

func TestPollQuotesValidation(t *testing.T) {
	client, _ := newQuoteTestClient(t, nil)

	cases := []struct {
		name     string
		symbols  []string
		interval time.Duration
	}{
		{"empty list", nil, time.Second},
		{"zero interval", []string{"EURUSD"}, 0},
		{"negative interval", []string{"EURUSD"}, -time.Second},
		{"empty name", []string{""}, time.Second},
		{"comma", []string{"EUR,USD"}, time.Second},
		{"wildcard", []string{"EUR*"}, time.Second},
		{"negation", []string{"!EURUSD"}, time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := client.PollQuotes(context.Background(), tc.symbols, tc.interval); err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestPollQuotesFailFastUnknownSymbol(t *testing.T) {
	base := loadSymbolRecord(t)
	payloads := make(chan []byte)
	client, _ := newQuoteTestClient(t, payloads)

	prePush(payloads, symbolsResponse(
		symbolRecord(t, base, "EURUSD", 1700000000, 1.1, 1.2, 0),
	))

	_, err := client.PollQuotes(context.Background(), []string{"EURUSD", "GBPUSD"}, time.Millisecond)
	if err == nil {
		t.Fatal("expected error for unknown symbol")
	}
	if !strings.Contains(err.Error(), "unknown symbols") || !strings.Contains(err.Error(), "GBPUSD") {
		t.Errorf("error should name GBPUSD as unknown, got: %v", err)
	}
}

func TestPollQuotesFailFastUnselectedSymbol(t *testing.T) {
	base := loadSymbolRecord(t)
	payloads := make(chan []byte)
	client, _ := newQuoteTestClient(t, payloads)

	rec := symbolRecord(t, base, "GBPUSD", 1700000000, 2.1, 2.2, 0)
	rec[symbolRecordOffSelect] = 0

	prePush(payloads, symbolsResponse(
		symbolRecord(t, base, "EURUSD", 1700000000, 1.1, 1.2, 0),
		rec,
	))

	_, err := client.PollQuotes(context.Background(), []string{"EURUSD", "GBPUSD"}, time.Millisecond)
	if err == nil {
		t.Fatal("expected error for unselected symbol")
	}
	if !strings.Contains(err.Error(), "not selected") || !strings.Contains(err.Error(), "GBPUSD") {
		t.Errorf("error should name GBPUSD as not selected, got: %v", err)
	}
}

func TestPollQuotesFailFastRPCError(t *testing.T) {
	payloads := make(chan []byte)
	client, _ := newQuoteTestClient(t, payloads)

	prePush(payloads, nil)

	_, err := client.PollQuotes(context.Background(), []string{"EURUSD"}, time.Millisecond)
	if !errors.Is(err, gomt5.ErrFailed) {
		t.Fatalf("expected ErrFailed from first poll, got: %v", err)
	}
}

func TestPollQuotesWithErrorsRetriesMidStream(t *testing.T) {
	base := loadSymbolRecord(t)
	payloads := make(chan []byte)
	client, _ := newQuoteTestClient(t, payloads)

	prePush(payloads, symbolsResponse(
		symbolRecord(t, base, "EURUSD", 1700000000, 1.10000, 1.10010, 0),
	))
	quotes, errCh, err := client.PollQuotesWithErrors(context.Background(), []string{"EURUSD"}, time.Millisecond)
	if err != nil {
		t.Fatalf("poll quotes: %v", err)
	}
	recvQuotes(t, quotes)

	push(t, payloads, nil)
	select {
	case pollErr := <-errCh:
		if !errors.Is(pollErr, gomt5.ErrFailed) {
			t.Errorf("expected ErrFailed on error channel, got: %v", pollErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for poll error")
	}

	push(t, payloads, symbolsResponse(
		symbolRecord(t, base, "EURUSD", 1700000005, 1.10050, 1.10060, 0),
	))
	changed := recvQuotes(t, quotes)
	if changed["EURUSD"].Bid != 1.10050 {
		t.Errorf("post-error bid: expected 1.10050, got %v", changed["EURUSD"].Bid)
	}
}

func TestPollQuotesFiltersUnrequestedSymbols(t *testing.T) {
	base := loadSymbolRecord(t)
	payloads := make(chan []byte)
	client, _ := newQuoteTestClient(t, payloads)

	prePush(payloads, symbolsResponse(
		symbolRecord(t, base, "EURUSD", 1700000000, 1.10000, 1.10010, 0),
		symbolRecord(t, base, "XAUUSD", 1700000000, 2000, 2001, 0),
	))
	quotes, err := client.PollQuotes(context.Background(), []string{"EURUSD"}, time.Millisecond)
	if err != nil {
		t.Fatalf("poll quotes: %v", err)
	}

	snap := recvQuotes(t, quotes)
	if len(snap) != 1 {
		t.Fatalf("snapshot: expected EURUSD only, got %v", snap)
	}

	push(t, payloads, symbolsResponse(
		symbolRecord(t, base, "EURUSD", 1700000000, 1.10000, 1.10010, 0),
		symbolRecord(t, base, "XAUUSD", 1700000005, 2050, 2051, 0),
	))
	push(t, payloads, symbolsResponse(
		symbolRecord(t, base, "EURUSD", 1700000010, 1.10050, 1.10060, 0),
		symbolRecord(t, base, "XAUUSD", 1700000010, 2060, 2061, 0),
	))

	changed := recvQuotes(t, quotes)
	if len(changed) != 1 {
		t.Fatalf("changed: expected EURUSD only, got %v", changed)
	}
	if _, ok := changed["XAUUSD"]; ok {
		t.Error("XAUUSD leaked into emitted map")
	}
	if changed["EURUSD"].Bid != 1.10050 {
		t.Errorf("EURUSD bid: expected 1.10050, got %v", changed["EURUSD"].Bid)
	}
}

func TestPollQuotesClosesOnContextCancel(t *testing.T) {
	base := loadSymbolRecord(t)
	payloads := make(chan []byte)
	client, _ := newQuoteTestClient(t, payloads)

	ctx, cancel := context.WithCancel(context.Background())
	prePush(payloads, symbolsResponse(
		symbolRecord(t, base, "EURUSD", 1700000000, 1.1, 1.2, 0),
	))
	quotes, err := client.PollQuotes(ctx, []string{"EURUSD"}, time.Hour)
	if err != nil {
		t.Fatalf("poll quotes: %v", err)
	}

	cancel()
	waitClosed(t, quotes)
}

func TestPollQuotesClosesOnClientClose(t *testing.T) {
	base := loadSymbolRecord(t)
	payloads := make(chan []byte)
	client, _ := newQuoteTestClient(t, payloads)

	prePush(payloads, symbolsResponse(
		symbolRecord(t, base, "EURUSD", 1700000000, 1.1, 1.2, 0),
	))
	quotes, errCh, err := client.PollQuotesWithErrors(context.Background(), []string{"EURUSD"}, time.Hour)
	if err != nil {
		t.Fatalf("poll quotes: %v", err)
	}

	client.Close()
	waitClosed(t, quotes)
	waitClosed(t, errCh)
}

func TestPollQuotesAfterCloseFails(t *testing.T) {
	client, _ := newQuoteTestClient(t, nil)
	client.Close()

	_, err := client.PollQuotes(context.Background(), []string{"EURUSD"}, time.Millisecond)
	if !errors.Is(err, gomt5.ErrNotConnected) {
		t.Fatalf("expected ErrNotConnected after Close, got: %v", err)
	}
}

func TestSymbolInfoTickConversion(t *testing.T) {
	s := &gomt5.SymbolInfo{
		Time:       1700000123,
		Bid:        1.10000,
		Ask:        1.10012,
		Last:       1.10006,
		Volume:     42,
		VolumeReal: 4.2,
	}
	tick := s.Tick()

	if tick.Time != 1700000123 {
		t.Errorf("Time: got %d", tick.Time)
	}
	if tick.Bid != 1.10000 || tick.Ask != 1.10012 || tick.Last != 1.10006 {
		t.Errorf("quote fields: got Bid=%v Ask=%v Last=%v", tick.Bid, tick.Ask, tick.Last)
	}
	if tick.Volume != 42 {
		t.Errorf("Volume: got %d", tick.Volume)
	}
	if tick.TimeMsc != 1700000123000 {
		t.Errorf("TimeMsc: expected Time*1000, got %d", tick.TimeMsc)
	}
	if tick.Flags != 0 {
		t.Errorf("Flags: expected 0, got %d", tick.Flags)
	}
	if tick.VolumeReal != 4.2 {
		t.Errorf("VolumeReal: got %v", tick.VolumeReal)
	}
}

func TestPollQuotesEmittedMapsAreDistinct(t *testing.T) {
	base := loadSymbolRecord(t)
	payloads := make(chan []byte)
	client, _ := newQuoteTestClient(t, payloads)

	rec := func(bid float64) []byte {
		return symbolRecord(t, base, "EURUSD", 1700000000, bid, bid+0.0001, 0)
	}

	prePush(payloads, symbolsResponse(rec(1.10000)))
	quotes, err := client.PollQuotes(context.Background(), []string{"EURUSD"}, time.Millisecond)
	if err != nil {
		t.Fatalf("poll quotes: %v", err)
	}

	first := recvQuotes(t, quotes)
	first["EURUSD"] = gomt5.Tick{Bid: 999}
	first["INJECTED"] = gomt5.Tick{Bid: 666}

	push(t, payloads, symbolsResponse(rec(1.20000)))
	second := recvQuotes(t, quotes)
	if second["EURUSD"].Bid != 1.20000 {
		t.Fatalf("second map polluted by consumer mutation: %v", second)
	}
	if _, ok := second["INJECTED"]; ok {
		t.Fatal("emitted maps share storage with previously emitted map")
	}

	delete(second, "EURUSD")
	push(t, payloads, symbolsResponse(rec(1.30000)))
	third := recvQuotes(t, quotes)
	if third["EURUSD"].Bid != 1.30000 {
		t.Fatalf("third map affected by consumer mutation of second: %v", third)
	}
}
