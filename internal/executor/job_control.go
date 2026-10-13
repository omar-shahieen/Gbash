package executor

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/omar-shahieen/Gbash/internal/shell"
)

func executeFg(sh *shell.Shell, cmd shell.Command) int {
	_, out, outErr, cleanup, err := prepareRedirections(cmd, sh.Stdin, sh.Stdout, sh.Stderr)
	if err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: fg: %v\n", err)
		return 1
	}
	defer cleanup()

	job, err := selectJob(sh, cmd.Args)

	if err != nil {
		fmt.Fprintf(outErr, "minishell: fg: %v\n", err)
		return 1
	}

	return foregroundJob(sh, job, out, outErr)
}

func executeBg(sh *shell.Shell, cmd shell.Command) int {
	_, out, outErr, cleanup, err := prepareRedirections(cmd, sh.Stdin, sh.Stdout, sh.Stderr)
	if err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: bg: %v\n", err)
		return 1
	}
	defer cleanup()

	job, err := selectJob(sh, cmd.Args)

	if err != nil {
		fmt.Fprintf(outErr, "minishell: bg: %v\n", err)
		return 1
	}

	return backgroundJob(job, out, outErr)
}

func executeWait(sh *shell.Shell, cmd shell.Command) int {

	_, _, errOut, cleanup, err := prepareRedirections(cmd, sh.Stdin, sh.Stdout, sh.Stderr)
	if err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: wait: %v\n", err)
		return 1
	}
	defer cleanup()

	if len(cmd.Args) > 1 {
		fmt.Fprintln(errOut, "minishell: wait: too many arguments")
		return 2
	}
	if len(cmd.Args) == 0 {
		jobIDs := slices.Collect(maps.Keys(sh.Jobs))
		slices.Sort(jobIDs)

		for _, jobId := range jobIDs {
			job, exists := sh.Jobs[jobId]
			if !exists {
				continue
			}

			waitJob(sh, job)
		}

		return 0

	}

	if !strings.HasPrefix(cmd.Args[0], "%") {
		// pid argument
		pid, err := strconv.Atoi(cmd.Args[0])
		if err != nil || pid <= 0 {
			fmt.Fprintf(errOut, "minishell: wait: invalid pid: %s\n", cmd.Args[0])
			return 1
		}

		job, process := findProcess(sh, pid)
		if job == nil || process == nil {
			fmt.Fprintf(errOut, "minishell: wait: no such process: %d\n", pid)
			return 1
		}

		return waitProcess(sh, job, process)
	}

	var job *shell.Job
	jobID, err := parseJobID(cmd.Args[0])
	if err != nil {
		fmt.Fprintf(errOut, "minishell: wait: %v\n", err)
		return 1
	}

	var exists bool
	job, exists = sh.Jobs[jobID]
	if !exists {
		fmt.Fprintf(errOut, "minishell: wait: no such job: %s\n", cmd.Args[0])
		return 1
	}

	return waitJob(sh, job)
}

