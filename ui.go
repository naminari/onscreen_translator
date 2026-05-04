package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Константы стилей и сообщений
const (
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_CHILD            = 0x40000000
	WS_MAXIMIZEBOX      = 0x00010000
	WS_THICKFRAME       = 0x00040000
	WS_CAPTION          = 0x00C00000
	WS_SYSMENU          = 0x00080000
	WS_MINIMIZEBOX      = 0x00020000
	WM_CREATE           = 0x0001
	WM_COMMAND          = 0x0111
	WM_DESTROY          = 0x0002
	WM_CLOSE            = 0x0010
	WM_CTLCOLORSTATIC   = 0x0138
	WM_CTLCOLORBTN      = 0x0135

	BS_AUTORADIOBUTTON = 0x00000009
	BS_AUTOCHECKBOX    = 0x00000003

	CB_ADDSTRING     = 0x0143
	CB_GETCOUNT      = 0x0146
	CB_GETCURSEL     = 0x0147
	CB_SETCURSEL     = 0x014E
	CB_GETLBTEXT     = 0x0148
	BM_SETCHECK      = 0x00F1
	BM_GETCHECK      = 0x00F0
	BST_CHECKED      = 1
	CBS_DROPDOWNLIST = 0x0003
)

// Идентификаторы кнопок
const (
	IDC_BUTTON_CAPTURE  = 1001
	IDC_BUTTON_EXIT     = 1002
	IDC_BUTTON_SETTINGS = 1003
	// Идентификаторы для окна настроек
	IDC_RADIO_BUTTON  = 2001
	IDC_RADIO_HOTKEY  = 2002
	IDC_CHECK_CTRL    = 2003
	IDC_CHECK_ALT     = 2004
	IDC_CHECK_SHIFT   = 2005
	IDC_CHECK_WIN     = 2006
	IDC_COMBO_KEY     = 2007
	IDC_BUTTON_OK     = 2008
	IDC_BUTTON_CANCEL = 2009
)

var (
	mainHwnd           uintptr
	settingsHwnd       uintptr
	processing         bool
	oldMainWndProc     uintptr // для восстановления, если потребуется
	procSetBkMode      = gdi32.NewProc("SetBkMode")
	procSetTextColor   = gdi32.NewProc("SetTextColor")
	procGetWindowLongW = user32.NewProc("GetWindowLongW")
	procSetWindowLongW = user32.NewProc("SetWindowLongW")
)

func LOWORD(dw uintptr) uint16 { return uint16(dw & 0xFFFF) }
func HIWORD(dw uintptr) uint16 { return uint16((dw >> 16) & 0xFFFF) }

// Вспомогательные функции для клавиш
var mapVKToName = map[uint32]string{
	0x70: "F1", 0x71: "F2", 0x72: "F3", 0x73: "F4",
	0x74: "F5", 0x75: "F6", 0x76: "F7", 0x77: "F8",
	0x78: "F9", 0x79: "F10", 0x7A: "F11", 0x7B: "F12",
	0x09: "Tab", 0x0D: "Enter", 0x20: "Space",
	0x1B: "Esc", 0x08: "Backspace",
}

func keyNameFromVK(vk uint32) string {
	if name, ok := mapVKToName[vk]; ok {
		return name
	}
	if vk >= 'A' && vk <= 'Z' {
		return string(rune(vk))
	}
	if vk >= '0' && vk <= '9' {
		return string(rune(vk))
	}
	return "?"
}

func vkFromName(name string) uint32 {
	for vk, nm := range mapVKToName {
		if nm == name {
			return vk
		}
	}
	if len(name) == 1 {
		ch := name[0]
		if ch >= 'A' && ch <= 'Z' {
			return uint32(ch)
		}
		if ch >= '0' && ch <= '9' {
			return uint32(ch)
		}
	}
	return 0
}

// Создание кнопки
func createButton(parent uintptr, text string, x, y, w, h int, id uint16) uintptr {
	hInst := getModuleHandle()
	ret, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("BUTTON"))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))),
		WS_VISIBLE|WS_CHILD,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		parent,
		uintptr(id),
		hInst,
		0,
	)
	return ret
}

