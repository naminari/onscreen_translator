package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func translateNMT(text string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second) // увеличен до 90 секунд
	defer cancel()
	cmd := exec.CommandContext(ctx, "python", "translator.py")
	cmd.Stdin = strings.NewReader(text)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("NMT error: %v\nPython stderr: %s", err, stderr.String())
		}
		return "", fmt.Errorf("NMT error: %v", err)
	}
	return strings.TrimSpace(out.String()), nil
}
