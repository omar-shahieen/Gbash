package shell

import (
	"io"
	"os"
)

type RedirMode int

const (
	ModeRead RedirMode = iota
	ModeTruncate
	ModeAppend
)

type Redirection struct {
	FD       int // 0 = stdin, 1 = stdout, 2 = stderr
	Mode     RedirMode
	Filename string
}

type Command struct {
	Name         string
	Args         []string
	Redirections []Redirection
}

type Pipeline struct {
	Commands []Command
}

// Shell holds all mutable state that built-ins and the executor need.
type Shell struct {
	LastStatus int

	// Set by the exit built-in; the main loop checks these.
	Exiting bool

	ExitCode int

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func New() *Shell {
	return &Shell{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}
