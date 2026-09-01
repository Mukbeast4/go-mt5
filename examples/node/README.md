# Node.js mt5-bridge CLI

This is a complete Node.js client for the Rust `mt5-bridge` TCP protocol. It
uses only Node's built-in `net` module: no generated protobuf bindings or npm
dependencies are required.

Requirements:

- Node.js 18 or newer
- `mt5-bridge` running and listening on `127.0.0.1:19550` (or another address)
- `MT5_BRIDGE_TOKEN` set to the bridge token

From the repository root on Windows:

```powershell
$env:MT5_BRIDGE_TOKEN = "the-same-token-used-by-mt5-bridge"
node .\examples\node\cli.mjs SymbolInfoTick --symbol EURUSD --pretty
```

The CLI supports all requested market-data operations:

```powershell
node .\examples\node\cli.mjs CopyRatesFromPos --symbol EURUSD --timeframe M1 --start-pos 0 --count 100
node .\examples\node\cli.mjs CopyRatesFrom --symbol EURUSD --timeframe H1 --from 1710000000 --count 100
node .\examples\node\cli.mjs CopyRatesRange --symbol EURUSD --timeframe H1 --from 1710000000 --to 1713600000
node .\examples\node\cli.mjs CopyTicksFrom --symbol EURUSD --from 1710000000 --count 100 --flags all
node .\examples\node\cli.mjs CopyTicksRange --symbol EURUSD --from 1710000000 --to 1710003600 --flags info
```

Operation names are case-insensitive and may use kebab-case or snake_case,
so `copy-rates-from-pos` and `CopyRatesFromPos` are equivalent. `--from` and
`--to` accept broker epoch seconds or ISO-8601 dates. Numeric timestamps are
passed to MT5 unchanged; use the broker's timestamp convention described in
the root README.

Global options can be supplied with any operation:

```text
--address HOST:PORT       defaults to MT5_BRIDGE_ADDR or 127.0.0.1:19550
--token TOKEN             defaults to MT5_BRIDGE_TOKEN
--client-id ID            defaults to node-mt5-example
--timeout-ms N            defaults to 30000
--pretty                  indent JSON output
--verbose                 log handshake/error details to stderr
```

The result is written as JSON to stdout. Rates and ticks are decoded from the
bridge's `RateV1` and `TickV1` 60-byte records. Large responses replenish the
bridge's response credit as chunks are consumed, so the example is not
limited to the initial one-megabyte credit window.
