package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/omar-shahieen/gbash/internal/builtins"
	"github.com/omar-shahieen/gbash/internal/executor"
	"github.com/omar-shahieen/gbash/internal/shell"
)

const prompt = "minishell> "

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--builtin-child" {
		os.Exit(runBuiltinChild(os.Args[2:]))
	}
	os.Exit(run(shell.New()))
}

// runBuiltinChild executes one built-in in this child process, without a REPL.
func runBuiltinChild(args []string) int {

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
	sh.LastStatus = lastStatus
	return builtin(sh, args[2:], sh.Stdout, sh.Stderr)
}

// run owns startup and cleanup. Returning lets defers run before os.Exit.
func run(sh *shell.Shell) int {
	if err := executor.InitTerminal(sh); err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: terminal: %v\n", err)
		return 1
	}
	stopShutdownSignals := setupShutdownSignals(sh)
	defer stopShutdownSignals()
	defer shutdownJobs(sh)

	var running atomic.Bool
	stopPromptSignals := handlePromptSignals(sh, &running)
	defer stopPromptSignals()

	return runREPL(sh, bufio.NewReader(sh.Stdin), &running)
}

func shutdownJobs(sh *shell.Shell) {
	if err := executor.ShutdownJobs(sh, 2*time.Second); err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: shutdown: %v\n", err)
	}
}
