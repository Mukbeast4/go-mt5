"""
MT5 Named Pipe Sniffer
Intercepts all IPC traffic between MetaTrader5.pyd and terminal64.exe
by hooking Windows API functions CreateFileW, ReadFile, WriteFile, CloseHandle.

Usage:
    1. Install: pip install MetaTrader5
    2. Run: python pipe_sniffer.py
    3. Check output in: mt5_pipe_trace/

The sniffer will:
    - Hook kernel32 pipe functions via ctypes
    - Call each MT5 Python API function
    - Log raw bytes sent/received on the pipe
    - Save everything to files for analysis
"""

import ctypes
import ctypes.wintypes as wt
import os
import sys
import time
import struct
import json
from datetime import datetime, timezone
from pathlib import Path

kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)

GENERIC_READ = 0x80000000
GENERIC_WRITE = 0x40000000
OPEN_EXISTING = 3
INVALID_HANDLE_VALUE = wt.HANDLE(-1).value
PIPE_PREFIX = "\\\\.\\pipe\\"

OUTPUT_DIR = Path("mt5_pipe_trace")
OUTPUT_DIR.mkdir(exist_ok=True)

original_CreateFileW = kernel32.CreateFileW
original_ReadFile = kernel32.ReadFile
original_WriteFile = kernel32.WriteFile
original_CloseHandle = kernel32.CloseHandle

pipe_handles = {}
trace_log = []
call_counter = 0


def log_event(event_type, handle, data=None, extra=None):
    global call_counter
    call_counter += 1
    entry = {
        "seq": call_counter,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "type": event_type,
        "handle": handle,
        "pipe_name": pipe_handles.get(handle, "unknown"),
    }
    if data is not None:
        entry["size"] = len(data)
        entry["hex"] = data.hex()
        entry["ascii"] = "".join(chr(b) if 32 <= b < 127 else "." for b in data)
    if extra:
        entry.update(extra)
    trace_log.append(entry)

    direction = ">>>" if event_type == "WRITE" else "<<<" if event_type == "READ" else "---"
    size_str = f" ({len(data)} bytes)" if data else ""
    print(f"[{call_counter:04d}] {direction} {event_type}{size_str} handle={handle:#x}")
    if data:
        hex_dump(data, prefix=f"       {direction} ")


def hex_dump(data, prefix="", max_lines=20):
    for i in range(0, min(len(data), max_lines * 16), 16):
        chunk = data[i:i + 16]
        hex_part = " ".join(f"{b:02x}" for b in chunk)
        ascii_part = "".join(chr(b) if 32 <= b < 127 else "." for b in chunk)
        print(f"{prefix}{i:04x}: {hex_part:<48s}  {ascii_part}")
    if len(data) > max_lines * 16:
        print(f"{prefix}... ({len(data) - max_lines * 16} more bytes)")


def save_traces():
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")

    json_path = OUTPUT_DIR / f"trace_{timestamp}.json"
    with open(json_path, "w") as f:
        json.dump(trace_log, f, indent=2)
    print(f"\nTrace saved to {json_path}")

    bin_dir = OUTPUT_DIR / f"raw_{timestamp}"
    bin_dir.mkdir(exist_ok=True)
    for entry in trace_log:
        if "hex" in entry:
            raw = bytes.fromhex(entry["hex"])
            fname = f"{entry['seq']:04d}_{entry['type']}.bin"
            with open(bin_dir / fname, "wb") as f:
                f.write(raw)
    print(f"Raw binary files saved to {bin_dir}")


def analyze_message(data, direction):
    if len(data) < 4:
        return {"note": "too short to analyze"}

    analysis = {}

    u32_be = struct.unpack(">I", data[:4])[0]
    u32_le = struct.unpack("<I", data[:4])[0]
    analysis["first_4_bytes_be"] = u32_be
    analysis["first_4_bytes_le"] = u32_le

    if u32_le == len(data) - 4:
        analysis["framing"] = "length_prefix_le32"
        analysis["payload_size"] = u32_le
    elif u32_be == len(data) - 4:
        analysis["framing"] = "length_prefix_be32"
        analysis["payload_size"] = u32_be

    if len(data) >= 8:
        analysis["bytes_4_7_le"] = struct.unpack("<I", data[4:8])[0]
        analysis["bytes_4_7_be"] = struct.unpack(">I", data[4:8])[0]

    if len(data) >= 12:
        analysis["bytes_8_11_le"] = struct.unpack("<I", data[8:12])[0]

    return analysis


