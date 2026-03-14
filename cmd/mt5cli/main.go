package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/mukbeast4/go-mt5/pkg/mt5"
)

func main() {
	pipeName := flag.String("pipe", "", "MT5 named pipe (auto-discover if empty)")
	timeout := flag.Duration("timeout", 60*time.Second, "connection timeout")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	opts := []mt5.Option{mt5.WithTimeout(*timeout)}
	if *pipeName != "" {
		opts = append(opts, mt5.WithPipeName(*pipeName))
	}

	client, err := mt5.NewClient(opts...)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer client.Close()

	fmt.Printf("Connected (build %d)\n\n", client.Build())

	action := args[0]
	switch action {
	case "version":
		result, err := client.Version()
		exitOnErr(err)
		printJSON(result)

	case "account":
		result, err := client.AccountInfo()
		exitOnErr(err)
		printJSON(result)

	case "terminal":
		result, err := client.TerminalInfo()
		exitOnErr(err)
		printJSON(result)

	case "symbols-total":
		result, err := client.SymbolsTotal()
		exitOnErr(err)
		fmt.Println(result)

	case "symbol":
		requireArgs(args, 2)
		result, err := client.SymbolInfo(args[1])
		exitOnErr(err)
		printJSON(result)

	case "tick":
		requireArgs(args, 2)
		result, err := client.SymbolInfoTick(args[1])
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
		result, err := client.CopyRatesFromPos(symbol, tf, 0, count)
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
		result, err := client.CopyTicksFrom(symbol, from, count, mt5.CopyTicksAll)
		exitOnErr(err)
		printJSON(result)

	case "orders":
		total, err := client.OrdersTotal()
		exitOnErr(err)
		fmt.Printf("Total: %d\n", total)

	case "positions":
		total, err := client.PositionsTotal()
		exitOnErr(err)
		fmt.Printf("Total: %d\n", total)

	case "history-orders":
		from := time.Now().AddDate(0, -1, 0).Unix()
		to := time.Now().Unix()
		total, err := client.HistoryOrdersTotal(from, to)
		exitOnErr(err)
		fmt.Printf("Total: %d\n", total)

	case "history-deals":
		from := time.Now().AddDate(0, -1, 0).Unix()
		to := time.Now().Unix()
		total, err := client.HistoryDealsTotal(from, to)
		exitOnErr(err)
		fmt.Printf("Total: %d\n", total)

	case "raw":
		requireArgs(args, 2)
		var cmdID uint32
		fmt.Sscanf(args[1], "%d", &cmdID)
		data, err := client.SendRaw(cmdID, nil)
		exitOnErr(err)
		fmt.Printf("Response: %d bytes\n", len(data))
		hexDump(data)

	default:
		fmt.Fprintf(os.Stderr, "unknown action: %s\n", action)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: mt5cli [flags] <action> [args...]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -pipe string    MT5 named pipe path (auto-discover if empty)")
	fmt.Println("  -timeout dur    Connection timeout (default 60s)")
	fmt.Println()
	fmt.Println("Actions:")
	fmt.Println("  version                  MT5 version info")
	fmt.Println("  account                  Account info")
	fmt.Println("  terminal                 Terminal info")
	fmt.Println("  symbols-total            Total symbols count")
	fmt.Println("  symbol <name>            Symbol info")
	fmt.Println("  tick <symbol>            Current tick")
	fmt.Println("  rates <symbol> <tf> [n]  Copy rates (tf: M1,M5,H1,D1...)")
	fmt.Println("  ticks <symbol> [n]       Copy ticks")
	fmt.Println("  orders                   Active orders count")
	fmt.Println("  positions                Open positions count")
	fmt.Println("  history-orders           History orders count (last month)")
	fmt.Println("  history-deals            History deals count (last month)")
	fmt.Println("  raw <cmd_id>             Send raw command (debug)")
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

func hexDump(data []byte) {
	for i := 0; i < len(data); i += 16 {
		end := i + 16
		if end > len(data) {
			end = len(data)
		}
		chunk := data[i:end]
		hex := ""
		ascii := ""
		for _, b := range chunk {
			hex += fmt.Sprintf("%02x ", b)
			if b >= 32 && b < 127 {
				ascii += string(rune(b))
			} else {
				ascii += "."
			}
		}
		fmt.Printf("  %04x: %-48s  %s\n", i, hex, ascii)
	}
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