// Обработчик главного окна
func mainWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		// Увеличили размеры окна: ширина 240, высота 180 (раньше 200x150)
		procSetWindowPos.Call(hwnd, 0, 0, 0, 240, 180, 0x0002|0x0001)
		createButton(hwnd, "Захват области", 10, 10, 200, 40, IDC_BUTTON_CAPTURE)
		createButton(hwnd, "Настройки", 10, 60, 200, 40, IDC_BUTTON_SETTINGS)
		createButton(hwnd, "Выход", 10, 110, 200, 40, IDC_BUTTON_EXIT)
		return 0

	case WM_COMMAND:
		switch LOWORD(wParam) {
		case IDC_BUTTON_CAPTURE:
			if processing {
				fmt.Println("Обработка уже идёт, подождите...")
				return 0
			}
			processing = true
			fmt.Println("Запуск захвата области...")
			activateCaptureMode()
			return 0
		case IDC_BUTTON_SETTINGS:
			if settingsHwnd == 0 {
				createSettingsWindow(hwnd)
			} else {
				// Если окно уже открыто, показываем его
				procShowWindow.Call(settingsHwnd, 1)
				procSetForegroundWindow.Call(settingsHwnd)
			}
			return 0
		case IDC_BUTTON_EXIT:
			procDestroyWindow.Call(hwnd)
			return 0
		}
		return 0

	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0

	default:
		ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return ret
	}
}

// Регистрация класса главного окна
func initMainWindowClass() {
	className := syscall.StringToUTF16Ptr("MainWindowClass")
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
		lpfnWndProc:   syscall.NewCallback(mainWndProc),
		hInstance:     getModuleHandle(),
		hbrBackground: 0,
		lpszClassName: className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
}

// startUI запускает главное окно
func startUI() {
	initMainWindowClass()
	// Увеличили начальные размеры: 240x180
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("MainWindowClass"))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Screen Translator"))),
		WS_OVERLAPPEDWINDOW|WS_VISIBLE,
		100, 100, 240, 220,
		0, 0, getModuleHandle(), 0,
	)
	if hwnd == 0 {
		fmt.Println("Не удалось создать главное окно")
		return
	}
	mainHwnd = hwnd

	// Запретить изменение размера (закомментировано, можно раскомментировать)
	// style := uint32(procGetWindowLongW.Call(hwnd, -16)[0]) // GWL_STYLE
	// procSetWindowLongW.Call(hwnd, -16, uintptr(style & ^WS_THICKFRAME & ^WS_MAXIMIZEBOX))

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

func onSelectionDone(success bool) {
	processing = false
	fmt.Println("Захват завершён, кнопка разблокирована.")
}

// ----------------------------------------
// Окно настроек (немодальное, без вложенного цикла)
// ----------------------------------------

var (
	settingsWndProcBackup uintptr
)

func createSettingsWindow(parent uintptr) {
	// Регистрируем класс окна настроек
	className := syscall.StringToUTF16Ptr("SettingsWindowClass")
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
		lpfnWndProc:   syscall.NewCallback(settingsWndProc),
		hInstance:     getModuleHandle(),
		hbrBackground: 0, // будем рисовать белый фон
		lpszClassName: className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	// Создаём окно (без возможности изменения размера, но раскомментировать при желании)
	style := WS_OVERLAPPEDWINDOW | WS_VISIBLE & ^WS_MAXIMIZEBOX & ^WS_THICKFRAME
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Настройки"))),
		uintptr(style),
		150, 150, 340, 280,
		parent,
		0,
		getModuleHandle(),
		0,
	)
	if hwnd == 0 {
		fmt.Println("Не удалось создать окно настроек")
		return
	}
	settingsHwnd = hwnd

	// Блокируем главное окно (пока открыты настройки)
	procEnableWindow.Call(parent, 0)

	// Создаём элементы управления
	createRadioButton(hwnd, "Запуск по кнопке", 10, 10, 150, 20, IDC_RADIO_BUTTON)
	createRadioButton(hwnd, "Глобальная горячая клавиша", 10, 40, 180, 20, IDC_RADIO_HOTKEY)

	createCheckBox(hwnd, "Ctrl", 30, 70, 60, 20, IDC_CHECK_CTRL)
	createCheckBox(hwnd, "Alt", 100, 70, 60, 20, IDC_CHECK_ALT)
	createCheckBox(hwnd, "Shift", 170, 70, 60, 20, IDC_CHECK_SHIFT)
	createCheckBox(hwnd, "Win", 240, 70, 60, 20, IDC_CHECK_WIN)

	createComboBox(hwnd, 30, 100, 120, 200, IDC_COMBO_KEY, getKeyList())
	createButton(hwnd, "OK", 100, 150, 60, 25, IDC_BUTTON_OK)
	createButton(hwnd, "Отмена", 170, 150, 70, 25, IDC_BUTTON_CANCEL)

	applyConfigToDialog(hwnd)
}

