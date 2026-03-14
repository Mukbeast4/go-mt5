//go:build windows

package pipe

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
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
	pattern := `\\.\pipe\*`
	pipePath, err := windows.UTF16PtrFromString(pattern)
	if err != nil {
		return "", err
	}

	var fd windows.Win32finddata
	h, err := windows.FindFirstFile(pipePath, &fd)
	if err != nil {
		return "", fmt.Errorf("enumerate pipes: %w", err)
	}
	defer windows.FindClose(h)

	for {
		name := windows.UTF16ToString(fd.FileName[:])
		nameLower := strings.ToLower(name)
		if strings.Contains(nameLower, "mt5") || strings.Contains(nameLower, "metatrader") {
			return `\\.\pipe\` + name, nil
		}

		err = windows.FindNextFile(h, &fd)
		if err != nil {
			break
		}
	}

	return "", fmt.Errorf("no MT5 pipe found")
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
