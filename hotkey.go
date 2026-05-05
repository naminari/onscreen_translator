package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	MOD_ALT     = 0x0001
	MOD_CONTROL = 0x0002
	MOD_SHIFT   = 0x0004
	MOD_WIN     = 0x0008
)

var (
	hotkeyHwnd   uintptr
	hotkeyID     = 1
	hotkeyActive bool
)

var (
	user32hotkey            = syscall.NewLazyDLL("user32.dll")
	procRegisterHotKey      = user32hotkey.NewProc("RegisterHotKey")
	procUnregisterHotKey    = user32hotkey.NewProc("UnregisterHotKey")
	procCreateWindowExW_hk  = user32hotkey.NewProc("CreateWindowExW")
	procRegisterClassExW_hk = user32hotkey.NewProc("RegisterClassExW")
	procDefWindowProcW_hk   = user32hotkey.NewProc("DefWindowProcW")
)

func getModuleHandleHotkey() uintptr {
	ret, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	return ret
}

// CreateHotkeyWindow создаёт скрытое окно для приёма WM_HOTKEY
func CreateHotkeyWindow() uintptr {
	className := syscall.StringToUTF16Ptr("HotkeyMessageWindow")
	type WNDCLASSEX struct {
		cbSize        uint32
		style         uint32
		lpfnWndProc   uintptr
		cbClsExtra    int32
		cbWndExtra    int32
		hInstance     uintptr
		hIcon         uintptr
		hCursor       uintptr
		hbrBackground uintptr
		lpszMenuName  *uint16
		lpszClassName *uint16
		hIconSm       uintptr
	}
	wc := WNDCLASSEX{
		cbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		lpfnWndProc:   syscall.NewCallback(hotkeyWndProc),
		hInstance:     getModuleHandleHotkey(),
		lpszClassName: className,
	}
	procRegisterClassExW_hk.Call(uintptr(unsafe.Pointer(&wc)))
	hwnd, _, _ := procCreateWindowExW_hk.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0,
		0,
		0, 0, 0, 0,
		0, 0, 0, 0,
	)
	return hwnd
}

// hotkeyWndProc обрабатывает WM_HOTKEY
func hotkeyWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	const WM_HOTKEY = 0x0312
	if msg == WM_HOTKEY {
		if wParam == uintptr(hotkeyID) {
			onHotkey()
			return 0
		}
	}
	ret, _, _ := procDefWindowProcW_hk.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

// RegisterHotkey регистрирует глобальную горячую клавишу
func RegisterHotkey(modifiers, vk uint32) error {
	if hotkeyActive {
		UnregisterHotkey()
	}
	ret, _, _ := procRegisterHotKey.Call(hotkeyHwnd, uintptr(hotkeyID), uintptr(modifiers), uintptr(vk))
	if ret == 0 {
		return fmt.Errorf("RegisterHotKey failed (комбинация уже используется)")
	}
	hotkeyActive = true
	return nil
}

// UnregisterHotkey отменяет регистрацию
func UnregisterHotkey() {
	if !hotkeyActive {
		return
	}
	procUnregisterHotKey.Call(hotkeyHwnd, uintptr(hotkeyID))
	hotkeyActive = false
}
