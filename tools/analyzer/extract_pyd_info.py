"""
MetaTrader5.pyd Static Analyzer
Extracts structural information from the .pyd binary without Ghidra.
Uses pefile and capstone for lightweight PE analysis and disassembly.

Usage:
    pip install pefile capstone
    python extract_pyd_info.py [path_to_MetaTrader5.pyd]
"""

import sys
import struct
import json
from pathlib import Path
from datetime import datetime, timezone

try:
    import pefile
except ImportError:
    print("pip install pefile")
    sys.exit(1)

try:
    from capstone import Cs, CS_ARCH_X86, CS_MODE_64
    HAS_CAPSTONE = True
except ImportError:
    HAS_CAPSTONE = False
    print("[!] capstone not installed, disassembly disabled")
    print("    pip install capstone")

OUTPUT_DIR = Path("mt5_pyd_analysis")
OUTPUT_DIR.mkdir(exist_ok=True)


def find_pyd():
    if len(sys.argv) > 1:
        return Path(sys.argv[1])

    try:
        import MetaTrader5
        return Path(MetaTrader5.__file__)
    except ImportError:
        pass

    candidates = [
        Path.home() / "AppData/Local/Programs/Python/Python312/Lib/site-packages/MetaTrader5/MetaTrader5.pyd",
        Path.home() / "AppData/Local/Programs/Python/Python311/Lib/site-packages/MetaTrader5/MetaTrader5.pyd",
        Path.home() / "AppData/Local/Programs/Python/Python310/Lib/site-packages/MetaTrader5/MetaTrader5.pyd",
    ]
    for c in candidates:
        if c.exists():
            return c

    print("ERROR: Cannot find MetaTrader5.pyd")
    print("Usage: python extract_pyd_info.py <path_to_MetaTrader5.pyd>")
    sys.exit(1)


def analyze_pe(pyd_path):
    print(f"[*] Analyzing: {pyd_path}")
    print(f"[*] Size: {pyd_path.stat().st_size} bytes")

    pe = pefile.PE(str(pyd_path))

    info = {
        "file": str(pyd_path),
        "size": pyd_path.stat().st_size,
        "machine": hex(pe.FILE_HEADER.Machine),
        "is_64bit": pe.FILE_HEADER.Machine == 0x8664,
        "timestamp": pe.FILE_HEADER.TimeDateStamp,
        "compile_date": datetime.fromtimestamp(
            pe.FILE_HEADER.TimeDateStamp, timezone.utc
        ).isoformat(),
        "sections": [],
        "imports": {},
        "exports": [],
        "strings": [],
    }

    print(f"\n[*] Compile date: {info['compile_date']}")
    print(f"[*] Architecture: {'x86-64' if info['is_64bit'] else 'x86'}")

    print(f"\n{'='*60}")
    print("SECTIONS")
    print(f"{'='*60}")
    for section in pe.sections:
        name = section.Name.decode("utf-8", errors="replace").rstrip("\x00")
        s = {
            "name": name,
            "virtual_size": section.Misc_VirtualSize,
            "virtual_address": hex(section.VirtualAddress),
            "raw_size": section.SizeOfRawData,
            "characteristics": hex(section.Characteristics),
        }
        info["sections"].append(s)
        print(f"  {name:<10s} VA:{s['virtual_address']}  "
              f"VSize:{s['virtual_size']:>8d}  "
              f"RawSize:{s['raw_size']:>8d}  "
              f"Chars:{s['characteristics']}")

    print(f"\n{'='*60}")
    print("IMPORTS (DLL dependencies)")
    print(f"{'='*60}")

    kernel32_funcs = []
    if hasattr(pe, "DIRECTORY_ENTRY_IMPORT"):
        for entry in pe.DIRECTORY_ENTRY_IMPORT:
            dll_name = entry.dll.decode("utf-8", errors="replace")
            funcs = []
            for imp in entry.imports:
                func_name = imp.name.decode("utf-8", errors="replace") if imp.name else f"ordinal_{imp.ordinal}"
                funcs.append({
                    "name": func_name,
                    "address": hex(imp.address) if imp.address else None,
                    "thunk_rva": hex(imp.struct_table.AddressOfData) if hasattr(imp, 'struct_table') else None,
                })
                if dll_name.lower() == "kernel32.dll":
                    kernel32_funcs.append(func_name)

            info["imports"][dll_name] = funcs
            print(f"\n  {dll_name} ({len(funcs)} functions):")
            for f in funcs:
                marker = ""
                if f["name"] in ("CreateFileW", "ReadFile", "WriteFile",
                                  "CloseHandle", "WaitNamedPipeW",
                                  "CreateProcessW", "CreateNamedPipeW"):
                    marker = "  <-- PIPE/PROCESS"
                print(f"    {f['name']}{marker}")

    print(f"\n{'='*60}")
    print("EXPORTS")
    print(f"{'='*60}")
    if hasattr(pe, "DIRECTORY_ENTRY_EXPORT"):
        for exp in pe.DIRECTORY_ENTRY_EXPORT.symbols:
            name = exp.name.decode("utf-8", errors="replace") if exp.name else f"ordinal_{exp.ordinal}"
            e = {
                "name": name,
                "ordinal": exp.ordinal,
                "address": hex(pe.OPTIONAL_HEADER.ImageBase + exp.address),
                "rva": hex(exp.address),
            }
            info["exports"].append(e)
            print(f"  {name} @ RVA {e['rva']}")

    print(f"\n{'='*60}")
    print("STRINGS (pipe-related)")
    print(f"{'='*60}")

    raw_data = pyd_path.read_bytes()

    ascii_strings = extract_strings(raw_data, min_len=4, encoding="ascii")
    unicode_strings = extract_strings(raw_data, min_len=4, encoding="utf-16-le")

    pipe_keywords = ["pipe", "mt5", "meta", "terminal", "trade", "connect",
                     "init", "version", "account", "symbol", "order",
                     "position", "history", "copy", "rate", "tick",
                     "login", "password", "server", "timeout"]

    interesting = []
    for offset, s, enc in ascii_strings + unicode_strings:
        s_lower = s.lower()
        if any(k in s_lower for k in pipe_keywords):
            interesting.append((offset, s, enc))
            info["strings"].append({"offset": hex(offset), "value": s, "encoding": enc})

    interesting.sort(key=lambda x: x[0])
    for offset, s, enc in interesting:
        print(f"  [{enc:>7s}] 0x{offset:06x}: {s}")

    print(f"\n{'='*60}")
    print("ALL STRINGS (> 6 chars)")
    print(f"{'='*60}")
    all_strings = []
    for offset, s, enc in ascii_strings + unicode_strings:
        if len(s) >= 6 and not s.startswith("__") and s.isprintable():
            all_strings.append((offset, s, enc))
            info["strings"].append({"offset": hex(offset), "value": s, "encoding": enc})

    all_strings.sort(key=lambda x: x[0])
    for offset, s, enc in all_strings[:200]:
        print(f"  [{enc:>7s}] 0x{offset:06x}: {s}")
    if len(all_strings) > 200:
        print(f"  ... +{len(all_strings) - 200} more")

    if HAS_CAPSTONE and kernel32_funcs:
        print(f"\n{'='*60}")
        print("DISASSEMBLY: CreateFileW call sites")
        print(f"{'='*60}")
        find_api_calls(pe, raw_data, "CreateFileW")

        print(f"\n{'='*60}")
        print("DISASSEMBLY: WriteFile call sites")
        print(f"{'='*60}")
        find_api_calls(pe, raw_data, "WriteFile")

        print(f"\n{'='*60}")
        print("DISASSEMBLY: ReadFile call sites")
        print(f"{'='*60}")
        find_api_calls(pe, raw_data, "ReadFile")

    out_file = OUTPUT_DIR / "pyd_analysis.json"
    with open(out_file, "w") as f:
        json.dump(info, f, indent=2)
    print(f"\nFull analysis saved to {out_file}")

    pe.close()
    return info


