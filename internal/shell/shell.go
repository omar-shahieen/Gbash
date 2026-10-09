package shell

import (
	"io"
	"os"
)

type Command struct {
	Name string
	Args []string
}

// Shell holds all mutable state that built-ins and the executor need.
type Shell struct {
	LastStatus int

	// Set by the exit built-in; the main loop checks these.
	Exiting  bool
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
