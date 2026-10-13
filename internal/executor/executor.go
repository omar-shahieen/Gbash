package executor

import (
	"fmt"

	"github.com/omar-shahieen/gbash/internal/builtins"
	"github.com/omar-shahieen/gbash/internal/shell"
)

// Execute is the entry point the shell loop calls for one parsed line.
func Execute(sh *shell.Shell, pipeline shell.Pipeline) int {
	drainJobEvents(sh)

	if pipeline.Background {
		for _, cmd := range pipeline.Commands {
			if cmd.Name == "jobs" {
				fmt.Fprintf(sh.Stderr, "minishell: jobs: background execution not supported\n")
				return 1
			}
			if builtins.IsJobControl(cmd.Name) {
				fmt.Fprintf(sh.Stderr,
					"minishell: %s: background execution not supported for job control commands\n", cmd.Name)
				return 1
			}
		}
	}

	if len(pipeline.Commands) > 1 {
		for _, cmd := range pipeline.Commands {
			if cmd.Name == "jobs" {
				fmt.Fprintf(sh.Stderr, "minishell: jobs: cannot run in a pipeline\n")
				return 1
			}
			if builtins.IsJobControl(cmd.Name) {
				fmt.Fprintf(sh.Stderr, "minishell: %s: cannot run in a pipeline\n", cmd.Name)
				return 1
			}
		}
	}

	if len(pipeline.Commands) == 1 {
		cmd := pipeline.Commands[0]
		if builtins.IsJobControl(cmd.Name) {
			return executeJobControl(sh, cmd)
		}

		if builtin, ok := builtins.Lookup(cmd.Name); ok && !pipeline.Background {
			streams, err := prepareRedirections(cmd, shellStreams(sh))
			if err != nil {
				fmt.Fprintf(sh.Stderr, "minishell: %v\n", err)
				return 1
			}
			defer streams.close()
			return builtin(sh, cmd.Args, streams.stdout, streams.stderr)
		}
	}

	return executePipeline(sh, pipeline)
}