// Обработчик окна настроек
func settingsWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_ERASEBKGND:
		// Заливаем белым фоном
		brush, _, _ := procCreateSolidBrush.Call(0x00FFFFFF) // белый цвет
		rect := struct{ left, top, right, bottom int32 }{0, 0, 340, 280}
		procFillRect.Call(wParam, uintptr(unsafe.Pointer(&rect)), brush)
		procDeleteObject.Call(brush)
		return 1

	case WM_CTLCOLORSTATIC, WM_CTLCOLORBTN:
		// Белый фон для элементов
		brush, _, _ := procCreateSolidBrush.Call(0x00FFFFFF)
		procSetBkMode.Call(wParam, 1)             // TRANSPARENT
		procSetTextColor.Call(wParam, 0x00000000) // чёрный текст
		return brush

	case WM_COMMAND:
		switch LOWORD(wParam) {
		case IDC_BUTTON_OK:
			saveConfigFromDialog(hwnd)
			SaveConfig()
			ApplyConfig()
			procDestroyWindow.Call(hwnd)
			return 0
		case IDC_BUTTON_CANCEL:
			procDestroyWindow.Call(hwnd)
			return 0
		case IDC_RADIO_BUTTON, IDC_RADIO_HOTKEY:
			enableHotkeyControls(hwnd, getCheckedRadio(hwnd) == IDC_RADIO_HOTKEY)
		}
		return 0

	case WM_CLOSE:
		procDestroyWindow.Call(hwnd)
		return 0

	case WM_DESTROY:
		settingsHwnd = 0
		// Разблокируем главное окно
		procEnableWindow.Call(mainHwnd, 1)
		procSetForegroundWindow.Call(mainHwnd)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

// Вспомогательные функции для создания элементов
func createRadioButton(parent uintptr, text string, x, y, w, h int, id uint16) {
	hInst := getModuleHandle()
	procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("BUTTON"))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))),
		WS_VISIBLE|WS_CHILD|BS_AUTORADIOBUTTON,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		parent,
		uintptr(id),
		hInst,
		0,
	)
}

func createCheckBox(parent uintptr, text string, x, y, w, h int, id uint16) {
	hInst := getModuleHandle()
	procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("BUTTON"))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))),
		WS_VISIBLE|WS_CHILD|BS_AUTOCHECKBOX,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		parent,
		uintptr(id),
		hInst,
		0,
	)
}

func createComboBox(parent uintptr, x, y, w, h int, id uint16, items []string) {
	hInst := getModuleHandle()
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("COMBOBOX"))),
		0,
		WS_VISIBLE|WS_CHILD|CBS_DROPDOWNLIST,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		parent,
		uintptr(id),
		hInst,
		0,
	)
	if hwnd != 0 {
		for _, item := range items {
			procSendMessageW.Call(hwnd, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(item))))
		}
	}
}

func getKeyList() []string {
	keys := []string{}
	for i := 'A'; i <= 'Z'; i++ {
		keys = append(keys, string(i))
	}
	for i := 0; i <= 9; i++ {
		keys = append(keys, fmt.Sprintf("%d", i))
	}
	for i := 1; i <= 12; i++ {
		keys = append(keys, fmt.Sprintf("F%d", i))
	}
	return keys
}

func getCheckedRadio(hwnd uintptr) uint16 {
	btn1, _, _ := procSendMessageW.Call(GetDlgItem(hwnd, IDC_RADIO_BUTTON), BM_GETCHECK, 0, 0)
	if btn1 == BST_CHECKED {
		return IDC_RADIO_BUTTON
	}
	return IDC_RADIO_HOTKEY
}

