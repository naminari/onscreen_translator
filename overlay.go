package main

import (
	"fmt"
	"image"
	"syscall"
	"time"
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
	WM_DESTROY       = 0x0002
	WM_NCHITTEST     = 0x0084
	WM_MOUSEACTIVATE = 0x0021
)

var (
	overlayHwnd       uintptr
	startX, startY    int
	drawing           bool
	selectionRect     image.Rectangle
	captureModeActive bool
	prevRect          image.Rectangle
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

func processSelection(rect image.Rectangle) {
	fmt.Printf("\n=== Выделена область: (%d,%d) - (%d,%d) ===\n", rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y)
	text, err := captureAndOCR(rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy())
	if err != nil {
		fmt.Printf("Ошибка OCR: %v\n", err)
		activateCaptureMode()
		return
	}
	fmt.Println("=== Распознанный текст ===")
	fmt.Println(text)

	translated, err := translateNMT(text)
	if err != nil {
		fmt.Printf("Ошибка перевода: %v\n", err)
		activateCaptureMode()
		return
	}
	fmt.Println("=== Перевод ===")
	fmt.Println(translated)

	fmt.Println("(Программа будет ждать 3 секунды и снова активирует выделение)")
	time.Sleep(3 * time.Second)
	activateCaptureMode()
}

func activateCaptureMode() {
	if captureModeActive {
		return
	}
	captureModeActive = true
	startScreenOverlay()
}
