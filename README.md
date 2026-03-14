# go-mt5

[![Go Reference](https://pkg.go.dev/badge/github.com/mukbeast4/go-mt5.svg)](https://pkg.go.dev/github.com/mukbeast4/go-mt5)
[![Go Report Card](https://goreportcard.com/badge/github.com/mukbeast4/go-mt5)](https://goreportcard.com/report/github.com/mukbeast4/go-mt5)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/Mukbeast4/go-mt5)](https://github.com/Mukbeast4/go-mt5/releases)

Native Go client for MetaTrader 5. Communicates directly with the MT5 terminal via Windows Named Pipe IPC, the same mechanism used by the official Python `MetaTrader5` package. No Expert Advisor needed.

## Features

- Direct IPC connection to `terminal64.exe` via Windows Named Pipes
- Reverse-engineered binary protocol (same as `MetaTrader5.pyd`)
- Account and terminal information
- Real-time symbol info and tick data
- Historical rates and ticks (OHLCV, copy from position/date/range)
- Order management (send, check, calc margin/profit)
- Position and history queries (orders, deals)
- Streaming tick subscriptions
- Zero external dependencies (only `golang.org/x/sys`)

## Requirements

- Windows (named pipes are a Windows kernel feature)
- MetaTrader 5 terminal running with an active account
- Go 1.21+

## Installation

```bash
go get github.com/mukbeast4/go-mt5
```

## Quick Start

```go
package main

import (
	"fmt"
	"log"

	gomt5 "github.com/mukbeast4/go-mt5"
)

func main() {
	client, err := gomt5.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	fmt.Printf("Connected to MT5 build %d\n", client.Build())

	account, err := client.AccountInfo()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Account: %s Balance: %.2f %s\n",
		account.Name, account.Balance, account.Currency)

	tick, err := client.SymbolInfoTick("EURUSD")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("EURUSD Bid: %.5f Ask: %.5f\n", tick.Bid, tick.Ask)
}
```

## API

| Group | Methods |
|-------|---------|
| Connection | `NewClient()`, `Close()`, `Build()`, `LastError()` |
| Account | `AccountInfo()`, `TerminalInfo()`, `Version()` |
| Symbols | `SymbolsTotal()`, `SymbolInfo()`, `SymbolInfoTick()` |
| Market Data | `CopyRatesFrom()`, `CopyRatesFromPos()`, `CopyRatesRange()`, `CopyTicksFrom()`, `CopyTicksRange()` |
| Trading | `OrderSend()`, `OrderCheck()`, `OrderCalcMargin()`, `OrderCalcProfit()`, `OrdersTotal()`, `OrdersGet()` |
| Positions | `PositionsTotal()`, `PositionsGet()` |
| History | `HistoryOrdersTotal()`, `HistoryOrdersGet()`, `HistoryDealsTotal()`, `HistoryDealsGet()` |
| Streaming | `SubscribeTicks()`, `UnsubscribeTicks()` |
| Debug | `SendRaw()` |

## Protocol

```
Request:  [payload_len:LE32][cmd_id:LE32][params...]
Response: [payload_len:LE32][cmd_echo:LE32][success:LE32][data...]
```

- Integers: LE32/LE64
- Floats: IEEE 754 double LE64
- Strings: `[char_count:LE32][UTF-16LE data]`
- Booleans: LE64 (0/1)

## Testing

```bash
go test ./...
```

Tests use an in-memory mock pipe, no MT5 instance needed.

## Project Structure

```
go-mt5/
├── *.go                     # Public API (package gomt5)
├── internal/
│   ├── protocol/            # Binary codec + message framing
│   └── pipe/                # Windows named pipe transport
└── mql5/                    # EA bridge (alternative approach)
```

## Contributing

We welcome contributions! Please read our [Contributing Guide](CONTRIBUTING.md) and [Code of Conduct](CODE_OF_CONDUCT.md) before submitting a pull request.

## Security

To report a security vulnerability, please see our [Security Policy](SECURITY.md).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
