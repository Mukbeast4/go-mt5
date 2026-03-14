"""
MT5 Process Monitor Capture
Uses Windows ETW (Event Tracing for Windows) to capture pipe I/O
from terminal64.exe without modifying the process.

This is the most reliable method: no hooks, no injection,
just OS-level tracing of all pipe operations.

Usage:
    Run as Administrator:
    python procmon_capture.py

Alternative: Use Sysinternals Process Monitor manually:
    1. Download: https://learn.microsoft.com/en-us/sysinternals/downloads/procmon
    2. Add filter: Process Name is "python.exe" then Include
    3. Add filter: Path contains "pipe" then Include
    4. Add filter: Operation is "CreateFile" then Include
    5. Add filter: Operation is "ReadFile" then Include
    6. Add filter: Operation is "WriteFile" then Include
    7. Run pipe_sniffer.py in another terminal
    8. Export results as CSV
    9. Run: python procmon_capture.py --analyze procmon_export.csv
"""

import sys
import csv
import json
import struct
from pathlib import Path
from datetime import datetime, timezone

OUTPUT_DIR = Path("mt5_procmon_analysis")
OUTPUT_DIR.mkdir(exist_ok=True)


def analyze_procmon_csv(csv_path):
    """Analyze a Process Monitor CSV export for pipe operations."""
    print(f"[*] Analyzing {csv_path}...")

    pipe_ops = []
    with open(csv_path, "r", encoding="utf-8-sig") as f:
        reader = csv.DictReader(f)
        for row in reader:
            path = row.get("Path", "")
            op = row.get("Operation", "")
            proc = row.get("Process Name", "")
            detail = row.get("Detail", "")
            result = row.get("Result", "")

            if "pipe" not in path.lower():
                continue

            if any(k in path.lower() for k in ["mt5", "meta", "trade", "terminal"]):
                pipe_ops.append({
                    "time": row.get("Time of Day", ""),
                    "process": proc,
                    "pid": row.get("PID", ""),
                    "operation": op,
                    "path": path,
                    "result": result,
                    "detail": detail,
                })

    if not pipe_ops:
        print("[-] No MT5 pipe operations found in CSV")
        print("    Make sure to filter on pipe operations and run the sniffer")
        return

    print(f"[+] Found {len(pipe_ops)} pipe operations\n")

    pipe_names = set()
    for op in pipe_ops:
        pipe_names.add(op["path"])

    print("Pipe names:")
    for name in sorted(pipe_names):
        count = sum(1 for o in pipe_ops if o["path"] == name)
        print(f"  {name} ({count} operations)")

    print("\nOperations timeline:")
    for op in pipe_ops:
        detail_short = op["detail"][:80] if op["detail"] else ""
        print(f"  {op['time']} [{op['process']}:{op['pid']}] "
              f"{op['operation']} {op['result']}")
        if detail_short:
            print(f"    {detail_short}")

        if "Length:" in op["detail"]:
            for part in op["detail"].split(","):
                part = part.strip()
                if part.startswith("Length:"):
                    length = part.split(":")[1].strip()
                    print(f"    -> Data length: {length}")

    out_file = OUTPUT_DIR / "procmon_analysis.json"
    with open(out_file, "w") as f:
        json.dump(pipe_ops, f, indent=2)
    print(f"\nAnalysis saved to {out_file}")


def generate_procmon_filter():
    """Generate a Process Monitor filter configuration."""
    print("[*] Process Monitor Filter Configuration")
    print("=" * 50)
    print()
    print("Manual setup in Procmon (Filter > Filter...):")
    print()
    print("  1. Process Name  is  python.exe     then Include")
    print("  2. Process Name  is  terminal64.exe  then Include")
    print("  3. Path          contains  pipe      then Include")
    print("  4. Operation     is  CreateFile       then Include")
    print("  5. Operation     is  ReadFile         then Include")
    print("  6. Operation     is  WriteFile        then Include")
    print("  7. Operation     is  CloseFile        then Include")
    print()
    print("Steps:")
    print("  1. Start Procmon with these filters")
    print("  2. Clear current capture (Ctrl+X)")
    print("  3. In another terminal, run: python pipe_sniffer.py")
    print("  4. Wait for the sniffer to complete")
    print("  5. In Procmon: File > Save as CSV")
    print("  6. Run: python procmon_capture.py --analyze <file.csv>")
    print()
    print("Alternative: Use API Monitor")
    print("  1. Download: http://www.rohitab.com/apimonitor")
    print("  2. Select API Filter > System Services > Pipes")
    print("  3. Also select: Kernel32.dll > File Management")
    print("  4. Monitor python.exe")
    print("  5. Run pipe_sniffer.py")
    print("  6. Export the captured data")


