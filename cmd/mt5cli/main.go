package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mukbeast4/go-mt5/pkg/mt5"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:15555", "MT5 EA address")
	timeout := flag.Duration("timeout", 30*time.Second, "request timeout")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	client, err := mt5.NewClient(
		mt5.WithAddress(*addr),
		mt5.WithTimeout(*timeout),
	)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	action := args[0]
	switch action {
	case "version":
		result, err := client.Version(ctx)
		exitOnErr(err)
		printJSON(result)

	case "account":
		result, err := client.AccountInfo(ctx)
		exitOnErr(err)
		printJSON(result)

	case "terminal":
		result, err := client.TerminalInfo(ctx)
		exitOnErr(err)
		printJSON(result)

	case "symbols-total":
		result, err := client.SymbolsTotal(ctx)
		exitOnErr(err)
		fmt.Println(result)

	case "symbols":
		group := ""
		if len(args) > 1 {
			group = args[1]
		}
		result, err := client.SymbolsGet(ctx, group)
		exitOnErr(err)
		printJSON(result)

	case "symbol":
		requireArgs(args, 2)
		result, err := client.SymbolInfo(ctx, args[1])
		exitOnErr(err)
		printJSON(result)

	case "tick":
		requireArgs(args, 2)
		result, err := client.SymbolInfoTick(ctx, args[1])
		exitOnErr(err)
		printJSON(result)

	case "rates":
		requireArgs(args, 3)
		symbol := args[1]
		tf := parseTimeframe(args[2])
		count := 100
		if len(args) > 3 {
			fmt.Sscanf(args[3], "%d", &count)
		}
		from := time.Now().Add(-time.Duration(count) * time.Hour).Unix()
		result, err := client.CopyRatesFrom(ctx, symbol, tf, from, count)
		exitOnErr(err)
		printJSON(result)

	case "ticks":
		requireArgs(args, 2)
		symbol := args[1]
		count := 100
		if len(args) > 2 {
			fmt.Sscanf(args[2], "%d", &count)
		}
		from := time.Now().Add(-1 * time.Hour).Unix()
		result, err := client.CopyTicksFrom(ctx, symbol, from, count, mt5.CopyTicksAll)
		exitOnErr(err)
		printJSON(result)

	case "orders":
		result, err := client.OrdersTotal(ctx)
		exitOnErr(err)
		fmt.Printf("Total orders: %d\n", result)
		orders, err := client.OrdersGet(ctx, "", "", 0)
		exitOnErr(err)
		printJSON(orders)

	case "positions":
		result, err := client.PositionsTotal(ctx)
		exitOnErr(err)
		fmt.Printf("Total positions: %d\n", result)
		positions, err := client.PositionsGet(ctx, "", "", 0)
		exitOnErr(err)
		printJSON(positions)

	case "subscribe":
		requireArgs(args, 2)
		symbol := args[1]
		ch, err := client.SubscribeTicks(ctx, symbol)
		exitOnErr(err)
		fmt.Printf("Subscribed to %s ticks (Ctrl+C to stop)\n", symbol)

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		for {
			select {
			case tick, ok := <-ch:
				if !ok {
					return
				}
				fmt.Printf("Bid: %.5f  Ask: %.5f  Last: %.5f  Time: %d\n",
					tick.Bid, tick.Ask, tick.Last, tick.TimeMsc)
			case <-sigCh:
				client.UnsubscribeTicks(context.Background(), symbol)
				return
			}
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown action: %s\n", action)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: mt5cli [flags] <action> [args...]")
	fmt.Println()
	fmt.Println("Actions:")
	fmt.Println("  version                  Show MT5/EA version")
	fmt.Println("  account                  Account info")
	fmt.Println("  terminal                 Terminal info")
	fmt.Println("  symbols-total            Total symbols count")
	fmt.Println("  symbols [group]          List symbols")
	fmt.Println("  symbol <name>            Symbol info")
	fmt.Println("  tick <symbol>            Current tick")
	fmt.Println("  rates <symbol> <tf> [n]  Copy rates (tf: M1,M5,H1,D1...)")
	fmt.Println("  ticks <symbol> [n]       Copy ticks")
	fmt.Println("  orders                   Active orders")
	fmt.Println("  positions                Open positions")
	fmt.Println("  subscribe <symbol>       Stream ticks in real-time")
}

func exitOnErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func requireArgs(args []string, n int) {
	if len(args) < n {
		fmt.Fprintf(os.Stderr, "expected at least %d arguments\n", n)
		os.Exit(1)
	}
}

func printJSON(v interface{}) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}

func parseTimeframe(s string) mt5.Timeframe {
	tfMap := map[string]mt5.Timeframe{
		"M1": mt5.TimeframeM1, "M2": mt5.TimeframeM2, "M3": mt5.TimeframeM3,
		"M4": mt5.TimeframeM4, "M5": mt5.TimeframeM5, "M6": mt5.TimeframeM6,
		"M10": mt5.TimeframeM10, "M12": mt5.TimeframeM12, "M15": mt5.TimeframeM15,
		"M20": mt5.TimeframeM20, "M30": mt5.TimeframeM30,
		"H1": mt5.TimeframeH1, "H2": mt5.TimeframeH2, "H3": mt5.TimeframeH3,
		"H4": mt5.TimeframeH4, "H6": mt5.TimeframeH6, "H8": mt5.TimeframeH8,
		"H12": mt5.TimeframeH12, "D1": mt5.TimeframeD1,
		"W1": mt5.TimeframeW1, "MN1": mt5.TimeframeMN1,
	}
	if tf, ok := tfMap[strings.ToUpper(s)]; ok {
		return tf
	}
	log.Fatalf("unknown timeframe: %s", s)
	return 0
}
