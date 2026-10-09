package builtins

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/omar-shahieen/Gbash/internal/shell"
)

// Func is the built-in contract: state, args (without the command name),
// and explicit output destinations so redirection works later.
type Func func(sh *shell.Shell, args []string, out, errOut io.Writer) int

// Populated in init() rather than in a package-level initializer, because
// "type" reads this table and would otherwise create an initialization cycle.
var table map[string]Func

func init() {
	table = map[string]Func{
		"cd":     cdCmd,
		"pwd":    pwdCmd,
		"echo":   echoCmd,
		"export": exportCmd,
		"unset":  unsetCmd,
		"exit":   exitCmd,
		"type":   typeCmd,
	}
}

func Lookup(name string) (Func, bool) {
	fn, ok := table[name]
	return fn, ok
}

func isValidName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		isLetter := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if !isLetter && (i == 0 || !isDigit) {
			return false
		}
	}
	return true
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

func exportCmd(sh *shell.Shell, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		env := os.Environ()
		sort.Strings(env)
		for _, kv := range env {
			k, v, _ := strings.Cut(kv, "=")
			fmt.Fprintf(out, "export %s=%q\n", k, v)
		}
		return 0
	}

	status := 0
	for _, arg := range args {
		name, value, hasValue := strings.Cut(arg, "=")
		if !isValidName(name) {
			fmt.Fprintf(errOut, "minishell: export: `%s': not a valid identifier\n", arg)
			status = 1
			continue
		}
		if !hasValue {
			continue // bare NAME: nothing to set until shell variables exist
		}
		if err := os.Setenv(name, value); err != nil {
			fmt.Fprintf(errOut, "minishell: export: %v\n", err)
			status = 1
		}
	}
	return status
}

func unsetCmd(sh *shell.Shell, args []string, out, errOut io.Writer) int {
	status := 0
	for _, name := range args {
		if !isValidName(name) {
			fmt.Fprintf(errOut, "minishell: unset: `%s': not a valid identifier\n", name)
			status = 1
			continue
		}
		if err := os.Unsetenv(name); err != nil {
			fmt.Fprintf(errOut, "minishell: unset: %v\n", err)
			status = 1
		}
	}
	return status
}