def analyze_raw_binary(bin_dir):
    """Analyze raw binary captures from the pipe sniffer."""
    bin_path = Path(bin_dir)
    if not bin_path.exists():
        print(f"[-] Directory not found: {bin_dir}")
        return

    files = sorted(bin_path.glob("*.bin"))
    if not files:
        print(f"[-] No .bin files in {bin_dir}")
        return

    print(f"[*] Analyzing {len(files)} binary captures from {bin_dir}\n")

    messages = []
    for f in files:
        data = f.read_bytes()
        name = f.stem
        parts = name.split("_")
        seq_num = int(parts[0]) if parts[0].isdigit() else 0
        direction = parts[1] if len(parts) > 1 else "UNKNOWN"

        msg = {
            "file": f.name,
            "seq": seq_num,
            "direction": direction,
            "size": len(data),
        }

        if len(data) >= 4:
            msg["header_le32"] = struct.unpack_from("<I", data, 0)[0]
            msg["header_be32"] = struct.unpack_from(">I", data, 0)[0]
            msg["header_le16"] = struct.unpack_from("<H", data, 0)[0]

            if msg["header_le32"] == len(data) - 4:
                msg["framing"] = "le32_length_prefix"
                msg["payload_offset"] = 4
                msg["payload_size"] = msg["header_le32"]
            elif msg["header_be32"] == len(data) - 4:
                msg["framing"] = "be32_length_prefix"
                msg["payload_offset"] = 4
                msg["payload_size"] = msg["header_be32"]
            elif msg["header_le16"] == len(data) - 2:
                msg["framing"] = "le16_length_prefix"
                msg["payload_offset"] = 2
                msg["payload_size"] = msg["header_le16"]

        if len(data) >= 8:
            msg["field_4_le32"] = struct.unpack_from("<I", data, 4)[0]
            msg["field_4_le16"] = struct.unpack_from("<H", data, 4)[0]

        if len(data) >= 12:
            msg["field_8_le32"] = struct.unpack_from("<I", data, 8)[0]

        if len(data) >= 16:
            msg["field_12_le32"] = struct.unpack_from("<I", data, 12)[0]

        messages.append(msg)

        print(f"{direction:>5s} [{seq_num:04d}] {len(data):>6d} bytes  "
              f"header: {data[:4].hex() if len(data) >= 4 else 'N/A'}")

        for i in range(0, min(len(data), 64), 16):
            chunk = data[i:i + 16]
            h = " ".join(f"{b:02x}" for b in chunk)
            a = "".join(chr(b) if 32 <= b < 127 else "." for b in chunk)
            print(f"       {i:04x}: {h:<48s}  {a}")
        print()

    print("\n" + "=" * 50)
    print("FRAMING ANALYSIS")
    print("=" * 50)

    framings = {}
    for m in messages:
        f = m.get("framing", "unknown")
        framings[f] = framings.get(f, 0) + 1

    for f, count in framings.items():
        print(f"  {f}: {count} messages")

    writes = [m for m in messages if m["direction"] == "WRITE"]
    reads = [m for m in messages if m["direction"] == "READ"]
    print(f"\n  Total writes: {len(writes)}, sizes: {[m['size'] for m in writes]}")
    print(f"  Total reads:  {len(reads)}, sizes: {[m['size'] for m in reads]}")

    if writes and reads:
        print("\n  Request/Response pairs:")
        for w, r in zip(writes, reads):
            print(f"    WRITE {w['size']}B -> READ {r['size']}B")

    out_file = OUTPUT_DIR / "binary_analysis.json"
    with open(out_file, "w") as f:
        json.dump(messages, f, indent=2)
    print(f"\nDetailed analysis saved to {out_file}")


if __name__ == "__main__":
    if len(sys.argv) < 2:
        generate_procmon_filter()
        print("\nUsage:")
        print(f"  {sys.argv[0]} --filter          Show Procmon filter setup")
        print(f"  {sys.argv[0]} --analyze <csv>   Analyze Procmon CSV export")
        print(f"  {sys.argv[0]} --binary <dir>    Analyze raw binary captures")
        sys.exit(0)

    if sys.argv[1] == "--filter":
        generate_procmon_filter()
    elif sys.argv[1] == "--analyze" and len(sys.argv) > 2:
        analyze_procmon_csv(sys.argv[2])
    elif sys.argv[1] == "--binary" and len(sys.argv) > 2:
        analyze_raw_binary(sys.argv[2])
    else:
        print(f"Unknown option: {sys.argv[1]}")
