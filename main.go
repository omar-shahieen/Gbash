package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/omar-shahieen/Gbash/internal/builtins"
	"github.com/omar-shahieen/Gbash/internal/executor"
	"github.com/omar-shahieen/Gbash/internal/parser"
	"github.com/omar-shahieen/Gbash/internal/shell"
)

func execute(sh *shell.Shell, cmd shell.Command) int {
	if fn, ok := builtins.Lookup(cmd.Name); ok {
		return fn(sh, cmd.Args, sh.Stdout, sh.Stderr)
	}
	return executor.ExecCommand(sh, cmd)
}

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

		if tokens := parser.Tokenize(strings.TrimSpace(line)); len(tokens) > 0 {
			cmd := shell.Command{Name: tokens[0], Args: tokens[1:]}
			sh.LastStatus = execute(sh, cmd)

			if sh.Exiting {
				return sh.ExitCode
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
