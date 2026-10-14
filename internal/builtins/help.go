package builtins

import (
	"fmt"
	"io"
	"strings"

	"github.com/omar-shahieen/gbash/internal/shell"
)

func helpCmd(sh *shell.Shell, args []string, out, errOut io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(errOut, "minishell: help: no arguments supported")
		return 1
	}
	fmt.Fprintln(out, "gbash builtins:", strings.Join(Names(), ", "))
	fmt.Fprintln(out, "Operators: |  &&  ||  ;  &  <  >  >>  2>  2>>")
	fmt.Fprintln(out, "Keys: Tab complete, Up/Down history, Ctrl+C cancel, Ctrl+D exit, Ctrl+L clear")
	return 0
}

func historyCmd(sh *shell.Shell, args []string, out, errOut io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(errOut, "minishell: history: no arguments supported")
		return 1
	}
	for i, line := range sh.History {
		fmt.Fprintf(out, "%5d  %s\n", i+1, line)
	}
	return 0
}
