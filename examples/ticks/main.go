package main

import (
	"context"
	"fmt"
	"log"
	"time"

	gomt5 "github.com/mukbeast4/go-mt5"
)

func main() {
	ctx := context.Background()

	client, err := gomt5.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	from := gomt5.FromTime(time.Now().Add(-1 * time.Hour))
	ticks, err := client.CopyTicksFrom(ctx, "EURUSD", from, 100, gomt5.CopyTicksAll)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Got %d ticks for EURUSD (last hour)\n", len(ticks))
	for i, tick := range ticks {
		if i >= 10 {
			fmt.Printf("... and %d more\n", len(ticks)-10)
			break
		}
		fmt.Printf("  %s Bid=%.5f Ask=%.5f\n",
			tick.TimeUTC().Format("15:04:05.000"),
			tick.Bid, tick.Ask)
	}
}
