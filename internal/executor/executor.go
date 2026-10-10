package executor

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/omar-shahieen/Gbash/internal/shell"
)

func ExecCommand(sh *shell.Shell, cmd shell.Command) int {

	proc := exec.Command(cmd.Name, cmd.Args...)

	proc.Stdin = sh.Stdin
	proc.Stdout = sh.Stdout
	proc.Stderr = sh.Stderr

	err := proc.Run()
	if err == nil {
		return 0
	}

	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		fmt.Fprintf(sh.Stderr, "minishell: %s: command not found\n", cmd.Name)
		return 127
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	fmt.Fprintf(sh.Stderr, "minishell: %s: %v\n", cmd.Name, err)
	return 126
}
