package shell

type ProcessState string

const (
	ProcessRunning ProcessState = "running"
	ProcessStopped ProcessState = "stopped"
	ProcessDone    ProcessState = "done"
)

type Process struct {
	Command    Command
	Stage      int
	PID        int
	State      ProcessState
	ExitStatus int
}

type Job struct {
	JobID         int
	PGID          int
	Pipeline      Pipeline
	Stopped       bool
	Processes     []*Process
	CommandText   string
	Background    bool
	StageDone     []bool
	StageStatuses []int
}
type JobEvent struct {
	JobID      int
	Stage      int
	PID        int
	State      ProcessState
	ExitStatus int
	Err        error
}

func (j *Job) State() ProcessState {
	live := len(j.Processes)
	for _, p := range j.Processes {
		switch p.State {
		case ProcessDone:
			live--
		case ProcessStopped:
			// Keep counting this as a live process.
		default:
			return ProcessRunning
		}
	}

	if live > 0 {
		return ProcessStopped
	}
	return ProcessDone
}