def extract_strings(data, min_len=4, encoding="ascii"):
    results = []
    if encoding == "ascii":
        current = []
        start = 0
        for i, b in enumerate(data):
            if 32 <= b < 127:
                if not current:
                    start = i
                current.append(chr(b))
            else:
                if len(current) >= min_len:
                    results.append((start, "".join(current), "ascii"))
                current = []
        if len(current) >= min_len:
            results.append((start, "".join(current), "ascii"))

    elif encoding == "utf-16-le":
        i = 0
        while i < len(data) - 1:
            current = []
            start = i
            while i < len(data) - 1:
                c = struct.unpack_from("<H", data, i)[0]
                if 32 <= c < 127:
                    current.append(chr(c))
                    i += 2
                else:
                    break
            if len(current) >= min_len:
                results.append((start, "".join(current), "utf-16"))
            i += 2

    return results


def find_api_calls(pe, raw_data, api_name):
    if not HAS_CAPSTONE:
        return

    iat_rva = None
    if hasattr(pe, "DIRECTORY_ENTRY_IMPORT"):
        for entry in pe.DIRECTORY_ENTRY_IMPORT:
            for imp in entry.imports:
                if imp.name and imp.name.decode("utf-8", errors="replace") == api_name:
                    iat_rva = imp.address - pe.OPTIONAL_HEADER.ImageBase
                    break

    if iat_rva is None:
        print(f"  {api_name} not found in IAT")
        return

    print(f"  {api_name} IAT RVA: {hex(iat_rva)}")

    text_section = None
    for section in pe.sections:
        if b".text" in section.Name:
            text_section = section
            break

    if not text_section:
        print("  .text section not found")
        return

    text_start = text_section.PointerToRawData
    text_size = text_section.SizeOfRawData
    text_va = text_section.VirtualAddress
    text_data = raw_data[text_start:text_start + text_size]

    md = Cs(CS_ARCH_X86, CS_MODE_64)
    md.detail = True

    call_sites = []
    for i in md.disasm(text_data, text_va):
        if i.mnemonic == "call" and len(i.operands) > 0:
            op = i.operands[0]
            if op.type == 3:
                target_rva = i.address + i.size + op.mem.disp
                if abs(target_rva - iat_rva) < 8:
                    call_sites.append(i.address)

    print(f"  Found {len(call_sites)} call sites")

    for site in call_sites:
        print(f"\n  Call at RVA {hex(site)}:")
        context_start = site - 64
        if context_start < text_va:
            context_start = text_va
        offset = context_start - text_va
        context_data = text_data[offset:offset + 128]

        for inst in md.disasm(context_data, context_start):
            marker = " <<<" if inst.address == site else ""
            print(f"    {hex(inst.address)}: {inst.mnemonic:<10s} {inst.op_str}{marker}")
            if inst.address > site + 16:
                break


if __name__ == "__main__":
    pyd_path = find_pyd()
    analyze_pe(pyd_path)
