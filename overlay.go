package main

import (
	"fmt"
	"image"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unicode"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procSetProcessDPIAware         = user32.NewProc("SetProcessDPIAware")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procDestroyWindow              = user32.NewProc("DestroyWindow")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procUpdateWindow               = user32.NewProc("UpdateWindow")
	procGetClientRect              = user32.NewProc("GetClientRect")
	procRegisterClassExW           = user32.NewProc("RegisterClassExW")
	procDefWindowProcW             = user32.NewProc("DefWindowProcW")
	procBeginPaint                 = user32.NewProc("BeginPaint")
	procEndPaint                   = user32.NewProc("EndPaint")
	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	procFillRect                   = user32.NewProc("FillRect")
	procPostQuitMessage            = user32.NewProc("PostQuitMessage")
	procSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	procSetFocus                   = user32.NewProc("SetFocus")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procSetCapture                 = user32.NewProc("SetCapture")
	procReleaseCapture             = user32.NewProc("ReleaseCapture")
	procIsWindow                   = user32.NewProc("IsWindow")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procGetDC                      = user32.NewProc("GetDC")
	procReleaseDC                  = user32.NewProc("ReleaseDC")
	procInvertRect                 = user32.NewProc("InvertRect")
	procGetMessageW                = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessageW           = user32.NewProc("DispatchMessageW")
	procSendMessageW               = user32.NewProc("SendMessageW")
	procGetDlgItem                 = user32.NewProc("GetDlgItem")
	procEnableWindow               = user32.NewProc("EnableWindow")
	procSetWindowTextW             = user32.NewProc("SetWindowTextW")
	procPostMessageW               = user32.NewProc("PostMessageW")
	procGetWindowLongPtrW          = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW          = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW            = user32.NewProc("CallWindowProcW")

	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
)

const (
	WS_EX_LAYERED = 0x80000
	WS_EX_TOPMOST = 0x8
	WS_POPUP      = 0x80000000
	WS_VISIBLE    = 0x10000000
	LWA_ALPHA     = 0x2

	SM_XVIRTUALSCREEN  = 76
	SM_YVIRTUALSCREEN  = 77
	SM_CXVIRTUALSCREEN = 78
	SM_CYVIRTUALSCREEN = 79

	HWND_TOPMOST = ^uintptr(0)
	SWP_NOSIZE   = 0x0001
	SWP_NOMOVE   = 0x0002

	WM_LBUTTONDOWN   = 0x0201
	WM_MOUSEMOVE     = 0x0200
	WM_LBUTTONUP     = 0x0202
	WM_PAINT         = 0x000F
	WM_ERASEBKGND    = 0x0014
	WM_NCHITTEST     = 0x0084
	WM_MOUSEACTIVATE = 0x0021

	// Стили для окна результата (EDIT)
	ES_MULTILINE     = 0x0004
	ES_READONLY      = 0x0800
	ES_AUTOVSCROLL   = 0x0040
	WS_VSCROLL       = 0x00200000
	WS_HSCROLL       = 0x00100000
	WS_EX_CLIENTEDGE = 0x00000200

	// Сообщения для подкласса EDIT
	WM_LBUTTONDBLCLK = 0x0203
	WM_GETTEXT       = 0x000D
	WM_GETTEXTLENGTH = 0x000E
	EM_GETSEL        = 0x00B0

	// SetWindowPos flags
	SWP_NOZORDER = 0x0004
	WM_SIZE      = 0x0005

	// SetWindowLongPtr index для замены WndProc
	GWLP_WNDPROC = ^uintptr(3) // -4
)

var (
	overlayHwnd         uintptr
	startX, startY      int
	drawing             bool
	selectionRect       image.Rectangle
	captureModeActive   bool
	prevRect            image.Rectangle
	currentResultWindow uintptr = 0

	// окно результата: два EDIT-контрола
	hwndOrigEdit  uintptr
	hwndTransEdit uintptr

	// подкласс оригинального EDIT
	origEditProc            uintptr
	editSubclassCallbackPtr uintptr
	editSubclassOnce        sync.Once
	resultWindowClassOnce   sync.Once
)

var className = syscall.StringToUTF16Ptr("OverlayWindowClass")

func getModuleHandle() uintptr {
	ret, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	return ret
}

func init() {
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
		lpfnWndProc:   syscall.NewCallback(wndProc),
		hInstance:     getModuleHandle(),
		lpszClassName: className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
}

func invertRect(hdc uintptr, r image.Rectangle) {
	rect := struct {
		Left, Top, Right, Bottom int32
	}{
		Left:   int32(r.Min.X),
		Top:    int32(r.Min.Y),
		Right:  int32(r.Max.X),
		Bottom: int32(r.Max.Y),
	}
	procInvertRect.Call(hdc, uintptr(unsafe.Pointer(&rect)))
}

