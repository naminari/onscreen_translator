package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func translateNMT(text string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python", "translator.py")
	cmd.Stdin = strings.NewReader(text)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("NMT error: %v", err)
	}
	return strings.TrimSpace(out.String()), nil
}
