# go-mt5

[![Go Reference](https://pkg.go.dev/badge/github.com/mukbeast4/go-mt5.svg)](https://pkg.go.dev/github.com/mukbeast4/go-mt5)
[![Go Report Card](https://goreportcard.com/badge/github.com/mukbeast4/go-mt5)](https://goreportcard.com/report/github.com/mukbeast4/go-mt5)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/Mukbeast4/go-mt5)](https://github.com/Mukbeast4/go-mt5/releases)

Native Go client for MetaTrader 5. Communicates directly with the MT5 terminal via Windows Named Pipe IPC, the same mechanism used by the official Python `MetaTrader5` package. No Expert Advisor needed.

## Features

- Direct IPC connection to `terminal64.exe` via Windows Named Pipes
- Reverse-engineered binary protocol (same as `MetaTrader5.pyd`)
- `context.Context` support on all methods for cancellation and timeouts
- Account and terminal information
- Real-time symbol info, tick data, and market book (depth of market)
- Historical rates and ticks (OHLCV, copy from position/date/range)
- Order management (send, check, calc margin/profit)
- Position and history queries with rich filters (symbol, ticket, group)
- Pluggable logger and request hooks for observability
- Zero external dependencies (only `golang.org/x/sys`)

## Requirements

- Windows (named pipes are a Windows kernel feature)
- MetaTrader 5 terminal running with an active account
- Go 1.25+

## Installation

```bash
go get github.com/mukbeast4/go-mt5
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	gomt5 "github.com/mukbeast4/go-mt5"
)

func main() {
	ctx := context.Background()

	client, err := gomt5.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	fmt.Printf("Connected to MT5 build %d\n", client.Build())

	account, err := client.AccountInfo(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Account: %s Balance: %.2f %s\n",
		account.Name, account.Balance, account.Currency)

	tick, err := client.SymbolInfoTick(ctx, "EURUSD")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("EURUSD Bid: %.5f Ask: %.5f\n", tick.Bid, tick.Ask)
}
```

## Examples

The `examples/` directory contains runnable programs covering the main use cases:

| Example | What it demonstrates |
|---------|----------------------|
| `examples/connect` | Connect, read account / terminal info, count symbols |
| `examples/rates` | Fetch OHLCV bars via `CopyRatesFromPos` |
| `examples/ticks` | Fetch recent ticks via `CopyTicksFrom` |
| `examples/stream` | Subscribe to live ticks across multiple symbols |
| `examples/trade` | Place a market order with `tradeutil.Buy` |
| `examples/risk` | Position sizing from balance, symbol info, and current tick |

Run any example with `go run ./examples/<name>` on a Windows host with MT5 running.

## API

| Group | Methods |
|-------|---------|
| Connection | `NewClient(ctx)`, `NewClientFromConn(ctx, conn)`, `Close()`, `Build()`, `LastError()` |
| Account | `AccountInfo(ctx)`, `TerminalInfo(ctx)`, `Version(ctx)` |
| Symbols | `SymbolsTotal(ctx)`, `SymbolsGet(ctx, group)`, `SymbolInfo(ctx, symbol)`, `SymbolInfoTick(ctx, symbol)`, `SymbolSelect(ctx, symbol, enable)` |
| Market Data | `CopyRatesFrom(ctx, ...)`, `CopyRatesFromPos(ctx, ...)`, `CopyRatesRange(ctx, ...)`, `CopyTicksFrom(ctx, ...)`, `CopyTicksRange(ctx, ...)` |
| Market Book | `MarketBookAdd(ctx, symbol)`, `MarketBookGet(ctx, symbol)`, `MarketBookRelease(ctx, symbol)` |
| Trading | `OrderSend(ctx, req)`, `OrderCheck(ctx, req)`, `OrderCalcMargin(ctx, ...)`, `OrderCalcProfit(ctx, ...)`, `OrdersTotal(ctx)`, `OrdersGet(ctx, *OrderFilter)` |
| Positions | `PositionsTotal(ctx)`, `PositionsGet(ctx, *PositionFilter)` |
| History | `HistoryOrdersTotal(ctx, from, to)`, `HistoryOrdersGet(ctx, *HistoryFilter)`, `HistoryDealsTotal(ctx, from, to)`, `HistoryDealsGet(ctx, *HistoryFilter)` |
| Debug | `SendRaw(ctx, cmdID, params)` |

### Options

```go
gomt5.NewClient(ctx,
	gomt5.WithPipeName(`\\.\pipe\MT5.123456`),
	gomt5.WithTimeout(30*time.Second),
	gomt5.WithDebug(true),
	gomt5.WithLogger(myLogger),
	gomt5.WithOnRequest(func(cmdID uint32, d time.Duration, err error) {
		// metrics hook
	}),
)
```

### Filters

```go
positions, _ := client.PositionsGet(ctx, &gomt5.PositionFilter{
	Symbol: "EURUSD",
})

positions, _ = client.PositionsGet(ctx, &gomt5.PositionFilter{
	Ticket: 12345,
})

positions, _ = client.PositionsGet(ctx, &gomt5.PositionFilter{
	Group: "EUR*",
})

// nil filter returns all
positions, _ = client.PositionsGet(ctx, nil)
```

## Protocol

```
Request:  [payload_len:LE32][cmd_id:LE32][params...]
Response: [payload_len:LE32][cmd_echo:LE32][success:LE32][data...]
```

- Integers: LE32/LE64
- Floats: IEEE 754 double LE64
- Strings: `[char_count:LE32][UTF-16LE data]`
- Booleans: LE64 (0/1)

## Python API Compatibility

| Python function | go-mt5 equivalent | Status |
|----------------|-------------------|--------|
| `initialize()` | `NewClient(ctx)` | Done |
| `shutdown()` | `Close()` | Done |
| `login()` | `Login(ctx, login, password, server)` | Done |
| `version()` | `Version(ctx)` | Done |
| `account_info()` | `AccountInfo(ctx)` | Done |
| `terminal_info()` | `TerminalInfo(ctx)` | Done |
| `symbols_total()` | `SymbolsTotal(ctx)` | Done |
| `symbols_get()` | `SymbolsGet(ctx, group)` | Done |
| `symbol_info()` | `SymbolInfo(ctx, symbol)` | Done |
| `symbol_info_tick()` | `SymbolInfoTick(ctx, symbol)` | Done |
| `symbol_select()` | `SymbolSelect(ctx, symbol, enable)` | Done |
| `market_book_add()` | `MarketBookAdd(ctx, symbol)` | Done |
| `market_book_get()` | `MarketBookGet(ctx, symbol)` | Done |
| `market_book_release()` | `MarketBookRelease(ctx, symbol)` | Done |
| `copy_rates_from()` | `CopyRatesFrom(ctx, ...)` | Done |
| `copy_rates_from_pos()` | `CopyRatesFromPos(ctx, ...)` | Done |
| `copy_rates_range()` | `CopyRatesRange(ctx, ...)` | Done |
| `copy_ticks_from()` | `CopyTicksFrom(ctx, ...)` | Done |
| `copy_ticks_range()` | `CopyTicksRange(ctx, ...)` | Done |
| `orders_total()` | `OrdersTotal(ctx)` | Done |
| `orders_get()` | `OrdersGet(ctx, *OrderFilter)` | Done |
| `positions_total()` | `PositionsTotal(ctx)` | Done |
| `positions_get()` | `PositionsGet(ctx, *PositionFilter)` | Done |
| `history_orders_total()` | `HistoryOrdersTotal(ctx, from, to)` | Done |
| `history_orders_get()` | `HistoryOrdersGet(ctx, *HistoryFilter)` | Done |
| `history_deals_total()` | `HistoryDealsTotal(ctx, from, to)` | Done |
| `history_deals_get()` | `HistoryDealsGet(ctx, *HistoryFilter)` | Done |
| `order_send()` | `OrderSend(ctx, req)` | Done |
| `order_check()` | `OrderCheck(ctx, req)` | Done |
| `order_calc_margin()` | `OrderCalcMargin(ctx, ...)` | Done |
| `order_calc_profit()` | `OrderCalcProfit(ctx, ...)` | Done |
| `last_error()` | `LastError()` | Done |

### go-mt5 extras

| Feature | Method |
|---------|--------|
| Request hooks | `WithOnRequest(hook)` |
| Debug logging | `WithDebug(true)`, `WithLogger(l)` |
| Tick streaming (polling) | `SubscribeTicks(ctx, symbol)` |
| Trade helpers | `tradeutil.Buy()`, `Sell()`, `ClosePosition()`, etc. |
| Data analysis | `analysis.NewRateSeries()`, `SMA()`, `EMA()` |
| CSV export | `analysis.ToCSV(writer, rates)` |
| Time helpers | `ToTime()`, `FromTime()`, `.TimeUTC()` methods |
| Request validation | `TradeRequest.Validate()` |
| Auto-reconnect | `WithAutoReconnect(true)` |

## Limitations

- **Windows only**: Named pipes are a Windows kernel feature. The library cannot connect to MT5 from Linux or macOS (cross-compilation works, but runtime requires Windows).
- **Reverse-engineered protocol**: The binary protocol is not documented by MetaQuotes. Command IDs and payload formats were discovered by analyzing the official Python `MetaTrader5.pyd` library.
- **MT5 build compatibility**: MetaQuotes may change the pipe protocol between MT5 builds without notice. If something breaks after an MT5 update, please open an issue with your build number.
- **Broker differences**: Different brokers configure their MT5 servers differently. Filling modes, symbol visibility, available order types, and margin calculation methods vary. Always check `SymbolInfo` before trading.
- **Single terminal**: The library connects to one MT5 terminal instance. If multiple terminals are running, use `WithPipeName` to target a specific one.
- **Tick streaming via polling**: `SubscribeTicks` works by polling `SymbolInfoTick` on an interval. True push streaming from the pipe protocol has not been reverse-engineered yet.

## Testing

```bash
go test ./...
```

Tests use an in-memory mock pipe, no MT5 instance needed.

## Project Structure

```
go-mt5/
├── *.go                     # Public API (package gomt5)
├── analysis/                # Rate series, SMA/EMA, CSV export
├── tradeutil/               # High-level trade helpers (Buy, Sell, Close…)
├── examples/                # Runnable usage examples
├── internal/
│   ├── protocol/            # Binary codec + message framing
│   └── pipe/                # Windows named pipe transport
└── .github/                 # CI workflows
```

## Changelog

### v0.1.8

Full `AccountInfo` decoder. v0.1.7 only populated `Login` and the trailing string fields (`Name`, `Server`, `Currency`, `Company`); every numeric and boolean field in between (`Balance`, `Equity`, `Margin`, `FreeMargin`, `Profit`, `Credit`, etc.) read as `0`/`false`. This release decodes the full 139-byte middle of `CmdAccountInfo` against the real MT5 wire layout (4×int32, 2×bool1, 2×int32, 1×bool1, 14×float64 — no alignment padding), verified against a live build-5684 demo. A one-shot `[gomt5] AccountInfo debug: ...` hex log fires on the first call to help diagnose future build drift; it is intended to be removed in v0.1.9 once layout stability is confirmed.

Other fixes:
- `pipe.Discover()` uses native SHA-256 over `terminal64.exe` path (no external tools).
- Deals / orders / positions string fields decoded as fixed-width slots.
- ENUM field widths corrected across history / trading / positions decoders.

## Contributing

We welcome contributions! Please read our [Contributing Guide](CONTRIBUTING.md) and [Code of Conduct](CODE_OF_CONDUCT.md) before submitting a pull request.

## Security

To report a security vulnerability, please see our [Security Policy](SECURITY.md).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
