"""
MT5 Man-in-the-Middle Pipe Proxy
Creates a fake named pipe, forwards traffic to the real MT5 pipe,
and logs every byte in both directions.

This is the most effective approach: we sit between Python and MT5.

Architecture:
    MetaTrader5.pyd --> [FAKE PIPE (this script)] --> [REAL MT5 PIPE (terminal64.exe)]

Usage:
    1. Find the real pipe name first:
       python pipe_sniffer.py --enum

    2. Stop the MT5 terminal

    3. Start this proxy with the real pipe name:
       python mitm_pipe.py --real-pipe "\\.\pipe\MT5.Terminal.XXXX"

    4. Start the MT5 terminal (it will create its pipe)

    5. In another terminal, run Python MT5 code

    6. All traffic is logged to mt5_mitm_trace/

Alternative approach (no terminal restart needed):
    1. Rename/find the real pipe
    2. Create our pipe with the expected name
    3. Forward all traffic

Requires: Administrator privileges (to create named pipes)
"""

import ctypes
import ctypes.wintypes as wt
import struct
import sys
import json
import threading
import time
from pathlib import Path
from datetime import datetime, timezone

if sys.platform != "win32":
    print("ERROR: Windows only")
    sys.exit(1)

kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)

PIPE_ACCESS_DUPLEX = 0x00000003
PIPE_TYPE_BYTE = 0x00000000
PIPE_READMODE_BYTE = 0x00000000
PIPE_WAIT = 0x00000000
PIPE_UNLIMITED_INSTANCES = 255
GENERIC_READ = 0x80000000
GENERIC_WRITE = 0x40000000
OPEN_EXISTING = 3
INVALID_HANDLE_VALUE = wt.HANDLE(-1).value
NMPWAIT_USE_DEFAULT_WAIT = 0
NMPWAIT_WAIT_FOREVER = 0xFFFFFFFF
ERROR_PIPE_CONNECTED = 535
ERROR_MORE_DATA = 234
BUFFER_SIZE = 1024 * 1024

OUTPUT_DIR = Path("mt5_mitm_trace")
OUTPUT_DIR.mkdir(exist_ok=True)

trace_log = []
seq_counter = 0
lock = threading.Lock()


def log_traffic(direction, data, context=""):
    global seq_counter
    with lock:
        seq_counter += 1
        entry = {
            "seq": seq_counter,
            "ts": datetime.now(timezone.utc).isoformat(),
            "direction": direction,
            "size": len(data),
            "hex": data.hex(),
            "context": context,
        }

        if len(data) >= 4:
            entry["u32_le"] = struct.unpack_from("<I", data, 0)[0]
            entry["u32_be"] = struct.unpack_from(">I", data, 0)[0]
        if len(data) >= 8:
            entry["u32_le_4"] = struct.unpack_from("<I", data, 4)[0]
        if len(data) >= 12:
            entry["u32_le_8"] = struct.unpack_from("<I", data, 8)[0]
        if len(data) >= 16:
            entry["u32_le_12"] = struct.unpack_from("<I", data, 12)[0]

        trace_log.append(entry)

        arrow = ">>>" if direction == "CLIENT_TO_MT5" else "<<<"
        print(f"[{seq_counter:04d}] {arrow} {direction} ({len(data)} bytes)")
        for i in range(0, min(len(data), 256), 16):
            chunk = data[i:i + 16]
            h = " ".join(f"{b:02x}" for b in chunk)
            a = "".join(chr(b) if 32 <= b < 127 else "." for b in chunk)
            print(f"       {i:04x}: {h:<48s}  {a}")
        if len(data) > 256:
            print(f"       ... +{len(data) - 256} more bytes")
        print()


def create_named_pipe(pipe_name):
    """Create a named pipe server (we pretend to be MT5)."""
    h = kernel32.CreateNamedPipeW(
        pipe_name,
        PIPE_ACCESS_DUPLEX,
        PIPE_TYPE_BYTE | PIPE_READMODE_BYTE | PIPE_WAIT,
        PIPE_UNLIMITED_INSTANCES,
        BUFFER_SIZE,
        BUFFER_SIZE,
        0,
        None
    )
    if h == INVALID_HANDLE_VALUE:
        err = ctypes.get_last_error()
        raise OSError(f"CreateNamedPipe failed: error {err}")
    return h


