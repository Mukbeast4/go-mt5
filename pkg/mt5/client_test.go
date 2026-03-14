package mt5_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/mukbeast4/go-mt5/internal/protocol"
	"github.com/mukbeast4/go-mt5/pkg/mt5"
)

type mockServer struct {
	listener net.Listener
	handler  func(req protocol.Request) protocol.Response
	done     chan struct{}
}

func newMockServer(t *testing.T, handler func(req protocol.Request) protocol.Response) *mockServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &mockServer{listener: ln, handler: handler, done: make(chan struct{})}
	go s.serve(t)
	return s
}

func (s *mockServer) addr() string {
	return s.listener.Addr().String()
}

func (s *mockServer) close() {
	close(s.done)
	s.listener.Close()
}

func (s *mockServer) serve(t *testing.T) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				t.Logf("accept error: %v", err)
			}
			return
		}
		go s.handleConn(t, conn)
	}
}

func (s *mockServer) handleConn(t *testing.T, conn net.Conn) {
	defer conn.Close()
	for {
		header := make([]byte, 4)
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}
		size := binary.BigEndian.Uint32(header)
		payload := make([]byte, size)
		if _, err := io.ReadFull(conn, payload); err != nil {
			return
		}

		var req protocol.Request
		if err := json.Unmarshal(payload, &req); err != nil {
			t.Logf("unmarshal request: %v", err)
			return
		}

		resp := s.handler(req)
		respData, _ := json.Marshal(resp)

		respHeader := make([]byte, 4)
		binary.BigEndian.PutUint32(respHeader, uint32(len(respData)))
		conn.Write(respHeader)
		conn.Write(respData)
	}
}

func TestVersion(t *testing.T) {
	srv := newMockServer(t, func(req protocol.Request) protocol.Response {
		data, _ := json.Marshal(mt5.VersionInfo{
			Version:   "5.0.45",
			Build:     4500,
			BuildDate: "2025-01-01",
			EAVersion: "1.0.0",
		})
		return protocol.Response{
			ID:      req.ID,
			Type:    protocol.TypeResponse,
			Success: true,
			Data:    data,
		}
	})
	defer srv.close()

	client, err := mt5.NewClient(mt5.WithAddress(srv.addr()))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := client.Version(ctx)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if info.Version != "5.0.45" {
		t.Errorf("expected version 5.0.45, got %s", info.Version)
	}
	if info.Build != 4500 {
		t.Errorf("expected build 4500, got %d", info.Build)
	}
}

func TestAccountInfo(t *testing.T) {
	srv := newMockServer(t, func(req protocol.Request) protocol.Response {
		data, _ := json.Marshal(mt5.AccountInfo{
			Login:    12345,
			Name:     "Test Account",
			Balance:  10000.0,
			Equity:   10050.0,
			Currency: "USD",
			Leverage: 100,
		})
		return protocol.Response{
			ID:      req.ID,
			Type:    protocol.TypeResponse,
			Success: true,
			Data:    data,
		}
	})
	defer srv.close()

	client, err := mt5.NewClient(mt5.WithAddress(srv.addr()))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := client.AccountInfo(ctx)
	if err != nil {
		t.Fatalf("account info: %v", err)
	}
	if info.Login != 12345 {
		t.Errorf("expected login 12345, got %d", info.Login)
	}
	if info.Balance != 10000.0 {
		t.Errorf("expected balance 10000, got %f", info.Balance)
	}
}

func TestSymbolInfo(t *testing.T) {
	srv := newMockServer(t, func(req protocol.Request) protocol.Response {
		data, _ := json.Marshal(mt5.SymbolInfo{
			Name:   "EURUSD",
			Bid:    1.0850,
			Ask:    1.0852,
			Digits: 5,
			Point:  0.00001,
		})
		return protocol.Response{
			ID:      req.ID,
			Type:    protocol.TypeResponse,
			Success: true,
			Data:    data,
		}
	})
	defer srv.close()

	client, err := mt5.NewClient(mt5.WithAddress(srv.addr()))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := client.SymbolInfo(ctx, "EURUSD")
	if err != nil {
		t.Fatalf("symbol info: %v", err)
	}
	if info.Name != "EURUSD" {
		t.Errorf("expected EURUSD, got %s", info.Name)
	}
	if info.Digits != 5 {
		t.Errorf("expected 5 digits, got %d", info.Digits)
	}
}

