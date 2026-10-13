package main

import (
	"bufio"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/omar-shahieen/Gbash/internal/executor"
	"github.com/omar-shahieen/Gbash/internal/shell"
)

const prompt = "minishell> "

func main() {
	os.Exit(run(shell.New()))
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
