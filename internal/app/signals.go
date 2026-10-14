package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/omar-shahieen/gbash/internal/shell"
)

// setupShutdownSignals turns SIGTERM into a shared shutdown notification.
func setupShutdownSignals(sh *shell.Shell) func() {
	if sh.Interactive {
		signal.Ignore(syscall.SIGTTOU)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	sh.Shutdown = ctx.Done()
	return func() {
		stop()
		if sh.Interactive {
			signal.Reset(syscall.SIGTTOU)
		}
	}
}

func shutdownRequested(shutdown <-chan struct{}) bool {
	select {
	case <-shutdown:
		return true
	default:
		return false
	}
}

// handlePromptSignals preserves the shell and redraws the prompt on Ctrl+C.
// A foreground job receives terminal signals through its own process group.
func handlePromptSignals(sh *shell.Shell, running *atomic.Bool) func() {
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(signals, os.Interrupt, syscall.SIGTSTP)
	go func() {
		for {
			select {
			case <-done:
				return
			case sig := <-signals:
				if sig == os.Interrupt && !running.Load() {
					fmt.Fprint(sh.Stdout, "\n"+prompt)
				}
			}
		}
	}()
	return func() {
		signal.Stop(signals)
		close(done)
	}
}
