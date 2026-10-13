package builtins

import (
	"errors"
	"fmt"
	"github.com/omar-shahieen/Gbash/internal/shell"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func cdCmd(sh *shell.Shell, args []string, out, errOut io.Writer) int {
	if len(args) > 1 {
		fmt.Fprintln(errOut, "minishell: cd: too many arguments")
		return 1
	}

	var target string
	printDir := false

	switch {
	case len(args) == 0:
		target = os.Getenv("HOME")
		if target == "" {
			fmt.Fprintln(errOut, "minishell: cd: HOME not set")
			return 1
		}
	case args[0] == "-":
		target = os.Getenv("OLDPWD")
		if target == "" {
			fmt.Fprintln(errOut, "minishell: cd: OLDPWD not set")
			return 1
		}
		printDir = true
	default:
		target = args[0]
	}

	oldDir, _ := os.Getwd()

	if err := os.Chdir(target); err != nil {
		reason := err
		if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
			reason = pathErr.Err
		}
		fmt.Fprintf(errOut, "minishell: cd: %s: %v\n", target, reason)
		return 1
	}

	newDir, err := os.Getwd()
	if err != nil {
		newDir = target
	}
	if oldDir != "" {
		os.Setenv("OLDPWD", oldDir)
	}
	os.Setenv("PWD", newDir)

	if printDir {
		fmt.Fprintln(out, newDir)
	}
	return 0
}

func pwdCmd(sh *shell.Shell, args []string, out, errOut io.Writer) int {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(errOut, "minishell: pwd: %v\n", err)
		return 1
	}
	fmt.Fprintln(out, dir)
	return 0
}

func echoCmd(sh *shell.Shell, args []string, out, errOut io.Writer) int {
	newline := true
	if len(args) > 0 && args[0] == "-n" {
		newline = false
		args = args[1:]
	}

	fmt.Fprint(out, strings.Join(args, " "))
	if newline {
		fmt.Fprintln(out)
	}
	return 0
}

func typeCmd(sh *shell.Shell, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		return 0
	}

	status := 0
	for _, name := range args {
		if _, ok := table[name]; ok {
			fmt.Fprintf(out, "%s is a shell builtin\n", name)
			continue
		}
		if path, err := exec.LookPath(name); err == nil {
			fmt.Fprintf(out, "%s is %s\n", name, path)
			continue
		}
		fmt.Fprintf(errOut, "minishell: type: %s: not found\n", name)
		status = 1
	}
	return status
}

func exitCmd(sh *shell.Shell, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		sh.Exiting = true
		sh.ExitCode = sh.LastStatus
		return sh.LastStatus
	}

	code, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintf(errOut, "minishell: exit: %s: numeric argument required\n", args[0])
		sh.Exiting = true
		sh.ExitCode = 2
		return 2
	}

	if len(args) > 1 {
		fmt.Fprintln(errOut, "minishell: exit: too many arguments")
		return 1 // do not exit
	}

	code = ((code % 256) + 256) % 256 // wrap into 0-255, including negatives
	sh.Exiting = true
	sh.ExitCode = code
	return code
}

func jobsCmd(
	sh *shell.Shell,
	args []string,
	out, errOut io.Writer,
) int {
	if len(args) > 0 {
		fmt.Fprintln(errOut, "minishell: jobs: no arguments supported")
		return 1
	}
	for _, id := range sh.JobIDs() {
		job, exists := sh.Jobs[id]
		if !exists {
			continue
		}
		fmt.Fprintf(out, "[%d] %s pgid=%d %s\n",
			job.JobID, job.State(), job.PGID, job.CommandText)
	}
	return 0

}
