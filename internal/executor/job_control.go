package executor

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"syscall"

	"github.com/omar-shahieen/Gbash/internal/shell"
)

// Job-control commands run in the main shell, with the same redirection setup.
func executeJobControl(sh *shell.Shell, cmd shell.Command) int {
	streams, err := prepareRedirections(cmd, shellStreams(sh))
	if err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: %s: %v\n", cmd.Name, err)
		return 1
	}
	defer streams.close()
	if cmd.Name == "wait" {
		return executeWait(sh, cmd.Args, streams.stderr)
	}
	job, err := selectJob(sh, cmd.Args)
	if err != nil {
		fmt.Fprintf(streams.stderr, "minishell: %s: %v\n", cmd.Name, err)
		return 1
	}
	if cmd.Name == "fg" {
		return foregroundJob(sh, job, streams.stdout, streams.stderr)
	}
	return backgroundJob(job, streams.stdout, streams.stderr)
}

func executeWait(sh *shell.Shell, args []string, errOut io.Writer) int {
	if len(args) > 1 {
		fmt.Fprintln(errOut, "minishell: wait: too many arguments")
		return 2
	}
	if len(args) == 0 {
		for _, id := range sh.JobIDs() {
			if job, exists := sh.Jobs[id]; exists {
				waitJob(sh, job)
			}
		}
		return 0
	}
	target := args[0]
	if strings.HasPrefix(target, "%") {
		job, err := lookupJob(sh, target)
		if err != nil {
			fmt.Fprintf(errOut, "minishell: wait: %v\n", err)
			return 1
		}
		return waitJob(sh, job)
	}
	pid, err := strconv.Atoi(target)
	if err != nil || pid <= 0 {
		fmt.Fprintf(errOut, "minishell: wait: invalid pid: %s\n", target)
		return 1
	}
	job, process := findProcess(sh, pid)
	if process == nil {
		fmt.Fprintf(errOut, "minishell: wait: no such process: %d\n", pid)
		return 1
	}
	return waitProcess(sh, job, process)
}

func foregroundJob(sh *shell.Shell, job *shell.Job, out, errOut io.Writer) int {
	if !sh.Interactive || sh.Terminal == nil {
		fmt.Fprintln(errOut, "minishell: fg: requires a terminal")
		return 1
	}
	if job == nil || job.Complete() || job.PGID <= 0 {
		fmt.Fprintln(errOut, "minishell: fg: no active job")
		return 1
	}
	if err := setTerminalForegroundPGID(sh.Terminal.Fd(), job.PGID); err != nil {
		fmt.Fprintf(errOut, "minishell: fg: %v\n", err)
		return 1
	}
	if err := resumeJob(job); err != nil {
		fmt.Fprintf(errOut, "minishell: fg: resume: %v\n", err)
		restoreTerminal(sh, errOut)
		return 1
	}
	job.Background = false
	return runForegroundJob(sh, job, out, errOut)
}

// Both a newly launched foreground job and fg use this same wait/restore lifecycle.
func runForegroundJob(sh *shell.Shell, job *shell.Job, out, errOut io.Writer) (result int) {
	sh.ForegroundJob = job
	defer func() {
		restored := restoreTerminal(sh, errOut)
		if !restored {
			result = 1
		}
		sh.ForegroundJob = nil
		if job.Complete() {
			delete(sh.Jobs, job.JobID)
		} else if sh.Interactive && restored && job.State() == shell.ProcessStopped {
			fmt.Fprintf(out, "[%d] Stopped\n", job.JobID)
		}
	}()
	return waitForegroundJob(sh, job)
}

func restoreTerminal(sh *shell.Shell, errOut io.Writer) bool {
	if !sh.Interactive {
		return true
	}
	if err := setTerminalForegroundPGID(sh.Terminal.Fd(), sh.ShellPGID); err != nil {
		fmt.Fprintf(errOut, "minishell: restore terminal: %v\n", err)
		return false
	}
	return true
}

func backgroundJob(job *shell.Job, out, errOut io.Writer) int {
	if job == nil || job.Complete() || job.PGID <= 0 {
		fmt.Fprintln(errOut, "minishell: bg: no active job")
		return 1
	}
	if err := resumeJob(job); err != nil {
		fmt.Fprintf(errOut, "minishell: bg: resume: %v\n", err)
		return 1
	}
	job.Background = true
	fmt.Fprintf(out, "[%d] Running\n", job.JobID)
	return 0
}

func resumeJob(job *shell.Job) error {
	if err := syscall.Kill(-job.PGID, syscall.SIGCONT); err != nil {
		return err
	}
	job.MarkRunning()
	return nil
}

func parseJobID(target string) (int, error) {
	digits, hasPrefix := strings.CutPrefix(target, "%")
	id, err := strconv.Atoi(digits)
	if !hasPrefix || err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid job ID: %s", target)
	}
	return id, nil
}

func lookupJob(sh *shell.Shell, target string) (*shell.Job, error) {
	id, err := parseJobID(target)
	if err != nil {
		return nil, err
	}
	job, exists := sh.Jobs[id]
	if !exists {
		return nil, fmt.Errorf("no such job: %s", target)
	}
	return job, nil
}

func selectJob(sh *shell.Shell, args []string) (*shell.Job, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("too many arguments")
	}
	if len(args) == 1 {
		return lookupJob(sh, args[0])
	}
	var latest *shell.Job
	for _, job := range sh.Jobs {
		if job != nil && !job.Complete() && job.PGID > 0 &&
			(latest == nil || job.JobID > latest.JobID) {
			latest = job
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("no jobs")
	}
	return latest, nil
}

func findProcess(sh *shell.Shell, pid int) (*shell.Job, *shell.Process) {
	for _, job := range sh.Jobs {
		for _, process := range job.Processes {
			if process.PID == pid {
				return job, process
			}
		}
	}
	return nil, nil
}
