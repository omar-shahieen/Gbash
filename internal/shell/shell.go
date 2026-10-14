package shell

import (
	"io"
	"maps"
	"os"
	"slices"
)

// Shell holds all mutable state that built-ins and the executor need.
type Shell struct {
	LastStatus int

	// Set by the exit built-in; the main loop checks these.
	Exiting bool

	ExitCode int

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	ForegroundJob     *Job
	Jobs              map[int]*Job
	NextJobID         int
	LastBackgroundPID int
	JobEvents         chan JobEvent
	Shutdown          <-chan struct{}

	Interactive bool
	Terminal    *os.File
	ShellPGID   int
}

// JobIDs gives callers a stable order and a snapshot safe to iterate while removing jobs.
func (sh *Shell) JobIDs() []int {
	ids := slices.Collect(maps.Keys(sh.Jobs))
	slices.Sort(ids)
	return ids
}

func New() *Shell {
	return &Shell{
		Stdin:         os.Stdin,
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
		ForegroundJob: nil,
		Jobs:          make(map[int]*Job),
		NextJobID:     1,
		JobEvents:     make(chan JobEvent, 64),
	}
}
