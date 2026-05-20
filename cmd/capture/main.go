package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	gomt5 "github.com/mukbeast4/go-mt5"
	"github.com/mukbeast4/go-mt5/internal/protocol"
)

type capture struct {
	name      string
	cmdID     uint32
	params    func() []byte
	outFile   string
	optional  bool
	skipEmpty bool
}

func main() {
	outDir := flag.String("out", ".", "directory to write .bin fixtures")
	symbol := flag.String("symbol", "EURUSD", "primary symbol used for rates/ticks/book captures")
	timeoutSec := flag.Int("timeout", 60, "client timeout (seconds)")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", *outDir, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSec)*time.Second)
	defer cancel()

	opts := []gomt5.Option{
		gomt5.WithTimeout(time.Duration(*timeoutSec) * time.Second),
	}
	if pipeName := os.Getenv("MT5_PIPE_NAME"); pipeName != "" {
		log.Printf("[CAPTURE] using pipe %s", pipeName)
		opts = append(opts, gomt5.WithPipeName(pipeName))
	}

	client, err := gomt5.NewClient(ctx, opts...)
	if err != nil {
		log.Fatalf("connect MT5: %v", err)
	}
	defer client.Close()
	log.Printf("[CAPTURE] connected, terminal build=%d", client.Build())

	nowSec := time.Now().Unix()
	dateFromTicks := nowSec - 60*60
	dateFromDeals := nowSec - 30*24*60*60

	wString := func(s string) []byte {
		w := protocol.NewWriter()
		w.WriteString(s)
		return w.Bytes()
	}

	captures := []capture{
		{
			name:    "account_info",
			cmdID:   protocol.CmdAccountInfo,
			params:  func() []byte { return nil },
			outFile: "account_info.bin",
		},
		{
			name:      "positions_all",
			cmdID:     protocol.CmdPositionsGet,
			params:    func() []byte { return nil },
			outFile:   "positions_current.bin",
			skipEmpty: true,
		},
		{
			name:      "orders_all",
			cmdID:     protocol.CmdOrdersGet,
			params:    func() []byte { return nil },
			outFile:   "orders_pending.bin",
			skipEmpty: true,
		},
		{
			name:  "rates_h1_50",
			cmdID: protocol.CmdCopyRatesFromPos,
			params: func() []byte {
				w := protocol.NewWriter()
				w.WriteString(*symbol)
				w.WriteU32(uint32(gomt5.TimeframeH1))
				w.WriteU32(0)
				w.WriteU32(50)
				return w.Bytes()
			},
			outFile: fmt.Sprintf("rates_h1_50_%s.bin", strings.ToLower(*symbol)),
		},
		{
			name:  "ticks_100",
			cmdID: protocol.CmdCopyTicksFrom,
			params: func() []byte {
				flags := int32(gomt5.CopyTicksAll)
				w := protocol.NewWriter()
				w.WriteString(*symbol)
				w.WriteI64(dateFromTicks * 1000)
				w.WriteU32(100)
				w.WriteU32(uint32(flags))
				return w.Bytes()
			},
			outFile: fmt.Sprintf("ticks_100_%s.bin", strings.ToLower(*symbol)),
		},
		{
			name:    "symbols_get_all",
			cmdID:   protocol.CmdSymbolsGet,
			params:  func() []byte { return nil },
			outFile: "symbols_all.bin",
		},
		{
			name:    "symbols_get_forex",
			cmdID:   protocol.CmdSymbolsGetByGroup,
			params:  func() []byte { return wString("*Forex*") },
			outFile: "symbols_forex.bin",
		},
		{
			name:  "market_book",
			cmdID: protocol.CmdMarketBookGet,
			params: func() []byte {
				return wString(*symbol)
			},
			outFile:  fmt.Sprintf("market_book_%s.bin", strings.ToLower(*symbol)),
			optional: true,
		},
		{
			name:  "history_deals_30d",
			cmdID: protocol.CmdHistoryDealsGet,
			params: func() []byte {
				w := protocol.NewWriter()
				w.WriteI64(dateFromDeals)
				w.WriteI64(nowSec)
				return w.Bytes()
			},
			outFile:   "history_deals_30d.bin",
			skipEmpty: true,
		},
	}

	for _, cap := range captures {
		log.Printf("[CAPTURE] cmd=%d %s ...", cap.cmdID, cap.name)
		data, err := client.SendRaw(ctx, cap.cmdID, cap.params())
		if err != nil {
			if cap.optional {
				log.Printf("[CAPTURE] cmd=%d %s SKIPPED (optional): %v", cap.cmdID, cap.name, err)
				continue
			}
			log.Printf("[CAPTURE] cmd=%d %s FAILED: %v", cap.cmdID, cap.name, err)
			continue
		}

		if cap.skipEmpty && len(data) >= 4 {
			if data[0] == 0 && data[1] == 0 && data[2] == 0 && data[3] == 0 {
				log.Printf("[CAPTURE] cmd=%d %s SKIPPED (count=0)", cap.cmdID, cap.name)
				continue
			}
		}

		path := filepath.Join(*outDir, cap.outFile)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			log.Fatalf("write %s: %v", path, err)
		}
		log.Printf("[CAPTURE] cmd=%d %s -> %s (%d bytes)", cap.cmdID, cap.name, path, len(data))
	}

	log.Printf("[CAPTURE] done")
}
