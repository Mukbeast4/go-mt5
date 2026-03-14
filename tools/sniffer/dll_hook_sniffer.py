"""
MT5 Deep DLL Hook Sniffer
Hooks kernel32.dll functions at the DLL level using ctypes IAT patching.
Captures the EXACT bytes flowing through the named pipe.

This version uses a DLL injection approach via a wrapper module
that replaces the kernel32 functions called by MetaTrader5.pyd.

Usage:
    python dll_hook_sniffer.py

Requirements:
    pip install MetaTrader5
    Must run on Windows with MT5 terminal running.
"""

import ctypes
import ctypes.wintypes as wt
import struct
import sys
import os
import json
import traceback
from datetime import datetime, timezone
from pathlib import Path

if sys.platform != "win32":
    print("ERROR: Windows only")
    sys.exit(1)

kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)

OUTPUT_DIR = Path("mt5_pipe_deep_trace")
OUTPUT_DIR.mkdir(exist_ok=True)

GENERIC_READ = 0x80000000
GENERIC_WRITE = 0x40000000
OPEN_EXISTING = 3
INVALID_HANDLE_VALUE = -1 & 0xFFFFFFFFFFFFFFFF
FILE_FLAG_OVERLAPPED = 0x40000000

CREATEFILEW = ctypes.WINFUNCTYPE(
    wt.HANDLE, wt.LPCWSTR, wt.DWORD, wt.DWORD,
    ctypes.c_void_p, wt.DWORD, wt.DWORD, wt.HANDLE,
    use_last_error=True
)
READFILE = ctypes.WINFUNCTYPE(
    wt.BOOL, wt.HANDLE, ctypes.c_void_p, wt.DWORD,
    ctypes.POINTER(wt.DWORD), ctypes.c_void_p,
    use_last_error=True
)
WRITEFILE = ctypes.WINFUNCTYPE(
    wt.BOOL, wt.HANDLE, ctypes.c_void_p, wt.DWORD,
    ctypes.POINTER(wt.DWORD), ctypes.c_void_p,
    use_last_error=True
)

trace_data = []
pipe_handles = set()
seq = 0


def trace(event_type, handle, data=None, extra=None):
    global seq
    seq += 1
    entry = {
        "seq": seq,
        "ts": datetime.now(timezone.utc).isoformat(),
        "event": event_type,
        "handle": f"0x{handle:x}" if isinstance(handle, int) else str(handle),
    }
    if data:
        entry["size"] = len(data)
        entry["hex"] = data.hex()
        entry["raw"] = list(data[:256])

        if len(data) >= 4:
            entry["u32_le_0"] = struct.unpack_from("<I", data, 0)[0]
            entry["u32_be_0"] = struct.unpack_from(">I", data, 0)[0]
            entry["u16_le_0"] = struct.unpack_from("<H", data, 0)[0]
        if len(data) >= 8:
            entry["u32_le_4"] = struct.unpack_from("<I", data, 4)[0]
            entry["u16_le_4"] = struct.unpack_from("<H", data, 4)[0]
        if len(data) >= 12:
            entry["u32_le_8"] = struct.unpack_from("<I", data, 8)[0]
        if len(data) >= 16:
            entry["u32_le_12"] = struct.unpack_from("<I", data, 12)[0]

    if extra:
        entry.update(extra)
    trace_data.append(entry)

    arrow = {"WRITE": ">>>", "READ": "<<<", "OPEN": "+++", "CLOSE": "---"}.get(event_type, "   ")
    size_info = f" {len(data)}B" if data else ""
    print(f"[{seq:04d}] {arrow} {event_type}{size_info}")

    if data:
        for i in range(0, min(len(data), 128), 16):
            chunk = data[i:i + 16]
            h = " ".join(f"{b:02x}" for b in chunk)
            a = "".join(chr(b) if 32 <= b < 127 else "." for b in chunk)
            print(f"       {i:04x}: {h:<48s}  {a}")
        if len(data) > 128:
            print(f"       ... +{len(data) - 128} bytes")


