package main

import (
	"context"
	"fmt"
	"log"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/tradeutil"
)

func main() {
	ctx := context.Background()

	client, err := gomt5.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	result, err := tradeutil.Buy(ctx, client, "EURUSD", 0.01,
		tradeutil.WithComment("go-mt5 example"),
		tradeutil.WithDeviation(20),
	)
	if err != nil {
		log.Fatal(err)
	}

	if result.IsOK() {
		fmt.Printf("Order placed: deal=%d order=%d price=%.5f\n",
			result.Deal, result.Order, result.Price)
	} else {
		fmt.Printf("Order failed: retcode=%d comment=%s\n",
			result.Retcode, result.Comment)
	}
}