func foregroundJob(
	sh *shell.Shell,
	job *shell.Job,
	out, errOut io.Writer,
) (result int) {
	// validate the terminal
	if !sh.Interactive || sh.Terminal == nil {
		fmt.Fprintln(errOut, "minishell: fg: requires a terminal")
		return 1
	}
	// if the job is nil, complete, or has no process group ID, we cannot bring it to the foreground
	if job == nil || jobComplete(job) || job.PGID <= 0 {
		fmt.Fprintln(errOut, "minishell: fg: no active job")
		return 1
	}

	// transfer the terminal to the job's process group
	if err := setTerminalForegroundPGID(
		sh.Terminal.Fd(), job.PGID,
	); err != nil {
		fmt.Fprintf(errOut, "minishell: fg: %v\n", err)
		return 1
	}

	waited := false

	// Always reclaim the terminal, including when SIGCONT fails.
	defer func() {
		// restore the terminal to the shell's process group
		err := setTerminalForegroundPGID(
			sh.Terminal.Fd(), sh.ShellPGID,
		)
		if err != nil {
			fmt.Fprintf(errOut,
				"minishell: restore terminal: %v\n", err)
			result = 1
		}

		if !waited {
			return
		}

		sh.ForegroundJob = nil
		if jobComplete(job) {
			delete(sh.Jobs, job.JobID)
		} else if err == nil &&
			job.State() == shell.ProcessStopped {
			fmt.Fprintf(out, "[%d] Stopped\n", job.JobID)
		}
	}()

	// resume the job
	if err := syscall.Kill(-job.PGID, syscall.SIGCONT); err != nil {
		fmt.Fprintf(errOut, "minishell: fg: resume: %v\n", err)
		return 1
	}

	for _, record := range job.Processes {
		if record.State == shell.ProcessStopped {
			record.State = shell.ProcessRunning
		}
	}

	// update the job's background status and set it as the current job
	job.Background = false
	sh.ForegroundJob = job
	waited = true

	// wait for the job to complete or stop
	return waitForegroundJob(sh, job)
}

func backgroundJob(
	job *shell.Job,
	out, errOut io.Writer,
) int {

	// Only active jobs with a valid process group can resume in the background.
	if job == nil || jobComplete(job) || job.PGID <= 0 {
		fmt.Fprintln(errOut, "minishell: bg: no active job")
		return 1
	}

	// resume the job
	if err := syscall.Kill(-job.PGID, syscall.SIGCONT); err != nil {
		fmt.Fprintf(errOut, "minishell: bg: resume: %v\n", err)
		return 1
	}

	for _, record := range job.Processes {
		if record.State == shell.ProcessStopped {
			record.State = shell.ProcessRunning
		}
	}

	// Keep terminal ownership with the shell and mark the job as background.
	job.Background = true

	fmt.Fprintf(out, "[%d] Running\n", job.JobID)

	return 0
}

func waitForegroundJob(sh *shell.Shell, job *shell.Job) int {
	if len(job.StageStatuses) == 0 {
		return 0
	}
	for !jobComplete(job) {
		select {
		case <-sh.Shutdown:
			return 128 + int(syscall.SIGTERM)
		case event := <-sh.JobEvents:
			applyJobEvent(sh, event)
		}
		if sh.Interactive && job.State() == shell.ProcessStopped {
			return 128 + int(syscall.SIGTSTP)
		}
	}
	return job.StageStatuses[len(job.StageStatuses)-1]
}

func waitJob(sh *shell.Shell, job *shell.Job) int {
	if job == nil {
		return 1
	}
	if len(job.StageStatuses) == 0 {
		delete(sh.Jobs, job.JobID)
		return 0
	}
	for !jobComplete(job) {
		select {
		case <-sh.Shutdown:
			return 128 + int(syscall.SIGTERM)
		case event := <-sh.JobEvents:
			applyJobEvent(sh, event)
		}

	}

	status := job.StageStatuses[len(job.StageStatuses)-1]
	delete(sh.Jobs, job.JobID)

	return status
}

func waitProcess(
	sh *shell.Shell,
	job *shell.Job,
	record *shell.Process,
) int {
	if job == nil || record == nil {
		return 1
	}
	if record.Stage < 0 || record.Stage >= len(job.StageDone) {
		return 1
	}
	for !job.StageDone[record.Stage] {
		select {
		case <-sh.Shutdown:
			return 128 + int(syscall.SIGTERM)
		case event := <-sh.JobEvents:
			applyJobEvent(sh, event)
		}

	}
	if jobComplete(job) {
		delete(sh.Jobs, job.JobID)
	}
	return record.ExitStatus
}

func parseJobID(s string) (int, error) {
	original := s
	s, exist := strings.CutPrefix(s, "%")
	if !exist {
		return 0, fmt.Errorf("invalid job ID: %s", original)
	}

	jobID, err := strconv.Atoi(s)
	if err != nil || jobID <= 0 {
		return 0, fmt.Errorf("invalid job ID: %s", original)
	}

	return jobID, nil
}

