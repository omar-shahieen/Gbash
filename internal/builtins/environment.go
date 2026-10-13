package builtins

import (
	"fmt"
	"github.com/omar-shahieen/Gbash/internal/shell"
	"io"
	"os"
	"sort"
	"strings"
)

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
