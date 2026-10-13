package executor

import (
	"fmt"

	"github.com/omar-shahieen/Gbash/internal/builtins"
	"github.com/omar-shahieen/Gbash/internal/shell"
)

// Execute is the entry point the shell loop calls for one parsed line.
func Execute(sh *shell.Shell, pipeline shell.Pipeline) int {
	drainJobEvents(sh)
	if pipeline.Background {
		for _, cmd := range pipeline.Commands {
			_, builtin := builtins.Lookup(cmd.Name)
			if builtin || cmd.Name == "fg" || cmd.Name == "bg" || cmd.Name == "wait" {
				fmt.Fprintf(sh.Stderr,
					"minishell: %s: background execution not supported yet\n", cmd.Name)
				return 1
			}
		}
	}

	if len(pipeline.Commands) > 1 {
		for _, cmd := range pipeline.Commands {
			if cmd.Name == "fg" {
				fmt.Fprintln(sh.Stderr, "minishell: fg: cannot run in a pipeline")
				return 1
			}
			if cmd.Name == "bg" {
				fmt.Fprintln(sh.Stderr, "minishell: bg: cannot run in a pipeline")
				return 1
			}
			if cmd.Name == "wait" {
				fmt.Fprintln(sh.Stderr, "minishell: wait: cannot run in a pipeline")
				return 1
			}
		}
	}

	if len(pipeline.Commands) == 1 &&
		pipeline.Commands[0].Name == "fg" {
		return executeFg(sh, pipeline.Commands[0])
	}
	if len(pipeline.Commands) == 1 &&
		pipeline.Commands[0].Name == "bg" {
		return executeBg(sh, pipeline.Commands[0])
	}
	if len(pipeline.Commands) == 1 &&
		pipeline.Commands[0].Name == "wait" {
		return executeWait(sh, pipeline.Commands[0])
	}

	if len(pipeline.Commands) == 1 {
		cmd := pipeline.Commands[0]

		if _, ok := builtins.Lookup(cmd.Name); ok {
			finish := startCommand(
				sh,
				cmd,
				sh.Stdin,
				sh.Stdout,
				sh.Stderr,
				&ProcessGroup{},
				nil,
				0,
				0,
			)
			return finish()
		}
	}

	finish := startPipeline(sh, pipeline)

	return finish()
}