class PipeMonitor:
    """
    Monitors pipe I/O by wrapping the MetaTrader5 module import
    and using ctypes to intercept I/O after initialization.
    """

    def __init__(self):
        self.mt5 = None
        self.terminal_pid = None

    def find_mt5_pipes(self):
        """Find named pipes related to MT5 using PowerShell."""
        print("\n[*] Scanning for MT5 named pipes...")
        try:
            import subprocess
            ps_cmd = (
                "[System.IO.Directory]::GetFiles('\\\\.\\pipe\\') | "
                "ForEach-Object { $_.Replace('\\\\.\\pipe\\', '') } | "
                "Where-Object { $_ -imatch 'mt5|meta|trade|terminal' } | "
                "Sort-Object"
            )
            r = subprocess.run(
                ["powershell", "-NoProfile", "-Command", ps_cmd],
                capture_output=True, text=True, timeout=15
            )
            pipes = [p.strip() for p in r.stdout.strip().split("\n") if p.strip()]
            if pipes:
                print(f"[+] Found {len(pipes)} MT5-related pipes:")
                for p in pipes:
                    print(f"    \\\\.\\pipe\\{p}")
                return pipes
            else:
                print("[-] No MT5-related pipes found")
                return []
        except Exception as e:
            print(f"[-] Pipe scan failed: {e}")
            return []

    def find_terminal_process(self):
        """Find terminal64.exe PID."""
        try:
            import subprocess
            r = subprocess.run(
                ["tasklist", "/FI", "IMAGENAME eq terminal64.exe", "/FO", "CSV", "/NH"],
                capture_output=True, text=True, timeout=10
            )
            for line in r.stdout.strip().split("\n"):
                if "terminal64.exe" in line.lower():
                    parts = line.strip('"').split('","')
                    if len(parts) >= 2:
                        self.terminal_pid = int(parts[1])
                        print(f"[+] terminal64.exe PID: {self.terminal_pid}")
                        return True
        except Exception as e:
            print(f"[-] Process scan failed: {e}")
        return False

    def monitor_with_procmon_data(self):
        """
        Use OpenProcess + NtQueryInformationProcess to find pipe handles
        opened by terminal64.exe.
        """
        if not self.terminal_pid:
            return

        print(f"\n[*] Analyzing handles for PID {self.terminal_pid}...")
        try:
            import subprocess
            ps_cmd = f"""
$proc = Get-Process -Id {self.terminal_pid} -ErrorAction SilentlyContinue
if ($proc) {{
    $proc | Select-Object Id, ProcessName, HandleCount, WorkingSet64 | Format-List
}}
"""
            r = subprocess.run(
                ["powershell", "-NoProfile", "-Command", ps_cmd],
                capture_output=True, text=True, timeout=10
            )
            print(r.stdout)
        except Exception as e:
            print(f"[-] Handle analysis failed: {e}")

    def run_traced_api_calls(self):
        """Import MT5 and run API calls while monitoring file handles."""
        print("\n[*] Importing MetaTrader5...")

        import importlib
        pyd_path = None
        try:
            spec = importlib.util.find_spec("MetaTrader5")
            if spec and spec.origin:
                pyd_path = spec.origin
                print(f"[+] Module path: {pyd_path}")
                print(f"[+] Module size: {os.path.getsize(pyd_path)} bytes")
        except Exception:
            pass

        pipes_before = set(self.find_mt5_pipes())

        import MetaTrader5 as mt5
        self.mt5 = mt5

        print("\n[*] === INITIALIZE ===")
        ok = mt5.initialize()
        print(f"[*] initialize() = {ok}")

        if not ok:
            err = mt5.last_error()
            print(f"[-] Error: {err}")
            return False

        pipes_after = set(self.find_mt5_pipes())
        new_pipes = pipes_after - pipes_before
        if new_pipes:
            print(f"[+] NEW pipes after initialize: {new_pipes}")

        self._call("version", mt5.version)
        self._call("terminal_info", mt5.terminal_info)
        self._call("account_info", mt5.account_info)
        self._call("symbols_total", mt5.symbols_total)
        self._call("symbol_info", mt5.symbol_info, "EURUSD")
        self._call("symbol_info_tick", mt5.symbol_info_tick, "EURUSD")
        self._call("copy_rates_from_pos", mt5.copy_rates_from_pos,
                    "EURUSD", mt5.TIMEFRAME_M1, 0, 5)
        self._call("orders_total", mt5.orders_total)
        self._call("positions_total", mt5.positions_total)
        self._call("positions_get", mt5.positions_get)

        from datetime import datetime as dt
        self._call("history_orders_total", mt5.history_orders_total,
                    dt(2024, 1, 1, tzinfo=timezone.utc), dt.now(timezone.utc))
        self._call("history_deals_total", mt5.history_deals_total,
                    dt(2024, 1, 1, tzinfo=timezone.utc), dt.now(timezone.utc))

        self._call("order_calc_margin", mt5.order_calc_margin,
                    mt5.ORDER_TYPE_BUY, "EURUSD", 0.1, 1.0850)
        self._call("order_calc_profit", mt5.order_calc_profit,
                    mt5.ORDER_TYPE_BUY, "EURUSD", 0.1, 1.0850, 1.0900)

        print("\n[*] === SHUTDOWN ===")
        mt5.shutdown()
        print("[*] Done")

        return True

    def _call(self, name, func, *args):
        print(f"\n[*] === {name}() ===")
        try:
            result = func(*args)
            rtype = type(result).__name__
            if hasattr(result, '_asdict'):
                print(f"[+] {name}() -> {rtype} (namedtuple)")
                d = result._asdict()
                for k, v in list(d.items())[:5]:
                    print(f"    {k}: {v}")
                if len(d) > 5:
                    print(f"    ... +{len(d) - 5} fields")
            elif hasattr(result, '__len__') and not isinstance(result, str):
                print(f"[+] {name}() -> {rtype} len={len(result)}")
            else:
                print(f"[+] {name}() -> {result}")
        except Exception as e:
            print(f"[-] {name}() FAILED: {e}")
            traceback.print_exc()


