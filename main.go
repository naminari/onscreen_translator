package main

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/disintegration/imaging"
	"github.com/kbinani/screenshot"
)

func setConsoleUTF8() {
	exec.Command("cmd", "/c", "chcp 65001").Run()
}

func loadDictionary(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dict := make(map[string]string)
	s := bufio.NewScanner(f)
	for s.Scan() {
		parts := strings.SplitN(s.Text(), "|", 2)
		if len(parts) == 2 {
			dict[strings.ToLower(parts[0])] = parts[1]
		}
	}
	return dict, s.Err()
}

func preprocessImage(img image.Image) image.Image {
	gray := imaging.Grayscale(img)
	contrasted := imaging.AdjustContrast(gray, 25)
	binary := imaging.AdjustFunc(contrasted, func(c color.NRGBA) color.NRGBA {
		b := 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
		if b > 127 {
			return color.NRGBA{255, 255, 255, 255}
		}
		return color.NRGBA{0, 0, 0, 255}
	})
	return imaging.Sharpen(binary, 1.2)
}

var corrections = map[string]string{
	"Kak": "Как", "kak": "как",
	"Паскал": "Паскаль", "паскал": "паскаль",
	"Графииеский": "Графический", "графииеский": "графический",
}

func cleanOCRText(raw string) string {
	lines := strings.Split(raw, "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		letters := 0
		for _, ch := range line {
			if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == ' ' {
				letters++
			}
		}
		if float64(letters)/float64(len(line)) < 0.7 {
			continue
		}
		words := strings.Fields(line)
		filtered := make([]string, 0, len(words))
		for _, w := range words {
			if len(w) >= 2 || (len(w) == 1 && unicode.IsLetter([]rune(w)[0])) {
				if corr, ok := corrections[w]; ok {
					w = corr
				}
				filtered = append(filtered, w)
			}
		}
		if len(filtered) > 0 {
			out = append(out, strings.Join(filtered, " "))
		}
	}
	return strings.Join(out, "\n")
}

func ocrImage(img image.Image) (string, error) {
	processed := preprocessImage(img)
	tmp, err := os.CreateTemp("", "screenshot-*.png")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := png.Encode(tmp, processed); err != nil {
		return "", err
	}
	tmp.Close()

	cmd := exec.Command("tesseract", tmp.Name(), "stdout",
		"-l", "eng+rus", "--psm", "6", "--oem", "3",
		"-c", "tessedit_char_whitelist=ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyzАБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯабвгдеёжзийклмнопрстуфхцчшщъыьэюя0123456789 .,!?;:")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return cleanOCRText(out.String()), nil
}

func captureAndOCR(x, y, w, h int) (string, error) {
	bounds := image.Rect(x, y, x+w, y+h)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return "", err
	}
	return ocrImage(img)
}

func checkNMTDependencies() error {
	if err := exec.Command("python", "--version").Run(); err != nil {
		return fmt.Errorf("python not found")
	}
	if err := exec.Command("python", "-c", "import transformers").Run(); err != nil {
		return fmt.Errorf("transformers not installed: pip install transformers torch")
	}
	if _, err := os.Stat("translator.py"); os.IsNotExist(err) {
		return fmt.Errorf("translator.py missing")
	}
	return nil
}

func runTestMode() {
	if err := checkNMTDependencies(); err != nil {
		fmt.Println("NMT unavailable:", err)
		time.Sleep(5 * time.Second)
		return
	}
	files, _ := filepath.Glob("test_samples/*.png")
	files2, _ := filepath.Glob("test_samples/*.jpg")
	all := append(files, files2...)
	if len(all) == 0 {
		fmt.Println("No test images")
		return
	}
	setConsoleUTF8()
	for _, f := range all {
		fmt.Printf("--- %s ---\n", filepath.Base(f))
		imgFile, err := os.Open(f)
		if err != nil {
			fmt.Println(err)
			continue
		}
		img, _, err := image.Decode(imgFile)
		imgFile.Close()
		if err != nil {
			fmt.Println(err)
			continue
		}
		text, err := ocrImage(img)
		if err != nil {
			fmt.Println("OCR:", err)
			continue
		}
		fmt.Println("OCR:\n", text)
		if text != "" {
			trans, err := translateNMT(text)
			if err != nil {
				fmt.Println("Translation:", err)
			} else {
				fmt.Println("Translation:\n", trans)
			}
		}
		fmt.Println()
	}
}

func translateText(text string, mainDict, gameDict map[string]string) string {
	words := strings.Fields(text)
	out := make([]string, len(words))
	for i, w := range words {
		lower := strings.ToLower(w)
		if tr, ok := gameDict[lower]; ok {
			out[i] = tr
		} else if tr, ok := mainDict[lower]; ok {
			out[i] = tr
		} else {
			out[i] = w
		}
	}
	return strings.Join(out, " ")
}

// Функция activateCaptureMode – запускает оверлей и устанавливает captureModeActive в true
func activateCaptureMode() {
	if captureModeActive {
		return
	}
	captureModeActive = true
	startScreenOverlay()
}

func main() {
	runtime.LockOSThread()
	procSetProcessDPIAware.Call()

	if err := LoadConfig(); err != nil {
		fmt.Println("Ошибка загрузки настроек:", err)
	} else {
		ApplyConfig()
	}

	hotkeyHwnd = CreateHotkeyWindow()
	if hotkeyHwnd == 0 {
		fmt.Println("Предупреждение: не удалось создать окно для горячей клавиши")
	} else {
		if currentConfig.UseHotkey {
			if err := RegisterHotkey(currentConfig.HotkeyMod, currentConfig.HotkeyVk); err != nil {
				fmt.Println("Ошибка регистрации горячей клавиши:", err)
			} else {
				fmt.Println("Горячая клавиша зарегистрирована")
			}
		}
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Println("Получен сигнал завершения, выход...")
		os.Exit(0)
	}()

	startUI()
}
