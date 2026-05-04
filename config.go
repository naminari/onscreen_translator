package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Константы модификаторов (скопированы из hotkey.go)
const (
	MOD_ALT     = 0x0001
	MOD_CONTROL = 0x0002
	MOD_SHIFT   = 0x0004
	MOD_WIN     = 0x0008
)

type Config struct {
	UseHotkey bool   `json:"use_hotkey"`
	HotkeyMod uint32 `json:"hotkey_mod"`
	HotkeyVk  uint32 `json:"hotkey_vk"`
}

var currentConfig Config

const configFileName = "translator_config.json"

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ScreenTranslator", configFileName), nil
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
	fmt.Printf("Настройки применены: UseHotkey=%v, HotkeyMod=%d, HotkeyVk=%d\n",
		currentConfig.UseHotkey, currentConfig.HotkeyMod, currentConfig.HotkeyVk)
}