func selectJob(
	sh *shell.Shell,
	args []string,
) (*shell.Job, error) {

	if len(args) > 1 {
		return nil, fmt.Errorf("too many arguments")
	}

	jobs := slices.Collect(maps.Keys(sh.Jobs))
	slices.Sort(jobs)

	var job *shell.Job
	if len(args) == 0 {
		for i := len(jobs) - 1; i >= 0; i-- {
			candidate := sh.Jobs[jobs[i]]
			if candidate != nil && !jobComplete(candidate) && candidate.PGID > 0 {
				job = candidate
				break
			}
		}
		if job == nil {
			return nil, fmt.Errorf("no jobs")
		}
	} else {

		jobID, err := parseJobID(args[0])
		if err != nil {

			return nil, err
		}

		var exists bool
		job, exists = sh.Jobs[jobID]
		if !exists {

			return nil, fmt.Errorf("no such job: %s", args[0])
		}
	}

	return job, nil
}

func findProcess(
	sh *shell.Shell,
	pid int,
) (*shell.Job, *shell.Process) {
	for _, job := range sh.Jobs {
		for _, process := range job.Processes {
			if process.PID == pid {
				return job, process
			}
		}
	}
	return nil, nil
}

func applyJobEvent(sh *shell.Shell, event shell.JobEvent) {
	job, exists := sh.Jobs[event.JobID]
	if !exists {
		return
	}
	if event.Stage < 0 || event.Stage >= len(job.StageDone) || event.Stage >= len(job.StageStatuses) {
		return
	}
	for _, record := range job.Processes {
		if record.Stage != event.Stage {
			continue
		}
		record.State = event.State
		if event.State == shell.ProcessDone {
			record.ExitStatus = event.ExitStatus
		}
		break
	}
	if event.State == shell.ProcessDone {
		job.StageDone[event.Stage] = true
		job.StageStatuses[event.Stage] = event.ExitStatus
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

func jobComplete(job *shell.Job) bool {
	for _, done := range job.StageDone {
		if !done {
			return false
		}
	}
	return true
}

func NotifyCompletedJobs(sh *shell.Shell) {
	drainJobEvents(sh)

	jobIDs := slices.Collect(maps.Keys(sh.Jobs))
	slices.Sort(jobIDs)

	for _, jobID := range jobIDs {

		job, exists := sh.Jobs[jobID]

		if !exists {
			continue
		}

		if jobComplete(job) && !job.Notified {
			fmt.Fprintf(sh.Stdout, "[%d] Done %s\n",
				job.JobID, job.CommandText)
			job.Notified = true
		}

	}

}

func signalJobsOnExit(sh *shell.Shell) error {

	drainJobEvents(sh)

	jobs := slices.Collect(maps.Values(sh.Jobs))

	var errs []error

	for _, job := range jobs {
		if jobComplete(job) || job.PGID <= 0 {
			continue
		}

		err := syscall.Kill(-job.PGID, syscall.SIGHUP)
		if err != nil && !errors.Is(err, syscall.ESRCH) {
			errs = append(errs, err)
		}
		if job.State() == shell.ProcessStopped {
			err := syscall.Kill(-job.PGID, syscall.SIGCONT)
			if err != nil && !errors.Is(err, syscall.ESRCH) {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

func ShutdownJobs(sh *shell.Shell, timeout time.Duration) error {

	signalErr := signalJobsOnExit(sh)

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		allDone := true
		for _, job := range sh.Jobs {
			if !jobComplete(job) {
				allDone = false
				break
			}
		}
		if allDone {
			return signalErr
		}

		select {
		case event := <-sh.JobEvents:
			applyJobEvent(sh, event)

		case <-timer.C:
			return errors.Join(signalErr,
				fmt.Errorf("timed out waiting for jobs to exit"))
		}
	}

}
