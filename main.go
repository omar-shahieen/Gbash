package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/omar-shahieen/Gbash/internal/executor"
	"github.com/omar-shahieen/Gbash/internal/parser"
	"github.com/omar-shahieen/Gbash/internal/shell"
)

func run(sh *shell.Shell) int {
	reader := bufio.NewReader(sh.Stdin)

	for {
		fmt.Fprint(sh.Stdout, "minishell> ")

		line, err := reader.ReadString('\n')
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
				sh.LastStatus = executor.Execute(sh, pipeline)
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
