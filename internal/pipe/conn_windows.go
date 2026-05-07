//go:build windows

package pipe

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var procWaitNamedPipe = windows.NewLazySystemDLL("kernel32.dll").NewProc("WaitNamedPipeW")

type Conn struct {
	handle windows.Handle
	name   string
}

func Open(name string, timeout time.Duration) (*Conn, error) {
	pipePath, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, fmt.Errorf("invalid pipe name: %w", err)
	}

	deadline := time.Now().Add(timeout)
	for {
		handle, err := windows.CreateFile(
			pipePath,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			0,
			nil,
			windows.OPEN_EXISTING,
			0,
			0,
		)
		if err == nil {
			return &Conn{handle: handle, name: name}, nil
		}

		if !isErrPipeBusy(err) {
			return nil, fmt.Errorf("open pipe %s: %w", name, err)
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("pipe %s: timeout after %v", name, timeout)
		}

		waitMs := uint32(time.Until(deadline).Milliseconds())
		if waitMs > 5000 {
			waitMs = 5000
		}
		procWaitNamedPipe.Call(uintptr(unsafe.Pointer(pipePath)), uintptr(waitMs))
	}
}

func isErrPipeBusy(err error) bool {
	if errno, ok := err.(syscall.Errno); ok {
		return errno == 231 // ERROR_PIPE_BUSY
	}
	return false
}

func (c *Conn) Read(p []byte) (int, error) {
	var n uint32
	err := windows.ReadFile(c.handle, p, &n, nil)
	if err != nil {
		if err == windows.ERROR_BROKEN_PIPE {
			return 0, io.EOF
		}
		return int(n), fmt.Errorf("read pipe: %w", err)
	}
	return int(n), nil
}

func (c *Conn) Write(p []byte) (int, error) {
	var n uint32
	err := windows.WriteFile(c.handle, p, &n, nil)
	if err != nil {
		return int(n), fmt.Errorf("write pipe: %w", err)
	}
	return int(n), nil
}

func (c *Conn) Close() error {
	return windows.CloseHandle(c.handle)
}

func Discover() (string, error) {
	paths, err := findTerminal64Paths()
	if err != nil {
		return "", fmt.Errorf("locate terminal64.exe: %w", err)
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("no running terminal64.exe found")
	}

	for _, p := range paths {
		pipeName := mt5PipeNameForPath(p)
		conn, err := Open(pipeName, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return pipeName, nil
		}
	}

	return "", fmt.Errorf("no responding MT5 pipe (%d candidate(s))", len(paths))
}

func mt5PipeNameForPath(terminalPath string) string {
	input := `\\?\` + strings.ToLower(terminalPath)
	codes := utf16.Encode([]rune(input))
	buf := make([]byte, len(codes)*2)
	for i, c := range codes {
		buf[i*2] = byte(c)
		buf[i*2+1] = byte(c >> 8)
	}
	sum := sha256.Sum256(buf)
	return `\\.\pipe\MT5.Terminal.` + strings.ToUpper(hex.EncodeToString(sum[:]))
}

func findTerminal64Paths() ([]string, error) {
	const TH32CS_SNAPPROCESS = 0x2
	snap, err := windows.CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot: %w", err)
	}
	defer windows.CloseHandle(snap)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		return nil, fmt.Errorf("Process32First: %w", err)
	}

	var paths []string
	seen := make(map[string]bool)
	for {
		exeName := windows.UTF16ToString(pe.ExeFile[:])
		if strings.EqualFold(exeName, "terminal64.exe") {
			if p, err := imagePathForPID(pe.ProcessID); err == nil && p != "" && !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
		if err := windows.Process32Next(snap, &pe); err != nil {
			break
		}
	}
	return paths, nil
}

func imagePathForPID(pid uint32) (string, error) {
	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	h, err := windows.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:size]), nil
}

func FindTerminalPath() (string, error) {
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\MetaTrader 5`,
		registry.READ|registry.WOW64_64KEY,
	)
	if err != nil {
		key, err = registry.OpenKey(
			registry.CURRENT_USER,
			`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\MetaTrader 5`,
			registry.READ,
		)
		if err != nil {
			return "", fmt.Errorf("MT5 not found in registry: %w", err)
		}
	}
	defer key.Close()

	val, _, err := key.GetStringValue("InstallLocation")
	if err != nil {
		return "", fmt.Errorf("read InstallLocation: %w", err)
	}

	termPath := filepath.Join(val, "terminal64.exe")
	if _, err := os.Stat(termPath); err != nil {
		return "", fmt.Errorf("terminal not found at %s: %w", termPath, err)
	}

	return termPath, nil
}
