package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	translatorCmd    *exec.Cmd
	translatorStdin  io.WriteCloser
	translatorStdout *bufio.Reader
	translatorMu     sync.Mutex
	translatorOnce   sync.Once
	translatorReady  = make(chan struct{})
)

func initTranslator() error {
	var errInit error
	translatorOnce.Do(func() {
		cmd := exec.Command("python", "-u", "translator.py")
		stdin, err := cmd.StdinPipe()
		if err != nil {
			errInit = fmt.Errorf("stdin pipe: %v", err)
			return
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			errInit = fmt.Errorf("stdout pipe: %v", err)
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			errInit = fmt.Errorf("stderr pipe: %v", err)
			return
		}
		if err := cmd.Start(); err != nil {
			errInit = fmt.Errorf("start: %v", err)
			return
		}
		translatorCmd = cmd
		translatorStdin = stdin
		translatorStdout = bufio.NewReader(stdout)

		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := scanner.Text()
				fmt.Fprintln(os.Stderr, line)
				if strings.Contains(line, "[NMT] Ready") {
					close(translatorReady)
				}
			}
		}()
	})
	if errInit != nil {
		return errInit
	}
	select {
	case <-translatorReady:
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("таймаут ожидания готовности переводчика")
	}
}

func translateNMT(text string) (string, error) {
	if err := initTranslator(); err != nil {
		return "", fmt.Errorf("инициализация переводчика: %v", err)
	}
	lines := strings.Split(text, "\n")
	var results []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		translated, err := translateLine(line)
		if err != nil {
			return "", err
		}
		if translated != "" {
			results = append(results, translated)
		}
	}
	return strings.Join(results, "\n"), nil
}

func translateLine(line string) (string, error) {
	translatorMu.Lock()
	defer translatorMu.Unlock()
	if _, err := fmt.Fprintf(translatorStdin, "%s\n", line); err != nil {
		return "", err
	}
	result, err := translatorStdout.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result), nil
}

func closeTranslator() {
	if translatorCmd != nil && translatorCmd.Process != nil {
		translatorCmd.Process.Kill()
	}
}
