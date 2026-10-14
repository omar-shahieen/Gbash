package builtins

import (
	"io"
	"slices"

	"github.com/omar-shahieen/gbash/internal/shell"
)

// Func is the built-in contract: state, args (without the command name),
// and explicit output destinations for redirection.
type Func func(sh *shell.Shell, args []string, out, errOut io.Writer) int

// Populated in init() rather than in a package-level initializer, because
// "type" reads this table and would otherwise create an initialization cycle.
var table map[string]Func

func init() {
	table = map[string]Func{
		"cd":      cdCmd,
		"pwd":     pwdCmd,
		"echo":    echoCmd,
		"export":  exportCmd,
		"unset":   unsetCmd,
		"exit":    exitCmd,
		"type":    typeCmd,
		"jobs":    jobsCmd,
		"help":    helpCmd,
		"history": historyCmd,
	}
}

func Lookup(name string) (Func, bool) {
	fn, ok := table[name]
	return fn, ok
}

func IsJobControl(name string) bool {
	return name == "fg" || name == "bg" || name == "wait"
}

// Names includes the job-control builtins dispatched by the executor.
func Names() []string {
	names := []string{"fg", "bg", "wait"}
	for name := range table {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
