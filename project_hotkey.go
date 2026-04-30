package main

// import (
// 	"fmt"
// 	"syscall"
// 	"unsafe"
// )

// const (
// 	HOTKEY_ID = 1
// 	WM_HOTKEY = 0x0312
// )

// var (
// 	procRegisterHotKey   = user32.NewProc("RegisterHotKey")
// 	procUnregisterHotKey = user32.NewProc("UnregisterHotKey")
// 	procCreateWindowEx   = user32.NewProc("CreateWindowExW")
// 	procDefWindowProc    = user32.NewProc("DefWindowProcW")
// 	procGetMessage       = user32.NewProc("GetMessageW")
// 	procTranslateMessage = user32.NewProc("TranslateMessage")
// 	procDispatchMessage  = user32.NewProc("DispatchMessageW")
// )

// var hotkeyHwnd uintptr

// func init() {
// 	// Регистрируем класс окна (простой статический класс, но можно и свой)
// 	// Используем существующий класс "STATIC" для простоты
// 	hwnd, _, _ := procCreateWindowEx.Call(
// 		0, // dwExStyle
// 		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("STATIC"))),
// 		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("HotkeyWindow"))),
// 		0, // dwStyle: невидимое
// 		0, 0, 0, 0,
// 		0, // родитель (NULL)
// 		0, // меню
// 		0, // hInstance
// 		0, // lpParam
// 	)
// 	hotkeyHwnd = hwnd
// 	if hotkeyHwnd == 0 {
// 		fmt.Println("[Hotkey] Failed to create hidden window")
// 	}
// }

// func startHotkeyListener() {
// 	// Регистрируем Ctrl+Shift+D
// 	ret, _, _ := procRegisterHotKey.Call(
// 		hotkeyHwnd,
// 		HOTKEY_ID,
// 		0x0002|0x0004, // MOD_CONTROL | MOD_SHIFT
// 		0x44,          // 'D'
// 	)
// 	if ret == 0 {
// 		fmt.Println("[Hotkey] RegisterHotKey failed")
// 		return
// 	}
// 	fmt.Println("[Hotkey] Registered Ctrl+Shift+D")

// 	// Цикл сообщений (блокирует выполнение)
// 	var msg struct {
// 		hwnd    uintptr
// 		message uint32
// 		wParam  uintptr
// 		lParam  uintptr
// 		time    uint32
// 		pt      struct{ x, y int32 }
// 	}
// 	for {
// 		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
// 		if ret == 0 {
// 			break
// 		}
// 		if msg.message == WM_HOTKEY && msg.wParam == HOTKEY_ID {
// 			fmt.Println("[Hotkey] Activation pressed")
// 			if !captureModeActive {
// 				activateCaptureMode()
// 			}
// 		}
// 		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
// 		procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
// 	}
// }