func TestCopyRatesFrom(t *testing.T) {
	srv := newMockServer(t, func(req protocol.Request) protocol.Response {
		rates := []mt5.Rate{
			{Time: 1710000000, Open: 1.0850, High: 1.0860, Low: 1.0840, Close: 1.0855, TickVolume: 100},
			{Time: 1710003600, Open: 1.0855, High: 1.0870, Low: 1.0845, Close: 1.0865, TickVolume: 150},
		}
		data, _ := json.Marshal(rates)
		return protocol.Response{
			ID:      req.ID,
			Type:    protocol.TypeResponse,
			Success: true,
			Data:    data,
		}
	})
	defer srv.close()

	client, err := mt5.NewClient(mt5.WithAddress(srv.addr()))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rates, err := client.CopyRatesFrom(ctx, "EURUSD", mt5.TimeframeH1, 1710000000, 2)
	if err != nil {
		t.Fatalf("copy rates: %v", err)
	}
	if len(rates) != 2 {
		t.Fatalf("expected 2 rates, got %d", len(rates))
	}
	if rates[0].Open != 1.0850 {
		t.Errorf("expected open 1.0850, got %f", rates[0].Open)
	}
}

func TestOrderSend(t *testing.T) {
	srv := newMockServer(t, func(req protocol.Request) protocol.Response {
		data, _ := json.Marshal(mt5.TradeResult{
			Retcode: mt5.ErrCodeDone,
			Deal:    100001,
			Order:   200001,
			Volume:  0.1,
			Price:   1.0850,
		})
		return protocol.Response{
			ID:      req.ID,
			Type:    protocol.TypeResponse,
			Success: true,
			Data:    data,
		}
	})
	defer srv.close()

	client, err := mt5.NewClient(mt5.WithAddress(srv.addr()))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.OrderSend(ctx, mt5.TradeRequest{
		Action: mt5.TradeActionDeal,
		Symbol: "EURUSD",
		Volume: 0.1,
		Type:   mt5.OrderTypeBuy,
		Price:  1.0850,
	})
	if err != nil {
		t.Fatalf("order send: %v", err)
	}
	if result.Retcode != mt5.ErrCodeDone {
		t.Errorf("expected retcode %d, got %d", mt5.ErrCodeDone, result.Retcode)
	}
	if result.Deal != 100001 {
		t.Errorf("expected deal 100001, got %d", result.Deal)
	}
}

func TestErrorResponse(t *testing.T) {
	srv := newMockServer(t, func(req protocol.Request) protocol.Response {
		return protocol.Response{
			ID:      req.ID,
			Type:    protocol.TypeResponse,
			Success: false,
			Error: &protocol.ResponseError{
				Code:    mt5.ErrCodeInvalidParams,
				Message: "invalid parameters",
			},
		}
	})
	defer srv.close()

	client, err := mt5.NewClient(mt5.WithAddress(srv.addr()))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.AccountInfo(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	mt5Err, ok := err.(*mt5.MT5Error)
	if !ok {
		t.Fatalf("expected MT5Error, got %T: %v", err, err)
	}
	if mt5Err.Code != mt5.ErrCodeInvalidParams {
		t.Errorf("expected code %d, got %d", mt5.ErrCodeInvalidParams, mt5Err.Code)
	}
}

func TestMultipleConcurrentRequests(t *testing.T) {
	srv := newMockServer(t, func(req protocol.Request) protocol.Response {
		var data json.RawMessage
		switch req.Action {
		case "account_info":
			data, _ = json.Marshal(mt5.AccountInfo{Login: 12345, Balance: 10000})
		case "symbols_total":
			data, _ = json.Marshal(42)
		default:
			data, _ = json.Marshal(nil)
		}
		return protocol.Response{
			ID:      req.ID,
			Type:    protocol.TypeResponse,
			Success: true,
			Data:    data,
		}
	})
	defer srv.close()

	client, err := mt5.NewClient(mt5.WithAddress(srv.addr()))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh := make(chan error, 2)

	go func() {
		info, err := client.AccountInfo(ctx)
		if err != nil {
			errCh <- fmt.Errorf("account: %w", err)
			return
		}
		if info.Login != 12345 {
			errCh <- fmt.Errorf("expected login 12345, got %d", info.Login)
			return
		}
		errCh <- nil
	}()

	go func() {
		total, err := client.SymbolsTotal(ctx)
		if err != nil {
			errCh <- fmt.Errorf("symbols: %w", err)
			return
		}
		if total != 42 {
			errCh <- fmt.Errorf("expected 42 symbols, got %d", total)
			return
		}
		errCh <- nil
	}()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Error(err)
		}
	}
}