def connect_to_pipe(pipe_name, timeout_ms=10000):
    """Connect to an existing named pipe (connect to real MT5)."""
    ok = kernel32.WaitNamedPipeW(pipe_name, timeout_ms)
    if not ok:
        err = ctypes.get_last_error()
        raise OSError(f"WaitNamedPipe failed: error {err}")

    h = kernel32.CreateFileW(
        pipe_name,
        GENERIC_READ | GENERIC_WRITE,
        0, None, OPEN_EXISTING, 0, None
    )
    if h == INVALID_HANDLE_VALUE:
        err = ctypes.get_last_error()
        raise OSError(f"CreateFile failed: error {err}")
    return h


def pipe_read(handle, max_size=BUFFER_SIZE):
    """Read from a pipe handle."""
    buf = ctypes.create_string_buffer(max_size)
    bytes_read = wt.DWORD(0)
    ok = kernel32.ReadFile(handle, buf, max_size, ctypes.byref(bytes_read), None)
    if not ok:
        err = ctypes.get_last_error()
        if err == ERROR_MORE_DATA:
            return buf.raw[:bytes_read.value], True
        return None, False
    if bytes_read.value == 0:
        return None, False
    return buf.raw[:bytes_read.value], True


def pipe_write(handle, data):
    """Write to a pipe handle."""
    written = wt.DWORD(0)
    ok = kernel32.WriteFile(handle, data, len(data), ctypes.byref(written), None)
    if not ok:
        err = ctypes.get_last_error()
        raise OSError(f"WriteFile failed: error {err}")
    return written.value


def forward_thread(src_handle, dst_handle, direction, stop_event):
    """Forward data from src to dst, logging everything."""
    while not stop_event.is_set():
        data, ok = pipe_read(src_handle, BUFFER_SIZE)
        if not ok or data is None:
            print(f"[*] {direction}: pipe closed")
            stop_event.set()
            break

        log_traffic(direction, data)

        try:
            pipe_write(dst_handle, data)
        except OSError as e:
            print(f"[*] {direction}: write failed: {e}")
            stop_event.set()
            break


def run_mitm(fake_pipe_name, real_pipe_name):
    """
    Create a MITM proxy between the Python client and MT5 terminal.

    fake_pipe_name: the name Python expects (we serve this)
    real_pipe_name: the actual MT5 pipe (we connect to this)
    """
    print(f"[*] MITM Pipe Proxy")
    print(f"[*] Fake pipe (for Python): {fake_pipe_name}")
    print(f"[*] Real pipe (MT5):        {real_pipe_name}")
    print()

    print("[*] Creating fake pipe server...")
    fake_handle = create_named_pipe(fake_pipe_name)
    print(f"[+] Fake pipe created: handle={fake_handle:#x}")

    print("[*] Waiting for Python client to connect...")
    ok = kernel32.ConnectNamedPipe(fake_handle, None)
    err = ctypes.get_last_error()
    if not ok and err != ERROR_PIPE_CONNECTED:
        print(f"[-] ConnectNamedPipe failed: {err}")
        return
    print("[+] Python client connected!")

    print("[*] Connecting to real MT5 pipe...")
    try:
        real_handle = connect_to_pipe(real_pipe_name, 30000)
        print(f"[+] Connected to MT5: handle={real_handle:#x}")
    except OSError as e:
        print(f"[-] Cannot connect to MT5: {e}")
        kernel32.CloseHandle(fake_handle)
        return

    stop_event = threading.Event()

    t1 = threading.Thread(
        target=forward_thread,
        args=(fake_handle, real_handle, "CLIENT_TO_MT5", stop_event),
        daemon=True
    )
    t2 = threading.Thread(
        target=forward_thread,
        args=(real_handle, fake_handle, "MT5_TO_CLIENT", stop_event),
        daemon=True
    )

    t1.start()
    t2.start()

    print("[*] Proxy running... Press Ctrl+C to stop\n")

    try:
        while not stop_event.is_set():
            time.sleep(0.1)
    except KeyboardInterrupt:
        print("\n[*] Stopping proxy...")
        stop_event.set()

    kernel32.CloseHandle(fake_handle)
    kernel32.CloseHandle(real_handle)

    save_trace()


