# mt5-mcp-cli

Standalone command-line client for the MetaTrader 5 MCP Streamable HTTP server.

The CLI reads the API key from `MT5_MCP_API_KEY` so credentials are not stored in
source control. Set that variable to the key provided for your MT5 MCP server:

```powershell
$env:MT5_MCP_API_KEY = '<your MT5 MCP API key>'
```

The endpoint defaults to `http://127.0.0.1:22346/mcp`. Override it with
`MT5_MCP_URL` or `-url`.

## Build and run

From the repository root:

```powershell
go run ./cmd/mt5-mcp-cli init
go run ./cmd/mt5-mcp-cli quote -symbol EURUSD
go run ./cmd/mt5-mcp-cli symbols -limit 20
go run ./cmd/mt5-mcp-cli rates -symbol EURUSD -period H1 -limit 50
go run ./cmd/mt5-mcp-cli ticks -symbol EURUSD -limit 100
```

To create a reusable executable:

```powershell
go build -o mt5-mcp-cli.exe ./cmd/mt5-mcp-cli
.\mt5-mcp-cli.exe init
```

Every command initializes a fresh MCP session. `init` only performs the MCP
handshake and prints the server response. Market-data commands initialize the
session and then call the corresponding read-only MCP tool. Output is JSON on
stdout. No trading tools are called or exposed by this CLI.

## Commands

- `init` / `initialize`: perform the MCP initialize handshake.
- `symbols`: list symbols currently available in Market Watch.
- `quote -symbol SYMBOL`: return the latest Market Watch quote for one symbol.
- `rates -symbol SYMBOL`: fetch chart candles. Defaults to the last 24 hours and
  period `H1`.
- `ticks -symbol SYMBOL`: fetch tick history. Defaults to the last hour.

All commands accept `-url`, `-api-key`, and `-timeout`. The `rates` and `ticks`
commands accept ISO-8601 `-from` and `-to` values. The server expects the period
names supported by MT5, such as `M1`, `M5`, `H1`, `D1`, `W1`, and `MN1`.
