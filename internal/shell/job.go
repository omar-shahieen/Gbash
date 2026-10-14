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
	Processes     []*Process
	CommandText   string
	Background    bool
	StageDone     []bool
	StageStatuses []int
	Notified      bool
}
type JobEvent struct {
	JobID      int
	Stage      int
	PID        int
	State      ProcessState
	ExitStatus int
}

// Complete includes every stage, including commands containing only redirections.
func (j *Job) Complete() bool {
	for _, done := range j.StageDone {
		if !done {
			return false
		}
	}
	return true
}

// ExitStatus is the pipeline result: the last stage's status.
func (j *Job) ExitStatus() int {
	if len(j.StageStatuses) == 0 {
		return 0
	}
	return j.StageStatuses[len(j.StageStatuses)-1]
}

func (j *Job) MarkRunning() {
	for _, process := range j.Processes {
		if process.State == ProcessStopped {
			process.State = ProcessRunning
		}
	}
}

// ApplyEvent is called by the shell's event receiver, never by child workers.
func (j *Job) ApplyEvent(event JobEvent) {
	if event.JobID != j.JobID || event.Stage < 0 ||
		event.Stage >= len(j.StageDone) || event.Stage >= len(j.StageStatuses) {
		return
	}
	for _, process := range j.Processes {
		if process.Stage == event.Stage {
			process.State = event.State
			if event.State == ProcessDone {
				process.ExitStatus = event.ExitStatus
			}
			break
		}
	}
	if event.State == ProcessDone {
		j.StageDone[event.Stage] = true
		j.StageStatuses[event.Stage] = event.ExitStatus
	}
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
