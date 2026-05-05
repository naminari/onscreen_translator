package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	UseHotkey bool   `json:"use_hotkey"`
	HotkeyMod uint32 `json:"hotkey_mod"`
	HotkeyVk  uint32 `json:"hotkey_vk"`
}

var currentConfig Config

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ScreenTranslator", "config.json"), nil
}

func LoadConfig() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			currentConfig = Config{
				UseHotkey: false,
				HotkeyMod: MOD_CONTROL | MOD_SHIFT,
				HotkeyVk:  0x46, // 'F'
			}
			return SaveConfig()
		}
		return err
	}
	return json.Unmarshal(data, &currentConfig)
}

func SaveConfig() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("не удалось создать папку конфига: %w", err)
	}
	data, err := json.MarshalIndent(currentConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func ApplyConfig() {
	fmt.Printf("Настройки применены: UseHotkey=%v, Mod=%d, Vk=%d\n",
		currentConfig.UseHotkey, currentConfig.HotkeyMod, currentConfig.HotkeyVk)
	// Перерегистрируем горячую клавишу, если окно уже создано
	if hotkeyHwnd != 0 {
		UnregisterHotkey()
		if currentConfig.UseHotkey {
			if err := RegisterHotkey(currentConfig.HotkeyMod, currentConfig.HotkeyVk); err != nil {
				fmt.Println("Ошибка регистрации горячей клавиши:", err)
			} else {
				fmt.Println("Горячая клавиша зарегистрирована")
			}
		}
	}
}