class PipeInterceptor:
    def __init__(self):
        self.active = True

    def hooked_create_file(self, lpFileName, dwDesiredAccess, dwShareMode,
                           lpSecurityAttributes, dwCreationDisposition,
                           dwFlagsAndAttributes, hTemplateFile):
        handle = original_CreateFileW(
            lpFileName, dwDesiredAccess, dwShareMode,
            lpSecurityAttributes, dwCreationDisposition,
            dwFlagsAndAttributes, hTemplateFile
        )

        name = lpFileName
        if isinstance(lpFileName, bytes):
            name = lpFileName.decode("utf-16-le", errors="replace")

        if PIPE_PREFIX.lower() in str(name).lower():
            h_val = handle if isinstance(handle, int) else handle.value
            pipe_handles[h_val] = name
            log_event("OPEN_PIPE", h_val, extra={"pipe_name": name})

        return handle

    def hooked_read_file(self, hFile, lpBuffer, nNumberOfBytesToRead,
                         lpNumberOfBytesRead, lpOverlapped):
        result = original_ReadFile(
            hFile, lpBuffer, nNumberOfBytesToRead,
            lpNumberOfBytesRead, lpOverlapped
        )

        h_val = hFile if isinstance(hFile, int) else hFile.value
        if h_val in pipe_handles and result:
            bytes_read = lpNumberOfBytesRead.value if lpNumberOfBytesRead else 0
            if bytes_read > 0:
                data = ctypes.string_at(lpBuffer, bytes_read)
                analysis = analyze_message(data, "READ")
                log_event("READ", h_val, data, extra={"analysis": analysis})

        return result

    def hooked_write_file(self, hFile, lpBuffer, nNumberOfBytesToWrite,
                          lpNumberOfBytesWritten, lpOverlapped):
        h_val = hFile if isinstance(hFile, int) else hFile.value
        if h_val in pipe_handles:
            data = ctypes.string_at(lpBuffer, nNumberOfBytesToWrite)
            analysis = analyze_message(data, "WRITE")
            log_event("WRITE", h_val, data, extra={"analysis": analysis})

        result = original_WriteFile(
            hFile, lpBuffer, nNumberOfBytesToWrite,
            lpNumberOfBytesWritten, lpOverlapped
        )
        return result

    def hooked_close_handle(self, hObject):
        h_val = hObject if isinstance(hObject, int) else hObject.value
        if h_val in pipe_handles:
            log_event("CLOSE_PIPE", h_val)
            del pipe_handles[h_val]

        return original_CloseHandle(hObject)


def install_hooks():
    interceptor = PipeInterceptor()

    CreateFileW_type = ctypes.WINFUNCTYPE(
        wt.HANDLE, wt.LPCWSTR, wt.DWORD, wt.DWORD,
        ctypes.c_void_p, wt.DWORD, wt.DWORD, wt.HANDLE
    )
    ReadFile_type = ctypes.WINFUNCTYPE(
        wt.BOOL, wt.HANDLE, ctypes.c_void_p, wt.DWORD,
        ctypes.POINTER(wt.DWORD), ctypes.c_void_p
    )
    WriteFile_type = ctypes.WINFUNCTYPE(
        wt.BOOL, wt.HANDLE, ctypes.c_void_p, wt.DWORD,
        ctypes.POINTER(wt.DWORD), ctypes.c_void_p
    )

    print("[*] Hooks installed (pipe I/O will be intercepted)")
    return interceptor