func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_NCHITTEST:
		return 1
	case WM_MOUSEACTIVATE:
		return 3
	case WM_LBUTTONDOWN:
		drawing = true
		startX = int(int16(lParam & 0xFFFF))
		startY = int(int16(lParam >> 16))
		prevRect = image.Rect(0, 0, 0, 0)
		return 0
	case WM_MOUSEMOVE:
		if drawing {
			x := int(int16(lParam & 0xFFFF))
			y := int(int16(lParam >> 16))
			newRect := image.Rect(startX, startY, x, y)
			normalized := normalizeRect(newRect)
			hdc, _, _ := procGetDC.Call(hwnd)
			if hdc != 0 {
				if prevRect.Dx() != 0 && prevRect.Dy() != 0 {
					invertRect(hdc, prevRect)
				}
				invertRect(hdc, normalized)
				procReleaseDC.Call(hwnd, hdc)
				prevRect = normalized
			}
		}
		return 0
	case WM_LBUTTONUP:
		if !drawing {
			return 0
		}
		drawing = false
		if prevRect.Dx() != 0 && prevRect.Dy() != 0 {
			hdc, _, _ := procGetDC.Call(hwnd)
			if hdc != 0 {
				invertRect(hdc, prevRect)
				procReleaseDC.Call(hwnd, hdc)
			}
			prevRect = image.Rect(0, 0, 0, 0)
		}
		offX, _, _ := procGetSystemMetrics.Call(SM_XVIRTUALSCREEN)
		offY, _, _ := procGetSystemMetrics.Call(SM_YVIRTUALSCREEN)
		rawX := int(int16(lParam & 0xFFFF))
		rawY := int(int16(lParam >> 16))
		endX := rawX + int(offX)
		endY := rawY + int(offY)
		absRect := image.Rect(
			startX+int(offX), startY+int(offY),
			endX, endY,
		)
		absRect = normalizeRect(absRect)
		if absRect.Dx() < 10 || absRect.Dy() < 10 {
			procDestroyWindow.Call(hwnd)
			return 0
		}
		selectionRect = absRect
		procDestroyWindow.Call(hwnd)
		go processSelection(selectionRect)
		return 0
	case WM_PAINT:
		var ps struct {
			hdc         uintptr
			fErase      int32
			rcPaint     struct{ left, top, right, bottom int32 }
			fRestore    int32
			fIncUpdate  int32
			rgbReserved [32]byte
		}
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		brush, _, _ := procCreateSolidBrush.Call(0x00000000)
		var rect struct{ left, top, right, bottom int32 }
		procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rect)), brush)
		procDeleteObject.Call(brush)
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_ERASEBKGND:
		return 1
	case WM_DESTROY:
		procReleaseCapture.Call()
		captureModeActive = false
		return 0
	default:
		ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return ret
	}
}

func startScreenOverlay() {
	vX, _, _ := procGetSystemMetrics.Call(SM_XVIRTUALSCREEN)
	vY, _, _ := procGetSystemMetrics.Call(SM_YVIRTUALSCREEN)
	vW, _, _ := procGetSystemMetrics.Call(SM_CXVIRTUALSCREEN)
	vH, _, _ := procGetSystemMetrics.Call(SM_CYVIRTUALSCREEN)

	hwnd, _, _ := procCreateWindowExW.Call(
		WS_EX_LAYERED|WS_EX_TOPMOST,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Overlay"))),
		WS_POPUP|WS_VISIBLE,
		vX, vY, vW, vH,
		0, 0, 0, 0,
	)
	if hwnd == 0 {
		return
	}
	overlayHwnd = hwnd
	procSetWindowPos.Call(hwnd, HWND_TOPMOST, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE)
	procSetLayeredWindowAttributes.Call(hwnd, 0, 180, LWA_ALPHA)
	procSetForegroundWindow.Call(hwnd)
	procSetFocus.Call(hwnd)
	procSetCapture.Call(hwnd)
	captureModeActive = true
}

func normalizeRect(r image.Rectangle) image.Rectangle {
	if r.Min.X > r.Max.X {
		r.Min.X, r.Max.X = r.Max.X, r.Min.X
	}
	if r.Min.Y > r.Max.Y {
		r.Min.Y, r.Max.Y = r.Max.Y, r.Min.Y
	}
	return r
}

func closeResultWindow() {
	if currentResultWindow != 0 {
		ret, _, _ := procIsWindow.Call(currentResultWindow)
		if ret != 0 {
			procDestroyWindow.Call(currentResultWindow)
		}
		currentResultWindow = 0
	}
}