func applyConfigToDialog(hwnd uintptr) {
	if currentConfig.UseHotkey {
		procSendMessageW.Call(GetDlgItem(hwnd, IDC_RADIO_HOTKEY), BM_SETCHECK, BST_CHECKED, 0)
	} else {
		procSendMessageW.Call(GetDlgItem(hwnd, IDC_RADIO_BUTTON), BM_SETCHECK, BST_CHECKED, 0)
	}
	setCheckbox(hwnd, IDC_CHECK_CTRL, (currentConfig.HotkeyMod&MOD_CONTROL) != 0)
	setCheckbox(hwnd, IDC_CHECK_ALT, (currentConfig.HotkeyMod&MOD_ALT) != 0)
	setCheckbox(hwnd, IDC_CHECK_SHIFT, (currentConfig.HotkeyMod&MOD_SHIFT) != 0)
	setCheckbox(hwnd, IDC_CHECK_WIN, (currentConfig.HotkeyMod&MOD_WIN) != 0)

	keyName := keyNameFromVK(currentConfig.HotkeyVk)
	combo := GetDlgItem(hwnd, IDC_COMBO_KEY)
	count, _, _ := procSendMessageW.Call(combo, CB_GETCOUNT, 0, 0)
	for i := 0; i < int(count); i++ {
		buf := make([]uint16, 256)
		procSendMessageW.Call(combo, CB_GETLBTEXT, uintptr(i), uintptr(unsafe.Pointer(&buf[0])))
		if syscall.UTF16ToString(buf) == keyName {
			procSendMessageW.Call(combo, CB_SETCURSEL, uintptr(i), 0)
			break
		}
	}
	enableHotkeyControls(hwnd, currentConfig.UseHotkey)
}

func saveConfigFromDialog(hwnd uintptr) {
	currentConfig.UseHotkey = (getCheckedRadio(hwnd) == IDC_RADIO_HOTKEY)
	var mod uint32 = 0
	if isCheckboxChecked(hwnd, IDC_CHECK_CTRL) {
		mod |= MOD_CONTROL
	}
	if isCheckboxChecked(hwnd, IDC_CHECK_ALT) {
		mod |= MOD_ALT
	}
	if isCheckboxChecked(hwnd, IDC_CHECK_SHIFT) {
		mod |= MOD_SHIFT
	}
	if isCheckboxChecked(hwnd, IDC_CHECK_WIN) {
		mod |= MOD_WIN
	}
	currentConfig.HotkeyMod = mod

	combo := GetDlgItem(hwnd, IDC_COMBO_KEY)
	idx, _, _ := procSendMessageW.Call(combo, CB_GETCURSEL, 0, 0)
	if idx != 0xFFFFFFFF {
		buf := make([]uint16, 32)
		procSendMessageW.Call(combo, CB_GETLBTEXT, idx, uintptr(unsafe.Pointer(&buf[0])))
		keyName := syscall.UTF16ToString(buf)
		currentConfig.HotkeyVk = vkFromName(keyName)
	}
}

func setCheckbox(hwnd uintptr, id uint16, checked bool) {
	check := 0
	if checked {
		check = BST_CHECKED
	}
	procSendMessageW.Call(GetDlgItem(hwnd, id), BM_SETCHECK, uintptr(check), 0)
}

func isCheckboxChecked(hwnd uintptr, id uint16) bool {
	ret, _, _ := procSendMessageW.Call(GetDlgItem(hwnd, id), BM_GETCHECK, 0, 0)
	return ret == BST_CHECKED
}

func enableHotkeyControls(hwnd uintptr, enable bool) {
	val := uintptr(0)
	if enable {
		val = 1
	}
	procEnableWindow.Call(GetDlgItem(hwnd, IDC_CHECK_CTRL), val)
	procEnableWindow.Call(GetDlgItem(hwnd, IDC_CHECK_ALT), val)
	procEnableWindow.Call(GetDlgItem(hwnd, IDC_CHECK_SHIFT), val)
	procEnableWindow.Call(GetDlgItem(hwnd, IDC_CHECK_WIN), val)
	procEnableWindow.Call(GetDlgItem(hwnd, IDC_COMBO_KEY), val)
}

func GetDlgItem(hwnd uintptr, id uint16) uintptr {
	ret, _, _ := procGetDlgItem.Call(hwnd, uintptr(id))
	return ret
}

func onHotkey() {
	if processing {
		fmt.Println("Обработка уже идёт, горячая клавиша игнорируется")
		return
	}
	processing = true
	fmt.Println("Захват области по горячей клавише...")
	activateCaptureMode()
}
