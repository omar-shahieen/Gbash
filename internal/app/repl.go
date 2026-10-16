package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"syscall"

	"github.com/chzyer/readline"

	"github.com/omar-shahieen/gbash/internal/executor"
	"github.com/omar-shahieen/gbash/internal/shell"
)

const prompt = "minishell> "

type inputResult struct {
	line string
	err  error
}

// runREPL owns the sequence: prompt, read, execute, repeat.
func runREPL(sh *shell.Shell, reader *bufio.Reader, running *atomic.Bool, editor *interactiveInput) int {
	for {
		if shutdownRequested(sh.Shutdown) {
			return 128 + int(syscall.SIGTERM)
		}
		executor.NotifyCompletedJobs(sh)
		var input inputResult
		if editor != nil {
			input = editor.read(sh.Shutdown)
		} else {
			if sh.Interactive {
				fmt.Fprint(sh.Stdout, prompt)
			}
			input = readLine(reader, sh.Shutdown)
		}
		if shutdownRequested(sh.Shutdown) {
			return 128 + int(syscall.SIGTERM)
		}
		if errors.Is(input.err, readline.ErrInterrupt) {
			sh.LastStatus = 128 + int(syscall.SIGINT)
			continue
		}
		if input.err != nil && !errors.Is(input.err, io.EOF) {
			fmt.Fprintln(sh.Stderr, "minishell: read error:", input.err)
			return 1
		}

		if editor != nil {
			editor.remember(sh, input.line)
		}
		executeLine(sh, input.line, running)
		if shutdownRequested(sh.Shutdown) {
			return 128 + int(syscall.SIGTERM)
		}
		if sh.Exiting {
			return sh.ExitCode
		}
		if errors.Is(input.err, io.EOF) {
			if sh.Interactive {
				fmt.Fprintln(sh.Stdout)
			}
			return sh.LastStatus
		}
	}
}

// readLine starts one read per prompt, never a continuous input reader.
// The buffered channel lets the worker finish even if shutdown wins.
func readLine(reader *bufio.Reader, shutdown <-chan struct{}) inputResult {
	results := make(chan inputResult, 1)
	go func() {
		line, err := reader.ReadString('\n')
		results <- inputResult{line: line, err: err}
	}()
	select {
	case <-shutdown:
		return inputResult{err: context.Canceled}
	case result := <-results:
		return result
	}
}
