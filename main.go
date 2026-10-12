package main

import (
	"bufio"
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

func run(sh *shell.Shell) int {
	if err := executor.InitTerminal(sh); err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: terminal: %v\n", err)
		return 1
	}

	if sh.Interactive {
		signal.Ignore(syscall.SIGTTOU)
		defer signal.Reset(syscall.SIGTTOU)
	}
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
		fmt.Fprint(sh.Stdout, "minishell> ")

		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Fprintln(sh.Stderr, "minishell: read error:", err)
			time.Sleep(100 * time.Second) // Give the shell a moment to print the last status before exiting.
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
				running.Store(true)
				sh.LastStatus = executor.Execute(sh, pipeline)
				running.Store(false)
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
