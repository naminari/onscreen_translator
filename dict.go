package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"
)

var globalDict map[string][]string

func loadDictionaryMap(path string) (map[string][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dict := make(map[string][]string)
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		var translations []string
		for _, t := range strings.Split(val, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				translations = append(translations, t)
			}
		}
		if key != "" && len(translations) > 0 {
			dict[key] = translations
		}
	}
	return dict, s.Err()
}

func lookupWord(word string) []string {
	if globalDict == nil {
		return nil
	}
	return globalDict[strings.ToLower(word)]
}

func lookupAndShowWord(word string) {
	clean := strings.TrimFunc(word, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	clean = strings.ToLower(clean)
	if clean == "" {
		return
	}
	translations := lookupWord(clean)
	if len(translations) == 0 {
		msgBox("Словарь", fmt.Sprintf("Перевод слова «%s» не найден", clean))
		return
	}
	msgBox(fmt.Sprintf("Перевод: %s", clean), strings.Join(translations, "\n"))
}
