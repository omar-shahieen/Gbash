package executor

import (
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/omar-shahieen/gbash/internal/shell"
)

// Workers publish events. Only the main shell receives them and updates jobs.
func applyJobEvent(sh *shell.Shell, event shell.JobEvent) {
	if job, exists := sh.Jobs[event.JobID]; exists {
		job.ApplyEvent(event)
	}
}

func drainJobEvents(sh *shell.Shell) {
	for {
		select {
		case event := <-sh.JobEvents:
			applyJobEvent(sh, event)
		default:
			return
		}
	}
}

// receiveJobEvent waits for one event, or returns false when shutdown is requested.
func receiveJobEvent(sh *shell.Shell) bool {
	select {
	case <-sh.Shutdown:
		return false
	case event := <-sh.JobEvents:
		applyJobEvent(sh, event)
		return true
	}
}

func waitForegroundJob(sh *shell.Shell, job *shell.Job) int {
	for !job.Complete() {
		if !receiveJobEvent(sh) {
			return 128 + int(syscall.SIGTERM)
		}
		if sh.Interactive && job.State() == shell.ProcessStopped {
			return 128 + int(syscall.SIGTSTP)
		}
	}
	return job.ExitStatus()
}

func waitJob(sh *shell.Shell, job *shell.Job) int {
	if job == nil {
		return 1
	}
	for !job.Complete() {
		if !receiveJobEvent(sh) {
			return 128 + int(syscall.SIGTERM)
		}
	}
	delete(sh.Jobs, job.JobID)
	return job.ExitStatus()
}

func waitProcess(sh *shell.Shell, job *shell.Job, process *shell.Process) int {
	if job == nil || process == nil || process.Stage < 0 || process.Stage >= len(job.StageDone) {
		return 1
	}
	for !job.StageDone[process.Stage] {
		if !receiveJobEvent(sh) {
			return 128 + int(syscall.SIGTERM)
		}
	}
	if job.Complete() {
		delete(sh.Jobs, job.JobID)
	}
	return process.ExitStatus
}

// NotifyCompletedJobs reports completion once but retains status for a later wait.
func NotifyCompletedJobs(sh *shell.Shell) {
	drainJobEvents(sh)
	for _, id := range sh.JobIDs() {
		job := sh.Jobs[id]
		if job.Complete() && !job.Notified {
			fmt.Fprintf(sh.Stdout, "[%d] Done %s\n", job.JobID, job.CommandText)
			job.Notified = true
		}
	}
}

func signalJobsOnExit(sh *shell.Shell) error {
	drainJobEvents(sh)
	var errs []error
	for _, job := range sh.Jobs {
		if job.Complete() || job.PGID <= 0 {
			continue
		}
		if err := syscall.Kill(-job.PGID, syscall.SIGHUP); err != nil && !errors.Is(err, syscall.ESRCH) {
			errs = append(errs, err)
		}
		if job.State() == shell.ProcessStopped {
			if err := syscall.Kill(-job.PGID, syscall.SIGCONT); err != nil && !errors.Is(err, syscall.ESRCH) {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// ShutdownJobs requests termination and lets existing workers reap children.
// A timeout ends waiting; it does not force programs that ignore SIGHUP to exit.
func ShutdownJobs(sh *shell.Shell, timeout time.Duration) error {
	signalErr := signalJobsOnExit(sh)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		if allJobsComplete(sh) {
			return signalErr
		}
		select {
		case event := <-sh.JobEvents:
			applyJobEvent(sh, event)
		case <-timer.C:
			return errors.Join(signalErr, fmt.Errorf("timed out waiting for jobs to exit"))
		}
	}
}

func allJobsComplete(sh *shell.Shell) bool {
	for _, job := range sh.Jobs {
		if !job.Complete() {
			return false
		}
	}
	return true
}
