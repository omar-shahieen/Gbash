package app

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"github.com/omar-shahieen/gbash/internal/executor"
	"github.com/omar-shahieen/gbash/internal/parser"
	"github.com/omar-shahieen/gbash/internal/shell"
)

// executeLine parses one line and records its status; empty lines do nothing.
func executeLine(sh *shell.Shell, line string, running *atomic.Bool) {
	text := strings.TrimSpace(line)
	tokens, err := parser.Tokenize(text)
	if err != nil {
		reportParseError(sh, err)
		return
	}
	if len(tokens) == 0 {
		return
	}

	commandChains, err := parser.ParseChains(tokens)
	if err != nil {
		reportParseError(sh, err)
		return
	}

	for _, chain := range commandChains {
		if sh.Exiting || shutdownRequested(sh.Shutdown) {
			return
		}
		if chain.Background && len(chain.Entries) > 1 {
			running.Store(true)
			sh.LastStatus = startBackgroundChain(sh, chain, text)
			running.Store(false)
		} else {
			executeEntries(sh, chain.Entries, text, chain.Background, running)
		}
	}
}

func executeEntries(
	sh *shell.Shell,
	entries []parser.ListEntry,
	commandText string,
	background bool,
	running *atomic.Bool,
) {
	for _, entry := range entries {
		if sh.Exiting || shutdownRequested(sh.Shutdown) {
			return
		}
		if !shouldRun(entry.Condition, sh.LastStatus) {
			continue
		}
		entryTokens, err := parser.ExpandTokens(entry.Tokens, os.Getenv, sh.LastStatus, sh.LastBackgroundPID)
		if err != nil {
			reportParseError(sh, err)
			return
		}
		pipeline, err := parser.ParsePipeline(entryTokens)
		if err != nil {
			reportParseError(sh, err)
			return
		}
		pipeline.CommandText = commandText
		pipeline.Background = background
		running.Store(true)
		sh.LastStatus = executor.Execute(sh, pipeline)
		running.Store(false)
	}
}

func shouldRun(condition parser.RunCondition, lastStatus int) bool {
	switch condition {
	case parser.RunAlways:
		return true
	case parser.RunOnSuccess:
		return lastStatus == 0
	case parser.RunOnFailure:
		return lastStatus != 0
	default:
		return false
	}
}

func reportParseError(sh *shell.Shell, err error) {
	fmt.Fprintf(sh.Stderr, "minishell: %v\n", err)
	sh.LastStatus = 2
}
