// Command mt5-mcp-cli is a small CLI for the MetaTrader 5 MCP HTTP server.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

const (
	defaultEndpoint = "http://127.0.0.1:22346/mcp"
	protocolVersion = "2025-06-18"
	clientName      = "mt5-mcp-cli"
	clientVersion   = "0.1.0"
)

type config struct {
	endpoint string
	apiKey   string
	timeout  time.Duration
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		printUsage()
		return nil
	}

	switch args[0] {
	case "init", "initialize":
		return runInit(args[1:])
	case "symbols":
		return runSymbols(args[1:])
	case "quote":
		return runQuote(args[1:])
	case "rates":
		return runRates(args[1:])
	case "ticks":
		return runTicks(args[1:])
	case "help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q (use -help for usage)", args[0])
	}
}

func runInit(args []string) error {
	cfg, err := parseConfig("init", args)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	client := newMCPClient(cfg)
	result, err := client.initialize(ctx)
	if err != nil {
		return err
	}
	return printJSON(result)
}

func runSymbols(args []string) error {
	fs, common := newCommandFlagSet("symbols")
	symbol := fs.String("symbol", "", "exact symbol to return; omit to list Market Watch symbols")
	includeHidden := fs.Bool("include-hidden", false, "include hidden symbols")
	limit := fs.Int("limit", 1000, "maximum number of symbols")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := common.config()
	if err != nil {
		return err
	}
	if *limit <= 0 {
		return errors.New("-limit must be greater than zero")
	}

	return withMCPClient(cfg, func(ctx context.Context, client *mcpClient) error {
		arguments := map[string]any{
			"include_hidden": *includeHidden,
			"limit":          *limit,
		}
		if strings.TrimSpace(*symbol) != "" {
			arguments["symbol"] = strings.TrimSpace(*symbol)
		}
		result, err := client.callTool(ctx, "get_marketwatch_symbols", arguments)
		if err != nil {
			return err
		}
		return printToolResult(result)
	})
}

func runQuote(args []string) error {
	fs, common := newCommandFlagSet("quote")
	symbol := fs.String("symbol", "", "exact symbol to quote, for example EURUSD")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := common.config()
	if err != nil {
		return err
	}
	if strings.TrimSpace(*symbol) == "" {
		return errors.New("-symbol is required")
	}

	return withMCPClient(cfg, func(ctx context.Context, client *mcpClient) error {
		result, err := client.callTool(ctx, "get_marketwatch_symbols", map[string]any{
			"symbol": strings.TrimSpace(*symbol),
			"limit":  1,
		})
		if err != nil {
			return err
		}
		return printToolResult(result)
	})
}

func runRates(args []string) error {
	fs, common := newCommandFlagSet("rates")
	symbol := fs.String("symbol", "", "exact symbol, for example EURUSD")
	period := fs.String("period", "H1", "chart period, for example M1, M5, H1, D1")
	from := fs.String("from", "", "inclusive ISO-8601 start time; defaults to 24 hours ago")
	to := fs.String("to", "", "exclusive ISO-8601 end time; defaults to now")
	limit := fs.Int("limit", 1000, "maximum number of candles")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := common.config()
	if err != nil {
		return err
	}
	if strings.TrimSpace(*symbol) == "" {
		return errors.New("-symbol is required")
	}
	if *limit <= 0 {
		return errors.New("-limit must be greater than zero")
	}
	if !validPeriod(*period) {
		return fmt.Errorf("unsupported -period %q", *period)
	}
	fromValue, toValue := defaultDateRange(*from, *to, 24*time.Hour)

	return withMCPClient(cfg, func(ctx context.Context, client *mcpClient) error {
		result, err := client.callTool(ctx, "get_chart_history", map[string]any{
			"datetime_from": fromValue,
			"datetime_to":   toValue,
			"symbol":        strings.TrimSpace(*symbol),
			"period":        strings.ToUpper(strings.TrimSpace(*period)),
			"limit":         *limit,
		})
		if err != nil {
			return err
		}
		return printToolResult(result)
	})
}

