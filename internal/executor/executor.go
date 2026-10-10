package executor

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/omar-shahieen/Gbash/internal/builtins"
	"github.com/omar-shahieen/Gbash/internal/shell"
)

// ExecCommand runs an external program with the given streams and returns
// its exit status.
func runExternalCommand(sh *shell.Shell, cmd shell.Command, in io.Reader, out, errOut io.Writer) int {
	proc := exec.Command(cmd.Name, cmd.Args...)
	proc.Stdin = in
	proc.Stdout = out
	proc.Stderr = errOut

	err := proc.Run()
	if err == nil {
		return 0
	}

	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		fmt.Fprintf(errOut, "minishell: %s: command not found\n", cmd.Name)
		return 127
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	fmt.Fprintf(errOut, "minishell: %s: %v\n", cmd.Name, err)
	return 126
}

// ExecPipeline runs every command of the pipeline concurrently, connected by
// OS pipes, and returns the exit status of the last command.
func executePipeline(sh *shell.Shell, pipeline shell.Pipeline) int {
	cmds := pipeline.Commands
	n := len(cmds)

	switch n {
	case 0:
		return 0
	case 1:
		return runCommand(sh, cmds[0], sh.Stdin, sh.Stdout, sh.Stderr)
	}

	ins := make([]io.Reader, n)
	outs := make([]io.Writer, n)
	// Each pipe end is owned by exactly one stage, which closes it when done.
	// This is what lets the neighbouring stage see EOF (or SIGPIPE).
	owned := make([][]io.Closer, n)

	ins[0] = sh.Stdin
	outs[n-1] = sh.Stdout

	for i := 0; i < n-1; i++ {
		r, w, err := os.Pipe()
		if err != nil {
			for _, group := range owned {
				for _, c := range group {
					_ = c.Close()
				}
			}
			fmt.Fprintf(sh.Stderr, "minishell: pipe: %v\n", err)
			return 1
		}
		outs[i] = w
		ins[i+1] = r
		owned[i] = append(owned[i], w)
		owned[i+1] = append(owned[i+1], r)
	}

	statuses := make([]int, n)
	var wg sync.WaitGroup
	wg.Add(n)

	for i := range cmds {
		go func(i int) {
			defer wg.Done()
			defer func() {
				for _, c := range owned[i] {
					_ = c.Close()
				}
			}()
			statuses[i] = runCommand(sh, cmds[i], ins[i], outs[i], sh.Stderr)
		}(i)
	}

	wg.Wait()
	return statuses[n-1]
}

// runCommand applies the command's redirections, then runs it as a builtin
// or an external program. in/out/errOut are the defaults (shell streams or
// pipe ends); redirections override them.
func runCommand(sh *shell.Shell, cmd shell.Command, in io.Reader, out, errOut io.Writer) int {
	var files []*os.File

	defer func() {
		for _, f := range files {
			_ = f.Close()
		}
	}()

	for _, r := range cmd.Redirections {
		f, err := openRedirection(r)
		if err != nil {
			fmt.Fprintf(sh.Stderr, "minishell: %s: %v\n", r.Filename, err)
			return 1
		}
		files = append(files, f)

		switch r.FD {
		case 0:
			in = f
		case 1:
			out = f
		case 2:
			errOut = f
		}
	}

	// `> f` alone: files were created above, nothing to run.
	if cmd.Name == "" {
		return 0
	}

	if fn, ok := builtins.Lookup(cmd.Name); ok {
		return fn(sh, cmd.Args, out, errOut)
	}
	return runExternalCommand(sh, cmd, in, out, errOut)
}

func openRedirection(r shell.Redirection) (*os.File, error) {
	switch r.Mode {
	case shell.ModeRead:
		return os.Open(r.Filename)
	case shell.ModeAppend:
		return os.OpenFile(r.Filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	default: // ModeTruncate
		return os.OpenFile(r.Filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	}
}

// Execute is the entry point the shell loop calls for one parsed line.
func Execute(sh *shell.Shell, pipeline shell.Pipeline) int {
	return executePipeline(sh, pipeline)
}
