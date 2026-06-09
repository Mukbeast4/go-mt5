package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	gomt5 "github.com/mukbeast4/go-mt5"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	client, err := gomt5.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	symbols := []string{"EURUSD", "GBPUSD", "USDJPY", "XAUUSD"}
	for _, sym := range symbols {
		if err := client.SymbolSelect(ctx, sym, true); err != nil {
			log.Fatalf("select %s: %v", sym, err)
		}
	}

	quotes, err := client.PollQuotes(ctx, symbols, time.Second)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Polling %d symbols, one RPC per second\n", len(symbols))

	for changed := range quotes {
		for sym, tick := range changed {
			fmt.Printf("%s %s Bid=%.5f Ask=%.5f\n",
				sym, tick.TimeUTC().Format("15:04:05"), tick.Bid, tick.Ask)
		}
	}
	fmt.Println("Stream closed")
}