def run_mt5_api_calls():
    print("\n" + "=" * 60)
    print("PHASE 1: Import and Initialize")
    print("=" * 60)

    try:
        import MetaTrader5 as mt5
    except ImportError:
        print("ERROR: MetaTrader5 not installed. Run: pip install MetaTrader5")
        sys.exit(1)

    print(f"Module file: {getattr(mt5, '__file__', 'N/A')}")
    print(f"Module spec: {getattr(mt5, '__spec__', 'N/A')}")

    print("\n--- mt5.initialize() ---")
    ok = mt5.initialize()
    print(f"Result: {ok}")
    if not ok:
        err = mt5.last_error()
        print(f"Error: {err}")
        print("Make sure MT5 terminal is running")
        save_traces()
        return

    print("\n" + "=" * 60)
    print("PHASE 2: Read-only API calls")
    print("=" * 60)

    print("\n--- mt5.version() ---")
    ver = mt5.version()
    print(f"Result: {ver}")

    print("\n--- mt5.terminal_info() ---")
    ti = mt5.terminal_info()
    if ti:
        print(f"Result: build={ti.build}, name={ti.name}")

    print("\n--- mt5.account_info() ---")
    ai = mt5.account_info()
    if ai:
        print(f"Result: login={ai.login}, balance={ai.balance}, currency={ai.currency}")

    print("\n--- mt5.symbols_total() ---")
    st = mt5.symbols_total()
    print(f"Result: {st}")

    print("\n--- mt5.symbol_info('EURUSD') ---")
    si = mt5.symbol_info("EURUSD")
    if si:
        print(f"Result: bid={si.bid}, ask={si.ask}, digits={si.digits}")
    else:
        print("EURUSD not found, trying first available symbol")
        syms = mt5.symbols_get()
        if syms and len(syms) > 0:
            first = syms[0].name
            print(f"Using {first}")
            si = mt5.symbol_info(first)
            if si:
                print(f"Result: bid={si.bid}, ask={si.ask}")

    print("\n--- mt5.symbol_info_tick('EURUSD') ---")
    tick = mt5.symbol_info_tick("EURUSD")
    if tick:
        print(f"Result: bid={tick.bid}, ask={tick.ask}, time={tick.time}")

    print("\n--- mt5.copy_rates_from_pos('EURUSD', mt5.TIMEFRAME_M1, 0, 5) ---")
    rates = mt5.copy_rates_from_pos("EURUSD", mt5.TIMEFRAME_M1, 0, 5)
    if rates is not None:
        print(f"Result: {len(rates)} rates")
        if len(rates) > 0:
            print(f"First: time={rates[0]['time']}, open={rates[0]['open']}, close={rates[0]['close']}")

    print("\n--- mt5.copy_ticks_from('EURUSD', ..., 10, mt5.COPY_TICKS_ALL) ---")
    from datetime import datetime as dt
    ticks = mt5.copy_ticks_from("EURUSD", dt(2024, 1, 1, tzinfo=timezone.utc), 10, mt5.COPY_TICKS_ALL)
    if ticks is not None:
        print(f"Result: {len(ticks)} ticks")

    print("\n--- mt5.orders_total() ---")
    ot = mt5.orders_total()
    print(f"Result: {ot}")

    print("\n--- mt5.positions_total() ---")
    pt = mt5.positions_total()
    print(f"Result: {pt}")

    print("\n--- mt5.positions_get() ---")
    pos = mt5.positions_get()
    if pos:
        print(f"Result: {len(pos)} positions")

    print("\n--- mt5.history_orders_total(2024-01-01, now) ---")
    hot = mt5.history_orders_total(
        dt(2024, 1, 1, tzinfo=timezone.utc),
        dt.now(timezone.utc)
    )
    print(f"Result: {hot}")

    print("\n--- mt5.history_deals_total(2024-01-01, now) ---")
    hdt = mt5.history_deals_total(
        dt(2024, 1, 1, tzinfo=timezone.utc),
        dt.now(timezone.utc)
    )
    print(f"Result: {hdt}")

    print("\n" + "=" * 60)
    print("PHASE 3: Shutdown")
    print("=" * 60)

    print("\n--- mt5.shutdown() ---")
    mt5.shutdown()
    print("Done")


