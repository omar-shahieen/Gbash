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
	cleanArgs, redirections, err := parser.ParseRedirections(cmd.Args)
	if err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: %v\n", err)
		return 2
	}

	// Save the shell's original streams.
	originalStdin := sh.Stdin
	originalStdout := sh.Stdout
	originalStderr := sh.Stderr

	var openedFiles []*os.File
	defer func() {
		sh.Stdin = originalStdin
		sh.Stdout = originalStdout
		sh.Stderr = originalStderr

		for _, f := range openedFiles {
			if closeErr := f.Close(); closeErr != nil {
				fmt.Fprintf(
					originalStderr,
					"minishell: close %s: %v\n",
					f.Name(),
					closeErr,
				)
			}
		}
	}()

	for _, redir := range redirections {
		var f *os.File
		// Process each redirection
		switch redir.Operator {
		case ">":
			f, err = os.OpenFile(redir.Filename,
				os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)

			if err == nil {
				sh.Stdout = f

			}
		case ">>":
			f, err = os.OpenFile(redir.Filename,
				os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {

				sh.Stdout = f
			}
		case "2>":
			f, err = os.OpenFile(redir.Filename,
				os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err == nil {

				sh.Stderr = f
			}
		case "2>>":
			f, err = os.OpenFile(redir.Filename,
				os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {

				sh.Stderr = f
			}
		case "<":
			f, err = os.Open(redir.Filename)
			if err == nil {

				sh.Stdin = f
			}
		}
		if err != nil {
			fmt.Fprintf(
				originalStderr,
				"minishell: %s: %v\n",
				redir.Filename,
				err,
			)
			return 1
		}

		openedFiles = append(openedFiles, f)

	}
	cmd.Args = cleanArgs
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