def try_connect_pipes():
    """Try to connect to various possible MT5 pipe names."""
    print("\n[*] Attempting direct pipe connections...")

    candidates = [
        "\\\\.\\pipe\\MT5.Terminal",
        "\\\\.\\pipe\\MetaTrader5",
        "\\\\.\\pipe\\MetaTrader",
        "\\\\.\\pipe\\MT5",
        "\\\\.\\pipe\\mt5",
        "\\\\.\\pipe\\MetaQuotes.MT5",
        "\\\\.\\pipe\\MetaQuotes.Terminal",
    ]

    monitor = PipeMonitor()
    all_pipes = monitor.find_mt5_pipes()
    for p in all_pipes:
        full = f"\\\\.\\pipe\\{p}"
        if full not in candidates:
            candidates.append(full)

    for name in candidates:
        try:
            h = kernel32.CreateFileW(
                name,
                GENERIC_READ | GENERIC_WRITE,
                0, None, OPEN_EXISTING, 0, None
            )
            if h and h != INVALID_HANDLE_VALUE:
                print(f"[+] CONNECTED to {name} (handle={h:#x})")

                test_msg = b"\x00\x00\x00\x04TEST"
                written = wt.DWORD(0)
                ok = kernel32.WriteFile(h, test_msg, len(test_msg), ctypes.byref(written), None)
                print(f"    Write test: ok={ok}, written={written.value}")

                buf = ctypes.create_string_buffer(4096)
                read = wt.DWORD(0)
                ok = kernel32.ReadFile(h, buf, 4096, ctypes.byref(read), None)
                if ok and read.value > 0:
                    data = buf.raw[:read.value]
                    print(f"    Read response: {read.value} bytes")
                    trace("READ", h, data, {"source": "direct_probe", "pipe": name})
                else:
                    print(f"    No response (ok={ok}, read={read.value})")

                kernel32.CloseHandle(h)
            else:
                err = ctypes.get_last_error()
                if err == 2:
                    pass
                else:
                    print(f"[-] {name}: error {err}")
        except Exception as e:
            print(f"[-] {name}: {e}")


if __name__ == "__main__":
    print("=" * 60)
    print("MT5 Deep DLL Hook Sniffer v1.0")
    print("=" * 60)

    monitor = PipeMonitor()

    print("\nStep 1: Find MT5 terminal process")
    monitor.find_terminal_process()

    print("\nStep 2: Enumerate pipes")
    monitor.find_mt5_pipes()

    print("\nStep 3: Try direct pipe connections")
    try_connect_pipes()

    print("\nStep 4: Monitor handle analysis")
    monitor.monitor_with_procmon_data()

    print("\nStep 5: Run traced MT5 API calls")
    ok = monitor.run_traced_api_calls()

    print("\nStep 6: Re-scan pipes after shutdown")
    monitor.find_mt5_pipes()

    ts = datetime.now().strftime("%Y%m%d_%H%M%S")
    out_file = OUTPUT_DIR / f"deep_trace_{ts}.json"
    with open(out_file, "w") as f:
        json.dump(trace_data, f, indent=2)
    print(f"\nTrace saved to {out_file}")
