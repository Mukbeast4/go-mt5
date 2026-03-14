# Reverse Engineering MetaTrader5.pyd with Ghidra

## Setup

### 1. Install Ghidra
- Download: https://ghidra-sre.org
- Requires JDK 17+

### 2. Locate MetaTrader5.pyd
```powershell
# Find the .pyd file
python -c "import MetaTrader5; print(MetaTrader5.__file__)"
# Typically: C:\Users\<user>\AppData\Local\Programs\Python\Python3x\Lib\site-packages\MetaTrader5\MetaTrader5.pyd

# Copy it for analysis
copy "C:\...\MetaTrader5.pyd" C:\analysis\MetaTrader5.pyd
```

### 3. File info
```powershell
# Check file size and type
dir MetaTrader5.pyd
# Should be ~48-69 KB, x86-64 DLL
```

## Ghidra Analysis

### Import
1. File > New Project > Non-Shared > name "MT5_RE"
2. File > Import File > select `MetaTrader5.pyd`
3. Format: PE (Windows Portable Executable)
4. Language: x86:LE:64:default (AMD64)
5. Click "Analyze" when prompted, enable all analyzers

### Key Functions to Find

#### Exported Functions (Python Module Interface)
The .pyd exports `PyInit_MetaTrader5` (Python module init).
Go to Symbol Tree > Exports to see all exported symbols.

#### Named Pipe Operations
Search for imported functions from kernel32.dll:

1. **Window > Symbol Table** > filter by "kernel32"
2. Look for these imports:
   - `CreateFileW` - opens the named pipe
   - `WaitNamedPipeW` - waits for pipe availability
   - `ReadFile` - reads from pipe
   - `WriteFile` - writes to pipe
   - `CloseHandle` - closes pipe handle
   - `CreateProcessW` - launches terminal64.exe

#### Finding the Pipe Name
1. **Search > For Strings** (minimum length 4)
2. Filter for: "pipe", "MT5", "Meta", "Terminal"
3. The pipe name is a wide string (UTF-16LE), search both ASCII and Unicode
4. Double-click any match, then right-click > References > Show References to Address

Alternative: find `CreateFileW` import, then find all XREF (cross-references).
Each call to CreateFileW has the pipe name as first argument.

```
; Pseudo-code pattern to find:
lea  rcx, [pipe_name_string]    ; lpFileName = "\\.\pipe\MT5.Terminal..."
mov  edx, 0C0000000h            ; dwDesiredAccess = GENERIC_READ | GENERIC_WRITE
xor  r8d, r8d                   ; dwShareMode = 0
xor  r9d, r9d                   ; lpSecurityAttributes = NULL
mov  [rsp+20h], 3               ; dwCreationDisposition = OPEN_EXISTING
mov  [rsp+28h], 0               ; dwFlagsAndAttributes = 0
mov  [rsp+30h], 0               ; hTemplateFile = NULL
call CreateFileW
```

### Finding the Message Format

#### Write operations (requests)
1. Find all XREFs to `WriteFile`
2. For each call, trace back the buffer (`lpBuffer` = 2nd arg = RDX)
3. The buffer construction reveals the message format:
   - Look for `memcpy`, struct assignments, integer stores
   - Length prefix: look for a 4-byte write before the payload
   - Command ID: look for a small integer or enum value

```
; Pattern for length-prefixed write:
mov  [buffer+0], payload_length   ; 4-byte length prefix
mov  [buffer+4], command_id       ; command type
mov  [buffer+8], ...              ; parameters
; then:
lea  rdx, [buffer]               ; lpBuffer
mov  r8d, total_length            ; nNumberOfBytesToWrite
call WriteFile
```

#### Read operations (responses)
1. Find all XREFs to `ReadFile`
2. After ReadFile, trace how the buffer is parsed
3. Look for struct field extraction, comparisons, conditionals

### Finding Command IDs

Each Python API function (version, account_info, etc.) sends a different
command ID over the pipe. To map them:

1. Find `PyInit_MetaTrader5` (the module init function)
2. It registers a `PyMethodDef` table - an array of {name, function_ptr, flags, doc}
3. Each function_ptr is the C function implementing that Python method
4. Follow each function_ptr to see what command ID it writes to the pipe

```
; PyMethodDef table pattern:
dq offset "version"          ; method name
dq offset mt5_version_impl   ; C function
dd METH_NOARGS               ; flags (4 = METH_NOARGS)
dd 0
dq offset "version doc..."   ; docstring
```

### Data Structure Reconstruction

For each response parser, Ghidra can help reconstruct the struct layout:

1. Find where ReadFile result is parsed
2. Create a struct in Ghidra (Data Type Manager > right-click > New Structure)
3. Map the offsets to field names based on what the Python wrapper returns

Example: if account_info returns a namedtuple with fields (login, trade_mode, leverage, ...),
the binary response likely has these in order as fixed-size fields.

### Handshake / Init Sequence

Find the function called during `mt5.initialize()`:
1. In PyMethodDef table, find "initialize" entry
2. Follow the function pointer
3. The init function likely:
   a. Reads registry for terminal path
   b. Calls CreateProcessW if needed
   c. Calls WaitNamedPipeW
   d. Calls CreateFileW to open the pipe
   e. Sends an init/handshake message
   f. Reads version response
   g. Validates compatibility

### Tips

- Use "Decompile" view (Window > Decompile) for C-like pseudocode
- Right-click functions > Edit Function Signature to name parameters
- Create bookmarks (Ctrl+D) on important locations
- The .pyd is small (~50KB code), so there are probably <50 functions total
- Focus on functions that call WriteFile/ReadFile - these ARE the protocol
- Export your analysis: File > Export Program > export as C headers
