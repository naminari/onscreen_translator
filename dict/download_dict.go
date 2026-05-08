package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

func main() {
	inPath := "dic.txt"
	outPath := "en-ru-utf8.txt"

	in, err := os.Open(inPath)
	if err != nil {
		fmt.Println("Ошибка открытия:", err)
		return
	}
	defer in.Close()

	decoder := charmap.KOI8R.NewDecoder()
	utf8Reader := decoder.Reader(in)

	out, err := os.Create(outPath)
	if err != nil {
		fmt.Println("Ошибка создания:", err)
		return
	}
	defer out.Close()

	scanner := bufio.NewScanner(utf8Reader)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			word := parts[0]
			trans := strings.Join(parts[1:], " ")
			fmt.Fprintf(out, "%s|%s\n", word, trans)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Ошибка чтения:", err)
	}
	fmt.Println("Конвертация завершена")
}
