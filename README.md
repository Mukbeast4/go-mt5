# go-mt5

Go library for MetaTrader 5, equivalent to the official Python `MetaTrader5` package.

## Architecture

A MQL5 Expert Advisor runs inside MT5 as a TCP socket server on `127.0.0.1:15555`. The Go library connects as a TCP client. Protocol is JSON with length-prefix framing (4 bytes big-endian).

```
[Go Client] --TCP--> [MT5 EA Server (127.0.0.1:15555)]
```

## Installation

```bash
go get github.com/mukbeast4/go-mt5
```

## MT5 Setup

1. Copy `mql5/GoMT5Bridge.mq5` to your MT5 `Experts` folder
2. Compile the EA in MetaEditor
3. Attach the EA to any chart (it uses `OnTimer`, not `OnTick` for request handling)
4. Allow socket connections in MT5: Tools > Options > Expert Advisors > Allow WebRequest

## Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/mukbeast4/go-mt5/pkg/mt5"
)

func main() {
    client, err := mt5.NewClient(
        mt5.WithAddress("127.0.0.1:15555"),
        mt5.WithTimeout(30 * time.Second),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    account, err := client.AccountInfo(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Account: %s Balance: %.2f %s\n", account.Name, account.Balance, account.Currency)

    rates, err := client.CopyRatesFrom(ctx, "EURUSD", mt5.TimeframeH1, time.Now().Add(-24*time.Hour).Unix(), 100)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Got %d rates\n", len(rates))

    result, err := client.OrderSend(ctx, mt5.TradeRequest{
        Action:   mt5.TradeActionDeal,
        Symbol:   "EURUSD",
        Volume:   0.01,
        Type:     mt5.OrderTypeBuy,
        Price:    1.0850,
        Deviation: 10,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Order: %d Deal: %d\n", result.Order, result.Deal)
}
```

## Streaming Ticks

```go
ch, err := client.SubscribeTicks(ctx, "EURUSD")
if err != nil {
    log.Fatal(err)
}

for tick := range ch {
    fmt.Printf("Bid: %.5f Ask: %.5f\n", tick.Bid, tick.Ask)
}
```

## CLI

```bash
go run ./cmd/mt5cli version
go run ./cmd/mt5cli account
go run ./cmd/mt5cli symbol EURUSD
go run ./cmd/mt5cli rates EURUSD H1 100
go run ./cmd/mt5cli subscribe EURUSD
```

## API

| Group | Methods |
|-------|---------|
| Connection | `Version()`, `LastError()` |
| Account | `AccountInfo()`, `TerminalInfo()` |
| Symbols | `SymbolsTotal()`, `SymbolsGet()`, `SymbolInfo()`, `SymbolInfoTick()`, `SymbolSelect()` |
| Market Data | `CopyRatesFrom()`, `CopyRatesFromPos()`, `CopyRatesRange()`, `CopyTicksFrom()`, `CopyTicksRange()`, `MarketBookAdd()`, `MarketBookGet()`, `MarketBookRelease()` |
| Trading | `OrderSend()`, `OrderCheck()`, `OrderCalcMargin()`, `OrderCalcProfit()`, `OrdersTotal()`, `OrdersGet()` |
| Positions | `PositionsTotal()`, `PositionsGet()` |
| History | `HistoryOrdersTotal()`, `HistoryOrdersGet()`, `HistoryDealsTotal()`, `HistoryDealsGet()` |
| Streaming | `SubscribeTicks()`, `UnsubscribeTicks()` |

## Protocol

```
[4 bytes uint32 BE = payload size][JSON payload]
```

Request: `{"id":"uuid","action":"copy_rates_from","params":{...}}`
Response: `{"id":"uuid","success":true,"data":[...]}`
Event: `{"type":"event","event":"tick","data":{"symbol":"EURUSD","bid":1.0850,...}}`

## Testing

```bash
go test ./...
```

Integration tests require a running MT5 instance with the EA attached:

```bash
go test -tags integration ./...
```
