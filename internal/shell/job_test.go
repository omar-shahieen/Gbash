package shell

import "testing"

func TestJobState(t *testing.T) {
	cases := []struct {
		states []ProcessState
		want   ProcessState
	}{
		{nil, ProcessDone},
		{[]ProcessState{ProcessDone, ProcessDone}, ProcessDone},
		{[]ProcessState{ProcessStopped, ProcessDone}, ProcessStopped},
		{[]ProcessState{ProcessStopped, ProcessStopped}, ProcessStopped},
		{[]ProcessState{ProcessStopped, ProcessRunning}, ProcessRunning},
		{[]ProcessState{ProcessRunning, ProcessDone}, ProcessRunning},
		{[]ProcessState{""}, ProcessRunning},
	}
	for _, tc := range cases {
		job := &Job{}
		for _, state := range tc.states {
			job.Processes = append(job.Processes, &Process{State: state})
		}
		if got := job.State(); got != tc.want {
			t.Errorf("states %v: got %s, want %s", tc.states, got, tc.want)
		}
	}
}

func TestJobEventsAndCompletion(t *testing.T) {
	process := &Process{Stage: 0, PID: 42, State: ProcessRunning}
	job := &Job{JobID: 1, Processes: []*Process{process}, StageDone: make([]bool, 2), StageStatuses: make([]int, 2)}
	job.ApplyEvent(JobEvent{JobID: 2, Stage: 0, State: ProcessDone})
	job.ApplyEvent(JobEvent{JobID: 1, Stage: -1, State: ProcessDone})
	if job.StageDone[0] {
		t.Fatal("invalid event changed completion")
	}
	job.ApplyEvent(JobEvent{JobID: 1, Stage: 0, State: ProcessStopped})
	if job.Complete() || process.State != ProcessStopped {
		t.Fatal("stop was treated as completion")
	}
	job.MarkRunning()
	if process.State != ProcessRunning {
		t.Fatal("resume did not update process")
	}
	job.ApplyEvent(JobEvent{JobID: 1, Stage: 0, State: ProcessDone, ExitStatus: 7})
	if job.Complete() || process.ExitStatus != 7 {
		t.Fatal("remaining built-in stage must still complete")
	}
	job.ApplyEvent(JobEvent{JobID: 1, Stage: 1, State: ProcessDone, ExitStatus: 3})
	if !job.Complete() || job.ExitStatus() != 3 {
		t.Fatal("pipeline must use last stage status")
	}
}
