package main

import (
	"context"
	"fmt"
	"log"
	"math"

	gomt5 "github.com/mukbeast4/go-mt5"
)

func main() {
	ctx := context.Background()

	client, err := gomt5.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	symbol := "EURUSD"
	riskPercent := 1.0
	slPips := 50.0

	account, err := client.AccountInfo(ctx)
	if err != nil {
		log.Fatal(err)
	}

	info, err := client.SymbolInfo(ctx, symbol)
	if err != nil {
		log.Fatal(err)
	}

	tick, err := client.SymbolInfoTick(ctx, symbol)
	if err != nil {
		log.Fatal(err)
	}

	riskAmount := account.Balance * (riskPercent / 100.0)
	slDistance := slPips * info.Point * math.Pow(10, float64(info.Digits-1))
	pipValue := info.TradeTickValue * (info.Point / info.TradeTickSize)
	lot := riskAmount / (slDistance / info.Point * pipValue)

	lot = math.Floor(lot/info.VolumeStep) * info.VolumeStep
	lot = math.Max(lot, info.VolumeMin)
	lot = math.Min(lot, info.VolumeMax)

	fmt.Printf("Symbol: %s\n", symbol)
	fmt.Printf("Balance: %.2f %s\n", account.Balance, account.Currency)
	fmt.Printf("Risk: %.1f%% = %.2f %s\n", riskPercent, riskAmount, account.Currency)
	fmt.Printf("SL: %.0f pips (%.5f)\n", slPips, slDistance)
	fmt.Printf("Bid: %.5f Ask: %.5f\n", tick.Bid, tick.Ask)
	fmt.Printf("Lot size: %.2f (min=%.2f max=%.2f step=%.2f)\n",
		lot, info.VolumeMin, info.VolumeMax, info.VolumeStep)
}
