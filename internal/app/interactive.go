package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/omar-shahieen/gbash/internal/shell"
)

type interactiveInput struct {
	line        *readline.Instance
	stdin       *readline.CancelableStdin
	historyFile string
}

func newInteractiveInput(sh *shell.Shell) (*interactiveInput, error) {
	// Readline manages the process's terminal. Custom streams use plain input.
	if !sh.Interactive || sh.Stdin != os.Stdin || sh.Stdout != os.Stdout || !readline.IsTerminal(int(os.Stdout.Fd())) {
		return nil, nil
	}
	stdin := readline.NewCancelableStdin(singleByteReader{os.Stdin})
	line, err := readline.NewEx(&readline.Config{
		Prompt:                 prompt,
		Stdin:                  stdin,
		Stdout:                 sh.Stdout,
		Stderr:                 sh.Stderr,
		AutoComplete:           shellCompleter{},
		HistoryLimit:           historyLimit,
		DisableAutoSaveHistory: true,
		// Ctrl+Z at the prompt must not suspend the parent shell.
		FuncFilterInputRune: func(r rune) (rune, bool) { return r, r != readline.CharCtrlZ },
		FuncGetWidth: func() int {
			if width := readline.GetScreenWidth(); width > 0 {
				return width
			}
			return 80
		},
	})
	if err != nil {
		stdin.Close()
		return nil, err
	}
	input := &interactiveInput{line: line, stdin: stdin}
	path, err := historyPath()
	if err == nil {
		sh.History, err = loadHistory(path)
	}
	if err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: history: %v\n", err)
	} else {
		input.historyFile = path
	}
	for _, command := range sh.History {
		line.SaveHistory(command)
	}
	return input, nil
}

func (input *interactiveInput) close() {
	input.stdin.Close()
	input.line.Close()
}

func (input *interactiveInput) read(shutdown <-chan struct{}) inputResult {
	results := make(chan inputResult, 1)
	go func() {
		line, err := input.line.Readline()
		results <- inputResult{line: line, err: err}
	}()
	select {
	case result := <-results:
		return result
	case <-shutdown:
		input.stdin.Close()
		<-results // Wait for Readline to restore the terminal before cleanup.
		return inputResult{err: context.Canceled}
	}
}

func (input *interactiveInput) remember(sh *shell.Shell, line string) {
	line = strings.TrimSpace(line)
	if line == "" || (len(sh.History) > 0 && sh.History[len(sh.History)-1] == line) {
		return
	}
	sh.History = append(sh.History, line)
	if len(sh.History) > historyLimit {
		sh.History = sh.History[1:]
	}
	input.line.SaveHistory(line)
	if err := appendHistory(input.historyFile, line); err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: history: %v\n", err)
	}
}

// Prevent the editor's buffered reader from taking input intended for a child.
// Reads occur only during a prompt and stop at Enter/Ctrl+C/Ctrl+D.
type singleByteReader struct{ io.Reader }

func (reader singleByteReader) Read(buffer []byte) (int, error) {
	if len(buffer) > 1 {
		buffer = buffer[:1]
	}
	return reader.Reader.Read(buffer)
}
