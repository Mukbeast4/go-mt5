# MT5 Rust bridge

`mt5-bridge` is a Windows x64 executable intended to run in the same Wine
prefix as MT5. It exposes an authenticated binary TCP server on
`127.0.0.1:19550` by default. The Go module at the repository root remains
the behavioral reference.

## Run

Set these variables in the environment that launches the Windows executable:

```text
MT5_BRIDGE_TOKEN=<per-backend secret>
MT5_PIPE_NAME=\\.\pipe\MT5.Terminal....
# or MT5_TERMINAL_PATH=C:\...\terminal64.exe
MT5_BRIDGE_LISTEN=127.0.0.1:19550             # optional
MT5_ACCOUNT_LOGIN=123456                      # optional, paired with server
MT5_ACCOUNT_SERVER=Broker-Server              # optional, paired with login
MT5_PIPE_IO_TIMEOUT_SECONDS=60                # optional; 0 disables it
MT5_HANDSHAKE_TIMEOUT_SECONDS=5               # optional, positive
MT5_TCP_WRITE_STALL_TIMEOUT_SECONDS=15        # optional, positive
MT5_REQUEST_QUEUE_CAPACITY=64                  # optional, positive
MT5_MAX_CONNECTIONS=32                         # optional, positive
MT5_PIPE_OPEN_TIMEOUT_SECONDS=60              # optional, pipe/terminal startup wait
```

If the account variables are omitted, the bridge adopts the first valid account
returned by MT5 before a market-data or trading request and uses that identity
for later checks. Deferring adoption avoids pinning the transient empty account
that a newly launched terminal can report while restoring its login. If one
account variable is provided without the other, startup fails. The executable
derives a pipe name from `MT5_TERMINAL_PATH` using the same
UTF-16LE/SHA-256 rule as the Go client. When that derived pipe is missing, the
bridge starts the configured `terminal64.exe` and waits for the pipe to become
available. It tracks a terminal it started so reconnects do not create duplicate
processes, and relaunches it only after the previous child has exited. An
explicit `MT5_PIPE_NAME` always takes precedence and never starts a terminal.
An explicit pipe or terminal path is required until automatic process discovery
has been verified in the target Wine environment.

## Protocol

The framing is a 24-byte little-endian header:

```text
[frame_length:u32][major:u16][type:u16][flags:u32]
[request_id:u64][metadata_length:u32][protobuf metadata][optional payload]
```

The complete language-neutral Protobuf schema is [proto/bridge.proto](proto/bridge.proto).
Header message types are: `Hello`, `HelloAck`, `Request`, `Response`,
`Error`, `ResponseStart`, `ResponseChunk`, `ResponseEnd`, `Cancel`,
`WindowUpdate`, `Ping`, and `Pong`.

Every request is finite. Large candles and ticks use one `ResponseStart`,
many bounded `ResponseChunk` frames with raw records, then one
`ResponseEnd`. Other arrays use bounded Protobuf `Value` record batches.
There are no subscriptions, unsolicited market-data events, or automatic
history pagination.

`RateV1` and `TickV1` records are both 60 bytes and use the layouts specified
in the schema documentation in the source. Their timestamps are MT5 raw
seconds/milliseconds; the bridge performs no timezone conversion.

Backends must send increasing nonzero request IDs, include the terminal epoch
from `HelloAck`, and send `WindowUpdate` frames when a large response exceeds
the initial one-megabyte credit allowance.

## Checks

```powershell
cargo test --workspace --manifest-path rust/Cargo.toml
```

After starting a bridge in Wine, verify framing/authentication interoperability
from the retained Go workspace:

```powershell
$env:MT5_BRIDGE_TOKEN = "the-same-token"
go run ./cmd/bridge-conformance
```
