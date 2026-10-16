package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"syscall"

	"github.com/omar-shahieen/gbash/internal/builtins"
	"github.com/omar-shahieen/gbash/internal/executor"
	"github.com/omar-shahieen/gbash/internal/parser"
	"github.com/omar-shahieen/gbash/internal/shell"
)

// runBuiltinChild executes one built-in in this child process, without a REPL.
func runBuiltinChild(args []string, history []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "minishell: no builtin specified")
		return 1
	}
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "minishell: requires at least two arguments")
		return 1
	}
	lastStatus, err := strconv.Atoi(args[0])
	if err != nil || lastStatus < 0 || lastStatus > 255 {
		fmt.Fprintf(os.Stderr, "minishell: %s: not a valid status\n", args[0])
		return 1
	}
	cmdName := args[1]
	builtin, ok := builtins.Lookup(cmdName)
	if !ok {
		fmt.Fprintf(os.Stderr, "minishell: %s: not a builtin\n", cmdName)
		return 1
	}
	sh := shell.New()
	sh.History = history
	sh.LastStatus = lastStatus
	return builtin(sh, args[2:], sh.Stdout, sh.Stderr)
}

func runChainChild(args []string, history []string) int {
	if len(args) != 3 {
		fmt.Fprintln(os.Stderr, "minishell: chain child: expected status, background PID, and entries")
		return 1
	}
	lastStatus, err := strconv.Atoi(args[0])
	if err != nil || lastStatus < 0 || lastStatus > 255 {
		fmt.Fprintf(os.Stderr, "minishell: chain child: invalid status %q\n", args[0])
		return 1
	}
	pid, err := strconv.Atoi(args[1])
	if err != nil || pid < 0 {
		fmt.Fprintf(os.Stderr, "minishell: chain child: invalid background PID %q\n", args[1])
		return 1
	}

	var entries []parser.ListEntry
	if err := json.Unmarshal([]byte(args[2]), &entries); err != nil {
		fmt.Fprintf(os.Stderr, "minishell: chain child: invalid entries: %v\n", err)
		return 1
	}
	sh := shell.New()
	sh.History = history
	sh.LastStatus = lastStatus
	sh.LastBackgroundPID = pid
	sh.ChildPGID = syscall.Getpgrp()

	var running atomic.Bool
	executeEntries(sh, entries, "", false, &running)
	if sh.Exiting {
		return sh.ExitCode
	}
	return sh.LastStatus
}

func startBackgroundChain(
	sh *shell.Shell,
	chain parser.CommandChain,
	commandText string,
) int {
	payload, err := json.Marshal(chain.Entries)
	if err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: chain child: failed to encode entries: %v\n", err)
		return 1
	}
	path, err := os.Executable()
	if err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: chain child: failed to get executable path: %v\n", err)
		return 1
	}
	command := shell.Command{
		Name:           path,
		InheritHistory: true,
		Args: []string{
			"--chain-child",
			strconv.Itoa(sh.LastStatus),
			strconv.Itoa(sh.LastBackgroundPID),
			string(payload),
		},
	}
	return executor.Execute(sh, shell.Pipeline{
		Commands:    []shell.Command{command},
		Background:  true,
		CommandText: commandText,
	})
}
