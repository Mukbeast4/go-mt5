package main

import (
	"context"
	"fmt"
	"log"

	gomt5 "github.com/mukbeast4/go-mt5"
)

func main() {
	ctx := context.Background()

	client, err := gomt5.NewClient(ctx, gomt5.WithDebug(true))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	fmt.Printf("Connected to MT5 build %d\n", client.Build())

	ver, err := client.Version(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Version: %d Build: %d Date: %s\n", ver.Version, ver.Build, ver.BuildDate)

	account, err := client.AccountInfo(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Account: %s (login %d)\n", account.Name, account.Login)
	fmt.Printf("Balance: %.2f %s\n", account.Balance, account.Currency)
	fmt.Printf("Equity: %.2f Margin: %.2f Free: %.2f\n",
		account.Equity, account.Margin, account.FreeMargin)

	terminal, err := client.TerminalInfo(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Terminal: %s (%s)\n", terminal.Name, terminal.Company)
	fmt.Printf("Trade allowed: %v\n", terminal.TradeAllowed)

	total, err := client.SymbolsTotal(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Symbols available: %d\n", total)
}
