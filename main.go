package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func tokenize(input string) []string {
	return strings.Fields(input)
}

type Command struct {
	Name      string
	Arguments []string
}

var lastStatus int // package-level for now; $? and && will read it later

func runLine(line string) int {
	tokens := tokenize(line)

	// Empty or whitespace-only input is not a command.
	if len(tokens) == 0 {
		return 0
	}

	cmd := Command{
		Name:      tokens[0],
		Arguments: tokens[1:],
	}

	proc := exec.Command(cmd.Name, cmd.Arguments...)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr

	err := proc.Run()
	if err == nil {
		return 0
	}

	var execErr *exec.Error
	if errors.As(err, &execErr) &&
		errors.Is(execErr.Err, exec.ErrNotFound) {
		fmt.Fprintf(
			os.Stderr,
			"minishell: %s: command not found\n",
			cmd.Name,
		)
		return 127
	}

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode()
	}

	fmt.Fprintf(os.Stderr, "minishell: %s: %v\n", cmd.Name, err)
	return 126
}
func main() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("minishell> ")

		line, err := reader.ReadString('\n')

		// A real read error: report to stderr and exit with failure.
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Fprintln(os.Stderr, "minishell: read error:", err)
			os.Exit(1)
		}
		atEOF := err != nil // only io.EOF can reach this point

		line = strings.TrimSpace(line)

		if line == "exit" {
			return
		}
		if line != "" {
			lastStatus = runLine(line)
		}

		// Handle any final line first, then stop on EOF.
		if atEOF {
			fmt.Println() // so the terminal prompt starts on a fresh line
			return
		}
	}
}