func runTicks(args []string) error {
	fs, common := newCommandFlagSet("ticks")
	symbol := fs.String("symbol", "", "exact symbol, for example EURUSD")
	from := fs.String("from", "", "inclusive ISO-8601 start time; defaults to one hour ago")
	to := fs.String("to", "", "exclusive ISO-8601 end time; defaults to now")
	limit := fs.Int("limit", 1000, "maximum number of ticks")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := common.config()
	if err != nil {
		return err
	}
	if strings.TrimSpace(*symbol) == "" {
		return errors.New("-symbol is required")
	}
	if *limit <= 0 {
		return errors.New("-limit must be greater than zero")
	}
	fromValue, toValue := defaultDateRange(*from, *to, time.Hour)

	return withMCPClient(cfg, func(ctx context.Context, client *mcpClient) error {
		result, err := client.callTool(ctx, "get_chart_ticks_history", map[string]any{
			"datetime_from": fromValue,
			"datetime_to":   toValue,
			"symbol":        strings.TrimSpace(*symbol),
			"limit":         *limit,
		})
		if err != nil {
			return err
		}
		return printToolResult(result)
	})
}

func withMCPClient(cfg config, fn func(context.Context, *mcpClient) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	client := newMCPClient(cfg)
	if _, err := client.initialize(ctx); err != nil {
		return err
	}
	return fn(ctx, client)
}

func parseConfig(command string, args []string) (config, error) {
	fs, common := newCommandFlagSet(command)
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	return common.config()
}

type commonFlags struct {
	endpoint *string
	apiKey   *string
	timeout  *time.Duration
}

func newCommandFlagSet(command string) (*flag.FlagSet, commonFlags) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	endpoint := os.Getenv("MT5_MCP_URL")
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	apiKey := os.Getenv("MT5_MCP_API_KEY")
	timeout := 30 * time.Second

	return fs, commonFlags{
		endpoint: fs.String("url", endpoint, "MCP HTTP endpoint"),
		apiKey:   fs.String("api-key", apiKey, "MCP API key; prefer MT5_MCP_API_KEY"),
		timeout:  fs.Duration("timeout", timeout, "request timeout"),
	}
}

func (f commonFlags) config() (config, error) {
	if *f.timeout <= 0 {
		return config{}, errors.New("-timeout must be greater than zero")
	}
	if err := validateEndpoint(*f.endpoint); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(*f.apiKey) == "" {
		return config{}, errors.New("MCP API key is required; set MT5_MCP_API_KEY or pass -api-key")
	}
	return config{endpoint: *f.endpoint, apiKey: *f.apiKey, timeout: *f.timeout}, nil
}

func validateEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid MCP URL %q", raw)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("MCP URL must use http or https, got %q", u.Scheme)
	}
	return nil
}

func defaultDateRange(from, to string, duration time.Duration) (string, string) {
	now := time.Now().UTC()
	if strings.TrimSpace(to) == "" {
		to = now.Format(time.RFC3339)
	}
	if strings.TrimSpace(from) == "" {
		from = now.Add(-duration).Format(time.RFC3339)
	}
	return from, to
}

func validPeriod(period string) bool {
	switch strings.ToUpper(strings.TrimSpace(period)) {
	case "M1", "M2", "M3", "M4", "M5", "M6", "M10", "M12", "M15", "M20", "M30", "H1", "H2", "H3", "H4", "H6", "H8", "H12", "D1", "W1", "MN1":
		return true
	default:
		return false
	}
}

func printUsage() {
	fmt.Println(`mt5-mcp-cli connects to a MetaTrader 5 MCP Streamable HTTP server.

Usage:
  mt5-mcp-cli init [common flags]
  mt5-mcp-cli symbols [common flags] [-symbol EURUSD] [-limit 1000]
  mt5-mcp-cli quote [common flags] -symbol EURUSD
  mt5-mcp-cli rates [common flags] -symbol EURUSD [-period H1] [-from time] [-to time]
  mt5-mcp-cli ticks [common flags] -symbol EURUSD [-from time] [-to time]

Commands:
  init       initialize an MCP session and print server information
  symbols    list Market Watch symbols
  quote      get the latest Market Watch quote for one symbol
  rates      get OHLCV chart history (defaults to the last 24 hours)
  ticks      get tick history (defaults to the last hour)

Common flags:
  -url       MCP endpoint (default: MT5_MCP_URL or http://127.0.0.1:22346/mcp)
  -api-key   MCP API key (default: MT5_MCP_API_KEY)
  -timeout   request timeout (default: 30s)

The CLI emits JSON on stdout. It never places or modifies trading orders.`)
}

