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

	client, err := gomt5.NewClient(ctx,
		gomt5.WithTickPollInterval(50*time.Millisecond),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	symbols := []string{"EURUSD", "GBPUSD", "USDJPY"}
	channels := make(map[string]<-chan gomt5.Tick)

	for _, sym := range symbols {
		ch, err := client.SubscribeTicks(ctx, sym)
		if err != nil {
			log.Fatalf("subscribe %s: %v", sym, err)
		}
		channels[sym] = ch
		fmt.Printf("Subscribed to %s\n", sym)
	}

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Shutting down...")
			return
		case tick, ok := <-channels["EURUSD"]:
			if !ok {
				return
			}
			fmt.Printf("EURUSD %s Bid=%.5f Ask=%.5f\n",
				tick.TimeUTC().Format("15:04:05.000"), tick.Bid, tick.Ask)
		case tick, ok := <-channels["GBPUSD"]:
			if !ok {
				return
			}
			fmt.Printf("GBPUSD %s Bid=%.5f Ask=%.5f\n",
				tick.TimeUTC().Format("15:04:05.000"), tick.Bid, tick.Ask)
		case tick, ok := <-channels["USDJPY"]:
			if !ok {
				return
			}
			fmt.Printf("USDJPY %s Bid=%.3f Ask=%.3f\n",
				tick.TimeUTC().Format("15:04:05.000"), tick.Bid, tick.Ask)
		}
	}
}
