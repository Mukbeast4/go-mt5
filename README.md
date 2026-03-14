# go-mt5

Native Go binding for MetaTrader 5. Communicates directly with the MT5 terminal via Windows Named Pipe IPC, the same mechanism used by the official Python `MetaTrader5` package. No Expert Advisor needed.

## Architecture

```
Go Client --Named Pipe IPC--> terminal64.exe (MT5)
```

The library reverse-engineered the proprietary binary protocol used by MetaQuotes between `MetaTrader5.pyd` and `terminal64.exe`. It connects to the same named pipe, performs the same handshake, and speaks the same binary framing.

### Protocol

```
Request:  [payload_len:LE32][cmd_id:LE32][params...]
Response: [payload_len:LE32][cmd_echo:LE32][success:LE32][data...]
```

- Integers: LE32/LE64
- Floats: IEEE 754 double LE64
- Strings: [char_count:LE32][UTF-16LE data]
- Booleans: LE64 (0/1)

## Requirements

- Windows (named pipes are a Windows kernel feature)
- MetaTrader 5 terminal running with an active account
- Go 1.21+

## Installation

```bash
go get github.com/mukbeast4/go-mt5
```

## Usage

```go
package main

import (
    "fmt"
    "log"

    gomt5 "github.com/mukbeast4/go-mt5"
)

func main() {
    // Auto-discovers the MT5 named pipe
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

    rates, err := client.CopyRatesFromPos("EURUSD", gomt5.TimeframeH1, 0, 100)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Got %d H1 rates\n", len(rates))

    result, err := client.OrderSend(gomt5.TradeRequest{
        Action:    gomt5.TradeActionDeal,
        Symbol:    "EURUSD",
        Volume:    0.01,
        Type:      gomt5.OrderTypeBuy,
        Price:     tick.Ask,
        Deviation: 10,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Order: %d Deal: %d\n", result.Order, result.Deal)
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
| Debug | `SendRaw()` |

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
│   └── pipe/                # Windows named pipe connection
├── tools/
│   ├── sniffer/             # Pipe traffic capture tools
│   └── analyzer/            # PYD binary analysis tools
├── mql5/                    # EA bridge (alternative approach)
└── docs/                    # Reverse engineering documentation
```