// registerResultWindowClass регистрирует класс окна результата (вызывается один раз).
func registerResultWindowClass() {
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
	clsName := syscall.StringToUTF16Ptr("ResultWindowClass")
	wc := WNDCLASSEX{
		cbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		lpfnWndProc:   syscall.NewCallback(resultWndProc),
		hInstance:     getModuleHandle(),
		hbrBackground: 6, // COLOR_WINDOW + 1
		lpszClassName: clsName,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
}

func resultWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		hInst := getModuleHandle()
		editStyle := uintptr(WS_CHILD | WS_VISIBLE | ES_MULTILINE | ES_READONLY | WS_VSCROLL | ES_AUTOVSCROLL)
		hwndOrigEdit, _, _ = procCreateWindowExW.Call(
			WS_EX_CLIENTEDGE,
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("EDIT"))),
			0, editStyle,
			5, 5, 490, 185,
			hwnd, 0, hInst, 0,
		)
		hwndTransEdit, _, _ = procCreateWindowExW.Call(
			WS_EX_CLIENTEDGE,
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("EDIT"))),
			0, editStyle,
			5, 205, 490, 185,
			hwnd, 0, hInst, 0,
		)
		if hwndOrigEdit != 0 {
			editSubclassOnce.Do(func() {
				editSubclassCallbackPtr = syscall.NewCallback(editSubclassProc)
			})
			origEditProc, _, _ = procSetWindowLongPtrW.Call(
				hwndOrigEdit, GWLP_WNDPROC, editSubclassCallbackPtr,
			)
		}
		return 0

	case WM_SIZE:
		w := int(lParam & 0xFFFF)
		h := int((lParam >> 16) & 0xFFFF)
		half := (h - 15) / 2
		if half < 30 {
			half = 30
		}
		procSetWindowPos.Call(hwndOrigEdit, 0, 5, 5,
			uintptr(w-10), uintptr(half), SWP_NOZORDER)
		procSetWindowPos.Call(hwndTransEdit, 0, 5, uintptr(half+10),
			uintptr(w-10), uintptr(half), SWP_NOZORDER)
		return 0

	case WM_CLOSE:
		procDestroyWindow.Call(hwnd)
		return 0

	case WM_DESTROY:
		if origEditProc != 0 && hwndOrigEdit != 0 {
			procSetWindowLongPtrW.Call(hwndOrigEdit, GWLP_WNDPROC, origEditProc)
			origEditProc = 0
		}
		hwndOrigEdit = 0
		hwndTransEdit = 0
		currentResultWindow = 0
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func editSubclassProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == WM_LBUTTONDBLCLK {
		procCallWindowProcW.Call(origEditProc, hwnd, uintptr(msg), wParam, lParam)

		var selStart, selEnd uint32
		procSendMessageW.Call(hwnd, EM_GETSEL,
			uintptr(unsafe.Pointer(&selStart)),
			uintptr(unsafe.Pointer(&selEnd)))

		if selEnd > selStart {
			textLen, _, _ := procSendMessageW.Call(hwnd, WM_GETTEXTLENGTH, 0, 0)
			if textLen > 0 && selEnd <= uint32(textLen) {
				buf := make([]uint16, textLen+1)
				procSendMessageW.Call(hwnd, WM_GETTEXT,
					textLen+1, uintptr(unsafe.Pointer(&buf[0])))
				wordU16 := make([]uint16, selEnd-selStart+1)
				copy(wordU16, buf[selStart:selEnd])
				word := syscall.UTF16ToString(wordU16)
				word = strings.ToLower(strings.TrimFunc(word, func(r rune) bool {
					return !unicode.IsLetter(r) && !unicode.IsDigit(r)
				}))
				if word != "" {
					lookupAndShowWord(word)
				}
			}
		}
		return 0
	}
	ret, _, _ := procCallWindowProcW.Call(origEditProc, hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func showResultWindow(originalText, translatedText string, x, y int) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	resultWindowClassOnce.Do(registerResultWindowClass)

	hwnd, _, _ := procCreateWindowExW.Call(
		WS_EX_TOPMOST,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("ResultWindowClass"))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Результат перевода"))),
		WS_OVERLAPPEDWINDOW|WS_VISIBLE,
		uintptr(x), uintptr(y), 520, 430,
		0, 0, getModuleHandle(), 0,
	)
	if hwnd == 0 {
		return
	}
	currentResultWindow = hwnd

	if hwndOrigEdit != 0 {
		procSetWindowTextW.Call(hwndOrigEdit,
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(originalText))))
	}
	if hwndTransEdit != 0 {
		procSetWindowTextW.Call(hwndTransEdit,
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(translatedText))))
	}

	var msg struct {
		hwnd    uintptr
		message uint32
		wParam  uintptr
		lParam  uintptr
		time    uint32
		pt      struct{ x, y int32 }
	}
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 || ret == 0xFFFFFFFF {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func processSelection(rect image.Rectangle) {
	closeResultWindow()

	fmt.Printf("\n=== Выделена область: (%d,%d) - (%d,%d) ===\n", rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y)
	text, err := captureAndOCR(rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy())
	if err != nil {
		fmt.Printf("Ошибка OCR: %v\n", err)
		onSelectionDone(false)
		return
	}
	fmt.Println("=== Распознанный текст ===")
	fmt.Println(text)

	translated, err := translateNMT(text)
	if err != nil {
		fmt.Printf("Ошибка перевода: %v\n", err)
		onSelectionDone(false)
		return
	}
	fmt.Println("=== Перевод ===")
	fmt.Println(translated)

	showResultWindow(text, translated, rect.Max.X, rect.Max.Y)

	fmt.Println("Перевод завершён. Можно запускать новый захват.")
	onSelectionDone(true)
}
