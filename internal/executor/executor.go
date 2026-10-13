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
			if builtin || isJobControl(cmd.Name) {
				fmt.Fprintf(sh.Stderr,
					"minishell: %s: background execution not supported yet\n", cmd.Name)
				return 1
			}
		}
	}

	if len(pipeline.Commands) > 1 {
		for _, cmd := range pipeline.Commands {
			if isJobControl(cmd.Name) {
				fmt.Fprintf(sh.Stderr, "minishell: %s: cannot run in a pipeline\n", cmd.Name)
				return 1
			}
		}
	}

	if len(pipeline.Commands) == 1 {
		cmd := pipeline.Commands[0]
		if isJobControl(cmd.Name) {
			return executeJobControl(sh, cmd)
		}

		if builtin, ok := builtins.Lookup(cmd.Name); ok {
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

func isJobControl(name string) bool {
	switch name {
	case "fg", "bg", "wait":
		return true
	default:
		return false
	}
}
