package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/omar-shahieen/Gbash/internal/executor"
	"github.com/omar-shahieen/Gbash/internal/parser"
	"github.com/omar-shahieen/Gbash/internal/shell"
)

type inputResult struct {
	line string
	err  error
}

func run(sh *shell.Shell) int {
	if err := executor.InitTerminal(sh); err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: terminal: %v\n", err)
		return 1
	}
	if sh.Interactive {
		signal.Ignore(syscall.SIGTTOU)
		defer signal.Reset(syscall.SIGTTOU)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()
	sh.Shutdown = ctx.Done()

	defer func() {
		if err := executor.ShutdownJobs(sh, 2*time.Second); err != nil {
			fmt.Fprintf(sh.Stderr, "minishell: shutdown: %v\n", err)
		}
	}()
	reader := bufio.NewReader(sh.Stdin)

	// Create a channel to receive signals.
	var running atomic.Bool

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTSTP)
	defer signal.Stop(sigChan)

	go func() {
		for sig := range sigChan {
			switch sig {
			case os.Interrupt:
				if !running.Load() {
					// Ctrl+C at the prompt: the tty already discarded the typed line,
					// so just start a fresh prompt.
					fmt.Fprint(sh.Stdout, "\nminishell> ")
				}
			// If a command is running, do nothing: the child got SIGINT
			// from the tty and will die on its own.
			case syscall.SIGTSTP:
				// Ctrl+Z
			}
		}
	}()

	for {
		if ctx.Err() != nil {
			return 128 + int(syscall.SIGTERM)
		}
		executor.NotifyCompletedJobs(sh)
		fmt.Fprint(sh.Stdout, "minishell> ")

		// Read one line per prompt; foreground commands retain access to stdin.
		inputResults := make(chan inputResult, 1)
		go func() {
			line, err := reader.ReadString('\n')
			// Buffer the result so shutdown cannot leave this sender blocked.
			inputResults <- inputResult{line: line, err: err}
		}()

		var input inputResult
		select {
		case <-sh.Shutdown:
			return 128 + int(syscall.SIGTERM)
		case input = <-inputResults:
		}
		if ctx.Err() != nil {
			return 128 + int(syscall.SIGTERM)
		}
		line, err := input.line, input.err
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Fprintln(sh.Stderr, "minishell: read error:", err)
			return 1
		}
		atEOF := err != nil

		tokens, err := parser.Tokenize(strings.TrimSpace(line))

		if err != nil {
			fmt.Fprintf(sh.Stderr, "minishell: %v\n", err)
			sh.LastStatus = 2
		} else if len(tokens) > 0 {
			pipeline, err := parser.ParsePipeline(tokens)
			if err != nil {
				fmt.Fprintf(sh.Stderr, "minishell: %v\n", err)
				sh.LastStatus = 2
			} else {
				pipeline.CommandText = strings.TrimSpace(line)
				running.Store(true)
				sh.LastStatus = executor.Execute(sh, pipeline)
				running.Store(false)
				select {
				case <-sh.Shutdown:
					return 128 + int(syscall.SIGTERM)
				default:
				}
				if sh.Exiting {
					return sh.ExitCode
				}
			}
		}

		if atEOF {
			fmt.Fprintln(sh.Stdout)
			return sh.LastStatus
		}
	}
}

func main() {

	os.Exit(run(shell.New()))
}
