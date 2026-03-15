package gomt5_test

import (
	"context"
	"encoding/binary"
	"io"
	"math"
	"testing"
	"time"
	"unicode/utf16"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

type mockPipe struct {
	reader *io.PipeReader
	writer *io.PipeWriter

	serverReader *io.PipeReader
	serverWriter *io.PipeWriter

	handler func(cmdID uint32, params []byte) (bool, []byte)
	done    chan struct{}
}

func newMockPipe(t testing.TB, handler func(cmdID uint32, params []byte) (bool, []byte)) *mockPipe {
	t.Helper()
	clientRead, serverWrite := io.Pipe()
	serverRead, clientWrite := io.Pipe()

	m := &mockPipe{
		reader:       clientRead,
		writer:       clientWrite,
		serverReader: serverRead,
		serverWriter: serverWrite,
		handler:      handler,
		done:         make(chan struct{}),
	}
	go m.serve(t)
	return m
}

func (m *mockPipe) Read(p []byte) (int, error)  { return m.reader.Read(p) }
func (m *mockPipe) Write(p []byte) (int, error) { return m.writer.Write(p) }
func (m *mockPipe) Close() error {
	select {
	case <-m.done:
		return nil
	default:
		close(m.done)
	}
	m.reader.Close()
	m.writer.Close()
	m.serverReader.Close()
	m.serverWriter.Close()
	return nil
}

func (m *mockPipe) serve(t testing.TB) {
	for {
		select {
		case <-m.done:
			return
		default:
		}

		lenBuf := make([]byte, 4)
		if _, err := io.ReadFull(m.serverReader, lenBuf); err != nil {
			return
		}
		payloadLen := binary.LittleEndian.Uint32(lenBuf)
		payload := make([]byte, payloadLen)
		if _, err := io.ReadFull(m.serverReader, payload); err != nil {
			return
		}

		cmdID := binary.LittleEndian.Uint32(payload[0:4])
		var params []byte
		if len(payload) > 4 {
			params = payload[4:]
		}

		success, data := m.handler(cmdID, params)

		respLen := uint32(8 + len(data))
		header := make([]byte, 4)
		binary.LittleEndian.PutUint32(header, respLen)
		m.serverWriter.Write(header)

		body := make([]byte, 8+len(data))
		binary.LittleEndian.PutUint32(body[0:4], cmdID)
		if success {
			binary.LittleEndian.PutUint32(body[4:8], 1)
		}
		copy(body[8:], data)
		m.serverWriter.Write(body)
	}
}

func writeU32(buf []byte, v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return append(buf, b...)
}

func writeI64(buf []byte, v int64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(v))
	return append(buf, b...)
}

func writeF64(buf []byte, v float64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, math.Float64bits(v))
	return append(buf, b...)
}

func writeStr(buf []byte, s string) []byte {
	runes := utf16.Encode([]rune(s))
	buf = writeU32(buf, uint32(len(runes)))
	for _, r := range runes {
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, r)
		buf = append(buf, b...)
	}
	return buf
}

