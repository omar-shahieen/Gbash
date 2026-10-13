package builtins

import (
	"github.com/omar-shahieen/Gbash/internal/shell"
	"io"
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
		"jobs":   jobsCmd,
	}
}

func Lookup(name string) (Func, bool) {
	fn, ok := table[name]
	return fn, ok
}
