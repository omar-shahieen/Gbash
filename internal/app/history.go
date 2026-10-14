package app

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const historyLimit = 1000

func historyPath() (string, error) {
	path, set := os.LookupEnv("HISTFILE")
	if !set {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, ".gbash_history")
	}
	if path == "" {
		return "", nil
	} // Empty HISTFILE disables persistence.
	return filepath.Abs(path)
}

func loadHistory(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var history []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || (len(history) > 0 && history[len(history)-1] == line) {
			continue
		}
		history = append(history, line)
		if len(history) > historyLimit {
			history = history[1:]
		}
	}
	return history, scanner.Err()
}

func appendHistory(path, line string) error {
	if path == "" {
		return nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(line + "\n")
	return errors.Join(writeErr, file.Close())
}
