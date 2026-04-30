package main

import (
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
	procInvalidateRect             = user32.NewProc("InvalidateRect")
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

	procCreateSolidBrush    = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject        = gdi32.NewProc("DeleteObject")
	procDrawTextW           = user32.NewProc("DrawTextW")
	procSelectObject        = gdi32.NewProc("SelectObject")
	procCreateFontIndirectW = gdi32.NewProc("CreateFontIndirectW")
)

const (
	WS_EX_LAYERED = 0x80000
	WS_EX_TOPMOST = 0x8
	WS_POPUP      = 0x80000000
	WS_VISIBLE    = 0x10000000
	WS_BORDER     = 0x00800000
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
	WM_KEYDOWN       = 0x0100
	VK_ESCAPE        = 0x1B

	DT_CENTER    = 0x00000001
	DT_VCENTER   = 0x00000004
	DT_WORDBREAK = 0x00000010
)

var (
	overlayHwnd       uintptr
	startX, startY    int
	drawing           bool
	selectionRect     image.Rectangle
	captureModeActive bool
	resultText        string
)

var className = syscall.StringToUTF16Ptr("OverlayWindowClass")
var resultClassName = syscall.StringToUTF16Ptr("ResultWindowClass")

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

func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_NCHITTEST:
		return 1
	case WM_MOUSEACTIVATE:
		return 3
	case WM_LBUTTONDOWN:
		drawing = true
		rawX := int(int16(lParam & 0xFFFF))
		rawY := int(int16(lParam >> 16))
		offX, _, _ := procGetSystemMetrics.Call(SM_XVIRTUALSCREEN)
		offY, _, _ := procGetSystemMetrics.Call(SM_YVIRTUALSCREEN)
		startX = rawX + int(offX)
		startY = rawY + int(offY)
		selectionRect = image.Rect(startX, startY, startX, startY)
		return 0
	case WM_MOUSEMOVE:
		if drawing {
			x := int(lParam & 0xFFFF)
			y := int(lParam >> 16)
			selectionRect = image.Rect(startX, startY, x, y)
		}
		return 0
	case WM_LBUTTONUP:
		if !drawing {
			return 0
		}
		drawing = false
		rawX := int(int16(lParam & 0xFFFF))
		rawY := int(int16(lParam >> 16))
		offX, _, _ := procGetSystemMetrics.Call(SM_XVIRTUALSCREEN)
		offY, _, _ := procGetSystemMetrics.Call(SM_YVIRTUALSCREEN)
		endX := rawX + int(offX)
		endY := rawY + int(offY)
		selectionRect.Max = image.Point{endX, endY}
		finalRect := normalizeRect(selectionRect)
		if finalRect.Dx() < 10 || finalRect.Dy() < 10 {
			procDestroyWindow.Call(hwnd)
			return 0
		}
		procDestroyWindow.Call(hwnd)
		go processSelection(finalRect)
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
	text, err := captureAndOCR(rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy())
	if err != nil {
		activateCaptureMode()
		return
	}
	translated, err := translateNMT(text)
	if err != nil {
		activateCaptureMode()
		return
	}
	done := make(chan struct{})
	// Позиционируем окно результата рядом с выделенной областью, не выходя за экран
	x, y := rect.Max.X+10, rect.Max.Y+10
	sw, _, _ := procGetSystemMetrics.Call(SM_CXVIRTUALSCREEN)
	sh, _, _ := procGetSystemMetrics.Call(SM_CYVIRTUALSCREEN)
	if x+400 > int(sw) {
		x = rect.Min.X - 410
	}
	if y+200 > int(sh) {
		y = rect.Min.Y - 210
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	showResultOverlay(translated, x, y, done)
	<-done
	activateCaptureMode()
}

func initResultClass() {
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
		lpfnWndProc:   syscall.NewCallback(resultWndProc),
		hInstance:     getModuleHandle(),
		lpszClassName: resultClassName,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
}

func resultWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
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

		font, _, _ := procCreateFontIndirectW.Call(
			uintptr(unsafe.Pointer(&struct {
				lfHeight         int32
				lfWidth          int32
				lfEscapement     int32
				lfOrientation    int32
				lfWeight         int32
				lfItalic         byte
				lfUnderline      byte
				lfStrikeOut      byte
				lfCharSet        byte
				lfOutPrecision   byte
				lfClipPrecision  byte
				lfQuality        byte
				lfPitchAndFamily byte
				lfFaceName       [32]uint16
			}{
				lfHeight:   -20,
				lfWeight:   400,
				lfFaceName: strToUTF16("Arial"),
			})))
		oldFont, _, _ := procSelectObject.Call(hdc, font)
		user32.NewProc("SetTextColor").Call(hdc, 0x00FFFFFF)
		user32.NewProc("SetBkMode").Call(hdc, 1)

		textPtr := uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(resultText)))
		procDrawTextW.Call(hdc, textPtr, uintptr(^uintptr(0)), uintptr(unsafe.Pointer(&rect)),
			DT_CENTER|DT_VCENTER|DT_WORDBREAK)

		procSelectObject.Call(hdc, oldFont)
		procDeleteObject.Call(font)
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_KEYDOWN:
		if wParam == VK_ESCAPE {
			procDestroyWindow.Call(hwnd)
		}
		return 0
	case WM_DESTROY:
		return 0
	default:
		ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return ret
	}
}

func strToUTF16(s string) [32]uint16 {
	var arr [32]uint16
	for i, ch := range s {
		if i >= 31 {
			break
		}
		arr[i] = uint16(ch)
	}
	return arr
}

func showResultOverlay(text string, x, y int, done chan struct{}) {
	initResultClass()
	resultText = text
	hwnd, _, _ := procCreateWindowExW.Call(
		WS_EX_TOPMOST,
		uintptr(unsafe.Pointer(resultClassName)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Translation"))),
		WS_POPUP|WS_VISIBLE|WS_BORDER,
		uintptr(x), uintptr(y), 400, 200,
		0, 0, getModuleHandle(), 0,
	)
	if hwnd == 0 {
		close(done)
		return
	}
	go func() {
		time.Sleep(7 * time.Second)
		procDestroyWindow.Call(hwnd)
	}()
	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			if isWindow, _, _ := procIsWindow.Call(hwnd); isWindow == 0 {
				close(done)
				return
			}
		}
	}()
}

func activateCaptureMode() {
	if captureModeActive {
		return
	}
	captureModeActive = true
	startScreenOverlay()
}
