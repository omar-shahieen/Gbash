package executor

import (
	"testing"

	"github.com/omar-shahieen/gbash/internal/shell"
)

func TestRegisterJobReusesRemovedIDs(t *testing.T) {
	sh := shell.New()
	first, second := &shell.Job{}, &shell.Job{}
	registerJob(sh, first)
	registerJob(sh, second)
	if first.JobID != 1 || second.JobID != 2 || sh.Jobs[1] != first || sh.Jobs[2] != second {
		t.Fatalf("registered jobs = %+v; IDs = %d, %d", sh.Jobs, first.JobID, second.JobID)
	}
	delete(sh.Jobs, second.JobID)
	replacement := &shell.Job{}
	registerJob(sh, replacement)
	if replacement.JobID != 2 || first.JobID != 1 || sh.Jobs[1] != first {
		t.Fatalf("replacement ID = %d; first job was changed or overwritten", replacement.JobID)
	}
	delete(sh.Jobs, first.JobID)
	delete(sh.Jobs, replacement.JobID)
	next := &shell.Job{}
	registerJob(sh, next)
	if next.JobID != 1 {
		t.Fatalf("ID after clearing jobs = %d, want 1", next.JobID)
	}
}

func TestRegisterJobKeepsRetainedIDs(t *testing.T) {
	sh := shell.New()
	// Completed jobs retain their IDs until wait consumes their status.
	retained := &shell.Job{JobID: 5, StageDone: []bool{true}}
	sh.Jobs[5] = retained
	job := &shell.Job{}
	registerJob(sh, job)
	if job.JobID != 6 || sh.Jobs[5] != retained {
		t.Fatalf("new ID = %d, want 6; retained job = %+v", job.JobID, sh.Jobs[5])
	}
}