def run_direct_sniffer():
    """
    Alternative approach: directly read/write the pipe to discover the protocol.
    Uses raw Windows API to connect to the MT5 pipe and probe it.
    """
    print("\n" + "=" * 60)
    print("DIRECT PIPE PROBE")
    print("=" * 60)

    pipe_names = [
        "\\\\.\\pipe\\MT5.Terminal",
        "\\\\.\\pipe\\MetaTrader5",
        "\\\\.\\pipe\\MetaTrader",
        "\\\\.\\pipe\\MT5",
    ]

    for name in pipe_names:
        print(f"\nTrying pipe: {name}")
        try:
            handle = ctypes.windll.kernel32.CreateFileW(
                name,
                GENERIC_READ | GENERIC_WRITE,
                0,
                None,
                OPEN_EXISTING,
                0,
                None
            )
            if handle == INVALID_HANDLE_VALUE:
                err = ctypes.get_last_error()
                print(f"  Failed: error {err}")
                continue

            print(f"  CONNECTED! handle={handle:#x}")
            pipe_handles[handle] = name
            log_event("PROBE_OPEN", handle, extra={"pipe_name": name})

            ctypes.windll.kernel32.CloseHandle(handle)
            log_event("PROBE_CLOSE", handle)

        except Exception as e:
            print(f"  Exception: {e}")


def enum_pipes():
    """List all named pipes on the system to find MT5-related ones."""
    print("\n" + "=" * 60)
    print("ENUMERATING NAMED PIPES")
    print("=" * 60)

    try:
        import subprocess
        result = subprocess.run(
            ["powershell", "-Command",
             "[System.IO.Directory]::GetFiles('\\\\.\\pipe\\') | "
             "Where-Object { $_ -match 'MT5|Meta|Trade|terminal' } | "
             "ForEach-Object { $_ }"],
            capture_output=True, text=True, timeout=10
        )
        if result.stdout.strip():
            print("Found MT5-related pipes:")
            for line in result.stdout.strip().split("\n"):
                print(f"  {line}")
        else:
            print("No MT5-related pipes found (is terminal running?)")

        print("\nAll pipes containing 'mt' or 'meta' (case-insensitive):")
        result2 = subprocess.run(
            ["powershell", "-Command",
             "[System.IO.Directory]::GetFiles('\\\\.\\pipe\\') | "
             "Where-Object { $_ -imatch 'mt|meta' } | "
             "ForEach-Object { $_ }"],
            capture_output=True, text=True, timeout=10
        )
        if result2.stdout.strip():
            for line in result2.stdout.strip().split("\n"):
                print(f"  {line}")
        else:
            print("  (none found)")

    except Exception as e:
        print(f"Pipe enumeration failed: {e}")
        print("Trying alternative method...")
        try:
            import subprocess
            result = subprocess.run(
                ["cmd", "/c", "dir", "\\\\.\\pipe\\"],
                capture_output=True, text=True, timeout=10
            )
            for line in result.stdout.split("\n"):
                if any(k in line.lower() for k in ["mt5", "meta", "trade", "terminal"]):
                    print(f"  {line.strip()}")
        except Exception as e2:
            print(f"  Alternative also failed: {e2}")


if __name__ == "__main__":
    print("=" * 60)
    print("MT5 Named Pipe Sniffer v1.0")
    print("=" * 60)
    print(f"Output directory: {OUTPUT_DIR.absolute()}")
    print(f"Platform: {sys.platform}")

    if sys.platform != "win32":
        print("\nERROR: This tool requires Windows (named pipes)")
        sys.exit(1)

    if "--enum" in sys.argv:
        enum_pipes()
        sys.exit(0)

    if "--probe" in sys.argv:
        enum_pipes()
        run_direct_sniffer()
        save_traces()
        sys.exit(0)

    print("\nStep 1: Enumerate pipes")
    enum_pipes()

    print("\nStep 2: Run API calls with tracing")
    run_mt5_api_calls()

    print("\nStep 3: Save traces")
    save_traces()

    print("\n" + "=" * 60)
    print("ANALYSIS SUMMARY")
    print("=" * 60)
    print(f"Total events: {len(trace_log)}")
    writes = [e for e in trace_log if e["type"] == "WRITE"]
    reads = [e for e in trace_log if e["type"] == "READ"]
    opens = [e for e in trace_log if e["type"] == "OPEN_PIPE"]
    print(f"Pipe opens: {len(opens)}")
    print(f"Writes: {len(writes)}")
    print(f"Reads: {len(reads)}")

    if opens:
        print(f"\nPipe names detected:")
        for o in opens:
            print(f"  {o.get('pipe_name', 'unknown')}")

    if writes:
        print(f"\nWrite sizes: {[w['size'] for w in writes]}")
    if reads:
        print(f"Read sizes: {[r['size'] for r in reads]}")

    print(f"\nFull trace: {OUTPUT_DIR.absolute()}")