func isHelp(value string) bool {
	return value == "-h" || value == "--help" || value == "-help"
}

type mcpClient struct {
	endpoint string
	apiKey   string
	http     *http.Client
	nextID   int64
	session  string
}

func newMCPClient(cfg config) *mcpClient {
	return &mcpClient{
		endpoint: cfg.endpoint,
		apiKey:   cfg.apiKey,
		http:     &http.Client{},
		nextID:   1,
	}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int64 `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (c *mcpClient) initialize(ctx context.Context) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.nextID, 1) - 1
	result, headers, err := c.post(ctx, rpcRequest{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo": map[string]string{
				"name":    clientName,
				"version": clientVersion,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("initialize MCP session: %w", err)
	}
	if session := headers.Get("Mcp-Session-Id"); session != "" {
		c.session = session
	}

	if _, _, err := c.post(ctx, rpcRequest{JSONRPC: "2.0", Method: "notifications/initialized"}); err != nil {
		return nil, fmt.Errorf("complete MCP initialization: %w", err)
	}
	return result, nil
}

func (c *mcpClient) callTool(ctx context.Context, name string, arguments map[string]any) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.nextID, 1) - 1
	result, _, err := c.post(ctx, rpcRequest{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      name,
			"arguments": arguments,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", name, err)
	}
	return result, nil
}

func (c *mcpClient) post(ctx context.Context, payload rpcRequest) (json.RawMessage, http.Header, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("encode MCP request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("create MCP request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", protocolVersion)
	if c.session != "" {
		req.Header.Set("Mcp-Session-Id", c.session)
	}

	response, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("send MCP request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return nil, response.Header, fmt.Errorf("read MCP response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = response.Status
		}
		return nil, response.Header, fmt.Errorf("MCP server returned %s: %s", response.Status, message)
	}

	// Notifications are acknowledged with 202 and no JSON body.
	if payload.ID == nil && len(bytes.TrimSpace(responseBody)) == 0 {
		return nil, response.Header, nil
	}
	jsonBody, err := extractJSON(responseBody, response.Header.Get("Content-Type"))
	if err != nil {
		return nil, response.Header, err
	}
	var envelope rpcResponse
	if err := json.Unmarshal(jsonBody, &envelope); err != nil {
		return nil, response.Header, fmt.Errorf("decode MCP response: %w", err)
	}
	if envelope.Error != nil {
		return nil, response.Header, fmt.Errorf("JSON-RPC %d: %s", envelope.Error.Code, envelope.Error.Message)
	}
	if len(envelope.Result) == 0 {
		return nil, response.Header, errors.New("MCP response did not contain a result")
	}
	return envelope.Result, response.Header, nil
}

func extractJSON(body []byte, contentType string) ([]byte, error) {
	trimmed := bytes.TrimSpace(body)
	if !strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return trimmed, nil
	}

	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		candidate := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if candidate == "" || candidate == "[DONE]" {
			continue
		}
		if json.Valid([]byte(candidate)) {
			return []byte(candidate), nil
		}
	}
	return nil, errors.New("MCP event-stream response did not contain JSON data")
}

type toolCallResult struct {
	IsError          bool            `json:"isError"`
	Content          []contentBlock  `json:"content"`
	StructuredResult json.RawMessage `json:"structuredContent"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func printToolResult(raw json.RawMessage) error {
	var result toolCallResult
	if err := json.Unmarshal(raw, &result); err == nil {
		if result.IsError {
			return errors.New(toolText(result))
		}
		for _, block := range result.Content {
			if block.Type != "text" || strings.TrimSpace(block.Text) == "" {
				continue
			}
			if json.Valid([]byte(block.Text)) {
				return printJSON(json.RawMessage(block.Text))
			}
			fmt.Println(block.Text)
			return nil
		}
		if len(result.StructuredResult) != 0 {
			return printJSON(result.StructuredResult)
		}
	}
	return printJSON(raw)
}

func toolText(result toolCallResult) string {
	for _, block := range result.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			return block.Text
		}
	}
	return "MCP tool returned an error"
}

func printJSON(raw []byte) error {
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, raw, "", "  "); err != nil {
		return fmt.Errorf("format JSON output: %w", err)
	}
	fmt.Println(formatted.String())
	return nil
}
