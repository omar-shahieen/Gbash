package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/omar-shahieen/gbash/internal/executor"
	"github.com/omar-shahieen/gbash/internal/parser"
	"github.com/omar-shahieen/gbash/internal/shell"
)

type inputResult struct {
	line string
	err  error
}

// runREPL owns the sequence: prompt, read, execute, repeat.
func runREPL(sh *shell.Shell, reader *bufio.Reader, running *atomic.Bool) int {
	for {
		if shutdownRequested(sh.Shutdown) {
			return 128 + int(syscall.SIGTERM)
		}
		executor.NotifyCompletedJobs(sh)
		fmt.Fprint(sh.Stdout, prompt)

		input := readLine(reader, sh.Shutdown)
		if shutdownRequested(sh.Shutdown) {
			return 128 + int(syscall.SIGTERM)
		}
		if input.err != nil && !errors.Is(input.err, io.EOF) {
			fmt.Fprintln(sh.Stderr, "minishell: read error:", input.err)
			return 1
		}

		executeLine(sh, input.line, running)
		if shutdownRequested(sh.Shutdown) {
			return 128 + int(syscall.SIGTERM)
		}
		if sh.Exiting {
			return sh.ExitCode
		}
		if errors.Is(input.err, io.EOF) {
			fmt.Fprintln(sh.Stdout)
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

// executeLine parses one line and records its status; empty lines do nothing.
func executeLine(sh *shell.Shell, line string, running *atomic.Bool) {
	text := strings.TrimSpace(line)
	tokens, err := parser.Tokenize(text)
	if err != nil {
		reportParseError(sh, err)
		return
	}
	if len(tokens) == 0 {
		return
	}

	list, err := parser.ParseList(tokens)
	if err != nil {
		reportParseError(sh, err)
		return
	}
	for _, entry := range list {
		if sh.Exiting || shutdownRequested(sh.Shutdown) {
			return
		}
		if !shouldRun(entry.Condition, sh.LastStatus) {
			continue
		}
		entryTokens, err := parser.ExpandTokens(entry.Tokens, os.Getenv, sh.LastStatus, sh.LastBackgroundPID)
		if err != nil {
			reportParseError(sh, err)
			return
		}
		pipeline, err := parser.ParsePipeline(entryTokens)
		if err != nil {
			reportParseError(sh, err)
			return
		}
		pipeline.CommandText = text
		running.Store(true)
		sh.LastStatus = executor.Execute(sh, pipeline)
		running.Store(false)
	}
}

func shouldRun(condition parser.RunCondition, lastStatus int) bool {
	switch condition {
	case parser.RunAlways:
		return true
	case parser.RunOnSuccess:
		return lastStatus == 0
	case parser.RunOnFailure:
		return lastStatus != 0
	default:
		return false
	}
}

func reportParseError(sh *shell.Shell, err error) {
	fmt.Fprintf(sh.Stderr, "minishell: %v\n", err)
	sh.LastStatus = 2
}