def run_standalone_capture():
    """
    Standalone mode: enumerate pipes, then ask user which to proxy.
    If we find the MT5 pipe, we create a copy and intercept.
    """
    print("[*] Standalone capture mode")
    print("[*] This mode intercepts pipe traffic without a full MITM setup")
    print()

    import subprocess

    print("[*] Looking for MT5 pipes...")
    try:
        r = subprocess.run(
            ["powershell", "-NoProfile", "-Command",
             "[System.IO.Directory]::GetFiles('\\\\.\\pipe\\') | "
             "ForEach-Object { $_.Replace('\\\\.\\pipe\\', '') } | "
             "Sort-Object"],
            capture_output=True, text=True, timeout=15
        )
        all_pipes = [p.strip() for p in r.stdout.strip().split("\n") if p.strip()]
        mt5_pipes = [p for p in all_pipes if any(
            k in p.lower() for k in ["mt5", "meta", "trade", "terminal"]
        )]

        if mt5_pipes:
            print(f"[+] Found {len(mt5_pipes)} MT5-related pipes:")
            for i, p in enumerate(mt5_pipes):
                print(f"    [{i}] \\\\.\\pipe\\{p}")

            print()
            for p in mt5_pipes:
                full_name = f"\\\\.\\pipe\\{p}"
                print(f"\n[*] Trying to connect to {full_name}...")
                try:
                    h = kernel32.CreateFileW(
                        full_name,
                        GENERIC_READ | GENERIC_WRITE,
                        0, None, OPEN_EXISTING, 0, None
                    )
                    if h and h != INVALID_HANDLE_VALUE:
                        print(f"[+] CONNECTED to {full_name}")
                        print(f"[*] Attempting to read initial data...")

                        data, ok = pipe_read(h, 4096)
                        if ok and data:
                            log_traffic("INITIAL_READ", data, full_name)
                        else:
                            print("[*] No initial data available")

                        kernel32.CloseHandle(h)
                    else:
                        err = ctypes.get_last_error()
                        print(f"[-] Cannot connect: error {err}")
                except Exception as e:
                    print(f"[-] Error: {e}")
        else:
            print("[-] No MT5 pipes found. Is the terminal running?")
            print("[*] All pipes containing common keywords:")
            for p in all_pipes:
                if len(p) < 50:
                    print(f"    {p}")

    except Exception as e:
        print(f"[-] Enumeration failed: {e}")

    if trace_log:
        save_trace()


def save_trace():
    ts = datetime.now().strftime("%Y%m%d_%H%M%S")
    out_file = OUTPUT_DIR / f"mitm_trace_{ts}.json"
    with open(out_file, "w") as f:
        json.dump(trace_log, f, indent=2)
    print(f"\n[*] Trace saved to {out_file}")
    print(f"[*] Total messages: {len(trace_log)}")

    writes = [e for e in trace_log if "CLIENT" in e["direction"]]
    reads = [e for e in trace_log if "MT5" in e["direction"]]
    print(f"[*] Client->MT5: {len(writes)} messages")
    print(f"[*] MT5->Client: {len(reads)} messages")

    if writes:
        print(f"\n[*] Request sizes: {[w['size'] for w in writes[:20]]}")
    if reads:
        print(f"[*] Response sizes: {[r['size'] for r in reads[:20]]}")

    bin_dir = OUTPUT_DIR / f"raw_{ts}"
    bin_dir.mkdir(exist_ok=True)
    for entry in trace_log:
        raw = bytes.fromhex(entry["hex"])
        fname = f"{entry['seq']:04d}_{entry['direction']}.bin"
        with open(bin_dir / fname, "wb") as f:
            f.write(raw)
    print(f"[*] Raw files: {bin_dir}")


if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser(description="MT5 MITM Pipe Proxy")
    parser.add_argument("--fake-pipe", default=None,
                        help="Pipe name for Python to connect to")
    parser.add_argument("--real-pipe", default=None,
                        help="Real MT5 pipe name to forward to")
    parser.add_argument("--standalone", action="store_true",
                        help="Standalone mode: enumerate and probe pipes")
    args = parser.parse_args()

    if args.standalone or (not args.fake_pipe and not args.real_pipe):
        run_standalone_capture()
    elif args.fake_pipe and args.real_pipe:
        run_mitm(args.fake_pipe, args.real_pipe)
    else:
        print("ERROR: specify both --fake-pipe and --real-pipe for MITM mode")
        print("       or use --standalone for probe mode")
