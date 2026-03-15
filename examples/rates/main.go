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

	rates, err := client.CopyRatesFromPos(ctx, "EURUSD", gomt5.TimeframeM1, 0, 1000)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Got %d M1 bars for EURUSD\n", len(rates))
	if len(rates) > 0 {
		first := rates[0]
		last := rates[len(rates)-1]
		fmt.Printf("First: %s O=%.5f H=%.5f L=%.5f C=%.5f\n",
			first.TimeUTC().Format("2006-01-02 15:04"),
			first.Open, first.High, first.Low, first.Close)
		fmt.Printf("Last:  %s O=%.5f H=%.5f L=%.5f C=%.5f\n",
			last.TimeUTC().Format("2006-01-02 15:04"),
			last.Open, last.High, last.Low, last.Close)
	}
}
