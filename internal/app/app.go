package app

import (
	"bufio"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/omar-shahieen/gbash/internal/executor"
	"github.com/omar-shahieen/gbash/internal/shell"
)

// Main selects a child mode or starts the interactive shell.
// args excludes the executable name. Returning allows cleanup before os.Exit.
func Main(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "--builtin-child":
			return runBuiltinChild(args[1:])
		case "--chain-child":
			return runChainChild(args[1:])
		}
	}
	return run(shell.New())
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
	input, err := newInteractiveInput(sh)
	if err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: input: %v\n", err)
		return 1
	}
	if input != nil {
		defer input.close()
		printBanner(sh.Stdout)
	}

	var running atomic.Bool
	stopPromptSignals := handlePromptSignals(sh, &running)
	defer stopPromptSignals()

	return runREPL(sh, bufio.NewReader(sh.Stdin), &running, input)
}

func shutdownJobs(sh *shell.Shell) {
	if err := executor.ShutdownJobs(sh, 2*time.Second); err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: shutdown: %v\n", err)
	}
}
