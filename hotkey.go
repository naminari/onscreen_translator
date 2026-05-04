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
	hotkeyID     int32 = 1
	hotkeyActive bool

	user32hotkey         = syscall.NewLazyDLL("user32.dll")
	procRegisterHotKey   = user32hotkey.NewProc("RegisterHotKey")
	procUnregisterHotKey = user32hotkey.NewProc("UnregisterHotKey")
	procGetModuleHandleW = syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW")
)

func getModuleHandleHotkey() uintptr {
	ret, _, _ := procGetModuleHandleW.Call(0)
	return ret
}

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
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0,
		0,
		0, 0, 0, 0,
		0, 0, 0, 0,
	)
	return hwnd
}

func hotkeyWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == 0x0312 { // WM_HOTKEY
		if wParam == uintptr(hotkeyID) {
			onHotkey()
			return 0
		}
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func RegisterHotkey(modifiers uint32, vk uint32) error {
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

func UnregisterHotkey() {
	if !hotkeyActive {
		return
	}
	procUnregisterHotKey.Call(hotkeyHwnd, uintptr(hotkeyID))
	hotkeyActive = false
}