func TestInitAndVersion(t *testing.T) {
	ctx := context.Background()
	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		switch cmdID {
		case protocol.CmdInitialize:
			return true, writeU32(nil, 5684)
		case protocol.CmdTerminalInfo:
			var data []byte
			data = writeI64(data, 500)
			data = writeI64(data, 5684)
			data = writeStr(data, "01 Jan 2025")
			return true, data
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	if client.Build() != 5684 {
		t.Errorf("expected build 5684, got %d", client.Build())
	}

	ver, err := client.Version(ctx)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if ver.Build != 5684 {
		t.Errorf("expected build 5684, got %d", ver.Build)
	}
}

func TestAccountInfo(t *testing.T) {
	ctx := context.Background()
	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		if cmdID == protocol.CmdAccountInfo {
			var d []byte
			d = writeI64(d, 12345)    // login
			d = writeI64(d, 0)        // trade_mode
			d = writeI64(d, 100)      // leverage
			d = writeI64(d, 200)      // limit_orders
			d = writeI64(d, 0)        // margin_so_mode
			d = writeI64(d, 1)        // trade_allowed
			d = writeI64(d, 1)        // trade_expert
			d = writeI64(d, 0)        // margin_mode
			d = writeI64(d, 2)        // currency_digits
			d = writeI64(d, 0)        // fifo_close
			d = writeF64(d, 10000.50) // balance
			d = writeF64(d, 0)        // credit
			d = writeF64(d, 150.25)   // profit
			d = writeF64(d, 10150.75) // equity
			d = writeF64(d, 500.0)    // margin
			d = writeF64(d, 9650.75)  // free_margin
			d = writeF64(d, 2030.15)  // margin_level
			d = writeF64(d, 50.0)     // margin_so_call
			d = writeF64(d, 30.0)     // margin_so_so
			d = writeF64(d, 0)        // margin_initial
			d = writeF64(d, 0)        // margin_maintenance
			d = writeF64(d, 0)        // assets
			d = writeF64(d, 0)        // liabilities
			d = writeF64(d, 0)        // commission_blocked
			d = writeStr(d, "Test User")
			d = writeStr(d, "MetaQuotes-Demo")
			d = writeStr(d, "USD")
			d = writeStr(d, "MetaQuotes Ltd.")
			return true, d
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	info, err := client.AccountInfo(ctx)
	if err != nil {
		t.Fatalf("account info: %v", err)
	}
	if info.Login != 12345 {
		t.Errorf("login: expected 12345, got %d", info.Login)
	}
	if info.Balance != 10000.50 {
		t.Errorf("balance: expected 10000.50, got %f", info.Balance)
	}
	if info.Leverage != 100 {
		t.Errorf("leverage: expected 100, got %d", info.Leverage)
	}
	if !info.TradeAllowed {
		t.Error("expected trade_allowed true")
	}
	if info.Currency != "USD" {
		t.Errorf("currency: expected USD, got %s", info.Currency)
	}
	if info.Server != "MetaQuotes-Demo" {
		t.Errorf("server: expected MetaQuotes-Demo, got %s", info.Server)
	}
}

func TestSymbolsTotal(t *testing.T) {
	ctx := context.Background()
	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		if cmdID == protocol.CmdSymbolsTotal {
			return true, writeU32(nil, 250)
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	total, err := client.SymbolsTotal(ctx)
	if err != nil {
		t.Fatalf("symbols total: %v", err)
	}
	if total != 250 {
		t.Errorf("expected 250, got %d", total)
	}
}

func TestSymbolInfoTick(t *testing.T) {
	ctx := context.Background()
	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		if cmdID == protocol.CmdSymbolInfoTick {
			var d []byte
			d = writeI64(d, 1710000000)    // time
			d = writeF64(d, 1.08500)       // bid
			d = writeF64(d, 1.08520)       // ask
			d = writeF64(d, 0)             // last
			d = writeI64(d, 0)             // volume (u64)
			d = writeI64(d, 1710000000123) // time_msc
			d = writeU32(d, 6)             // flags
			return true, d
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	tick, err := client.SymbolInfoTick(ctx, "EURUSD")
	if err != nil {
		t.Fatalf("symbol info tick: %v", err)
	}
	if tick.Bid != 1.08500 {
		t.Errorf("bid: expected 1.08500, got %f", tick.Bid)
	}
	if tick.Ask != 1.08520 {
		t.Errorf("ask: expected 1.08520, got %f", tick.Ask)
	}
	if tick.TimeMsc != 1710000000123 {
		t.Errorf("time_msc: expected 1710000000123, got %d", tick.TimeMsc)
	}
}

func TestCopyRatesFromPos(t *testing.T) {
	ctx := context.Background()
	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		if cmdID == protocol.CmdCopyRatesFromPos {
			var d []byte
			d = writeU32(d, 2) // count

			d = writeI64(d, 1710000000)
			d = writeF64(d, 1.0850)
			d = writeF64(d, 1.0860)
			d = writeF64(d, 1.0840)
			d = writeF64(d, 1.0855)
			d = writeI64(d, 100)
			d = writeU32(d, 5)
			d = writeI64(d, 0)

			d = writeI64(d, 1710003600)
			d = writeF64(d, 1.0855)
			d = writeF64(d, 1.0870)
			d = writeF64(d, 1.0845)
			d = writeF64(d, 1.0865)
			d = writeI64(d, 150)
			d = writeU32(d, 3)
			d = writeI64(d, 0)

			return true, d
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	rates, err := client.CopyRatesFromPos(ctx, "EURUSD", gomt5.TimeframeH1, 0, 2)
	if err != nil {
		t.Fatalf("copy rates: %v", err)
	}
	if len(rates) != 2 {
		t.Fatalf("expected 2 rates, got %d", len(rates))
	}
	if rates[0].Open != 1.0850 {
		t.Errorf("open: expected 1.0850, got %f", rates[0].Open)
	}
	if rates[1].Close != 1.0865 {
		t.Errorf("close: expected 1.0865, got %f", rates[1].Close)
	}
}

func TestErrorResponse(t *testing.T) {
	ctx := context.Background()
	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		var d []byte
		d = writeU32(d, uint32(0xFFFFFFFF-12))
		d = writeStr(d, "invalid parameters")
		return false, d
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	_, err = client.SymbolsTotal(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPositionsTotal(t *testing.T) {
	ctx := context.Background()
	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		if cmdID == protocol.CmdPositionsTotal {
			return true, writeU32(nil, 3)
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	total, err := client.PositionsTotal(ctx)
	if err != nil {
		t.Fatalf("positions total: %v", err)
	}
	if total != 3 {
		t.Errorf("expected 3, got %d", total)
	}
}

func mockPosition(ticket int64, symbol string) []byte {
	var d []byte
	d = writeI64(d, ticket)
	d = writeI64(d, 1710000000)
	d = writeI64(d, 1710000000000)
	d = writeI64(d, 1710000000)
	d = writeI64(d, 1710000000000)
	d = writeI64(d, 0)
	d = writeI64(d, 12345)
	d = writeI64(d, ticket)
	d = writeI64(d, 0)
	d = writeF64(d, 0.1)
	d = writeF64(d, 1.0850)
	d = writeF64(d, 1.0860)
	d = writeF64(d, 1.0800)
	d = writeF64(d, 1.0900)
	d = writeF64(d, -0.5)
	d = writeF64(d, 10.0)
	d = writeStr(d, symbol)
	d = writeStr(d, "")
	d = writeStr(d, "")
	return d
}

func TestPositionsGetWithFilter(t *testing.T) {
	ctx := context.Background()
	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		switch cmdID {
		case protocol.CmdPositionsGet:
			var d []byte
			d = writeU32(d, 2)
			d = append(d, mockPosition(100, "EURUSD")...)
			d = append(d, mockPosition(200, "USDJPY")...)
			return true, d
		case protocol.CmdPositionsGetByTicket:
			var d []byte
			d = writeU32(d, 1)
			d = append(d, mockPosition(100, "EURUSD")...)
			return true, d
		case protocol.CmdPositionsGetBySymbol:
			var d []byte
			d = writeU32(d, 1)
			d = append(d, mockPosition(100, "EURUSD")...)
			return true, d
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	all, err := client.PositionsGet(ctx, nil)
	if err != nil {
		t.Fatalf("positions get: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(all))
	}

	byTicket, err := client.PositionsGet(ctx, &gomt5.PositionFilter{Ticket: 100})
	if err != nil {
		t.Fatalf("positions get by ticket: %v", err)
	}
	if len(byTicket) != 1 || byTicket[0].Ticket != 100 {
		t.Errorf("expected 1 position with ticket 100, got %d", len(byTicket))
	}

	byGroup, err := client.PositionsGet(ctx, &gomt5.PositionFilter{Group: "EUR*"})
	if err != nil {
		t.Fatalf("positions get by group: %v", err)
	}
	if len(byGroup) != 1 || byGroup[0].Symbol != "EURUSD" {
		t.Errorf("expected 1 EURUSD position, got %d", len(byGroup))
	}
}

func mockOrder(ticket int64, symbol string) []byte {
	var d []byte
	d = writeI64(d, ticket)
	d = writeI64(d, 1710000000)
	d = writeI64(d, 1710000000000)
	d = writeI64(d, 0)
	d = writeI64(d, 0)
	d = writeI64(d, 0)
	d = writeI64(d, 2)
	d = writeI64(d, 0)
	d = writeI64(d, 0)
	d = writeI64(d, 0)
	d = writeI64(d, 12345)
	d = writeI64(d, 0)
	d = writeI64(d, 0)
	d = writeI64(d, 0)
	d = writeF64(d, 0.1)
	d = writeF64(d, 0.1)
	d = writeF64(d, 1.0850)
	d = writeF64(d, 1.0860)
	d = writeF64(d, 0)
	d = writeF64(d, 0)
	d = writeF64(d, 0)
	d = writeStr(d, symbol)
	d = writeStr(d, "")
	d = writeStr(d, "")
	return d
}

func TestOrdersGetWithFilter(t *testing.T) {
	ctx := context.Background()
	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		switch cmdID {
		case protocol.CmdOrdersGet:
			var d []byte
			d = writeU32(d, 2)
			d = append(d, mockOrder(1001, "EURUSD")...)
			d = append(d, mockOrder(1002, "GBPUSD")...)
			return true, d
		case protocol.CmdOrdersGetByTicket:
			var d []byte
			d = writeU32(d, 1)
			d = append(d, mockOrder(1001, "EURUSD")...)
			return true, d
		case protocol.CmdOrdersGetBySymbol:
			var d []byte
			d = writeU32(d, 1)
			d = append(d, mockOrder(1001, "EURUSD")...)
			return true, d
		}
		return false, nil
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	all, err := client.OrdersGet(ctx, nil)
	if err != nil {
		t.Fatalf("orders get: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(all))
	}

	byTicket, err := client.OrdersGet(ctx, &gomt5.OrderFilter{Ticket: 1001})
	if err != nil {
		t.Fatalf("orders get by ticket: %v", err)
	}
	if len(byTicket) != 1 || byTicket[0].Ticket != 1001 {
		t.Errorf("expected 1 order with ticket 1001")
	}
}

func TestEnumStrings(t *testing.T) {
	if gomt5.TimeframeH1.String() != "H1" {
		t.Errorf("expected H1, got %s", gomt5.TimeframeH1.String())
	}
	if gomt5.OrderTypeBuy.String() != "Buy" {
		t.Errorf("expected Buy, got %s", gomt5.OrderTypeBuy.String())
	}
	if gomt5.TradeActionDeal.String() != "Deal" {
		t.Errorf("expected Deal, got %s", gomt5.TradeActionDeal.String())
	}
	if gomt5.DealEntryIn.String() != "In" {
		t.Errorf("expected In, got %s", gomt5.DealEntryIn.String())
	}
	if gomt5.PositionTypeSell.String() != "Sell" {
		t.Errorf("expected Sell, got %s", gomt5.PositionTypeSell.String())
	}
}

func TestTradeResultIsOK(t *testing.T) {
	ok := &gomt5.TradeResult{Retcode: gomt5.RetcodeDone}
	if !ok.IsOK() {
		t.Error("expected IsOK true for RetcodeDone")
	}

	fail := &gomt5.TradeResult{Retcode: gomt5.RetcodeReject}
	if fail.IsOK() {
		t.Error("expected IsOK false for RetcodeReject")
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		return true, writeU32(nil, 250)
	})
	defer mock.Close()

	client, err := gomt5.NewClientFromConn(ctx, mock)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	cancel()

	_, err = client.SymbolsTotal(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestLoggerOption(t *testing.T) {
	ctx := context.Background()
	var logged bool

	mock := newMockPipe(t, func(cmdID uint32, params []byte) (bool, []byte) {
		if cmdID == protocol.CmdInitialize {
			return true, writeU32(nil, 5684)
		}
		return true, writeU32(nil, 42)
	})
	defer mock.Close()

	var hookCalled bool
	client, err := gomt5.NewClientFromConn(ctx, mock,
		gomt5.WithLogger(testLogger{onLog: func() { logged = true }}),
		gomt5.WithOnRequest(func(cmdID uint32, duration time.Duration, err error) {
			hookCalled = true
		}),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	_, _ = client.SymbolsTotal(ctx)
	if !logged {
		t.Error("expected logger to be called")
	}
	if !hookCalled {
		t.Error("expected OnRequest hook to be called")
	}
}

type testLogger struct {
	onLog func()
}

func (l testLogger) Debug(msg string, args ...any) {
	if l.onLog != nil {
		l.onLog()
	}
}
