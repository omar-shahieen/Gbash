package executor

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/omar-shahieen/gbash/internal/shell"
)

// processGroup launches a pipeline sequentially from the main shell.
// A zero group ID creates a group for the first child; otherwise children join it.
type processGroup struct {
	id int

	foreground bool
	ttyFD      int
}

func (g *processGroup) launch(
	path string,
	args []string,
	files []*os.File,
) (*os.Process, error) {
	attr := &os.ProcAttr{
		Env:   os.Environ(),
		Files: files,
		Sys: &syscall.SysProcAttr{
			Setpgid:    true,
			Pgid:       g.id,
			Foreground: g.foreground && g.id == 0,
			Ctty:       g.ttyFD,
		},
	}
	proc, err := os.StartProcess(path, args, attr)

	if err != nil {
		return nil, err
	}
	if g.id == 0 {
		g.id = proc.Pid
	}
	return proc, nil
}

func watchExternalCommand(
	sh *shell.Shell,
	proc *os.Process,
	jobID, stage int,
	errOut io.Writer,
) int {
	defer proc.Release()

	local := &shell.Process{
		PID:   proc.Pid,
		State: shell.ProcessRunning,
	}
	for {
		status, err := waitProcessEvent(proc.Pid)
		if err != nil {
			fmt.Fprintf(errOut, "minishell: wait: %v\n", err)
			return 1
		}
		applyWaitStatus(local, status)
		if status.Stopped() || status.Continued() {
			sh.JobEvents <- shell.JobEvent{
				JobID: jobID,
				Stage: stage,
				PID:   proc.Pid,
				State: local.State,
			}
		}
		if status.Signaled() && status.Signal() == syscall.SIGINT {
			fmt.Fprintln(errOut)
		}
		if status.Exited() || status.Signaled() {
			return local.ExitStatus
		}
	}
}

func waitProcessEvent(pid int) (syscall.WaitStatus, error) {
	for {
		var status syscall.WaitStatus

		_, err := syscall.Wait4(
			pid,
			&status,
			syscall.WUNTRACED|syscall.WCONTINUED,
			nil,
		)

		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return 0, err
		}

		return status, nil
	}
}

func applyWaitStatus(
	record *shell.Process,
	status syscall.WaitStatus,
) {
	switch {
	case status.Stopped():
		record.State = shell.ProcessStopped
	case status.Continued():
		record.State = shell.ProcessRunning
	case status.Exited():
		record.State = shell.ProcessDone
		record.ExitStatus = status.ExitStatus()
	case status.Signaled():
		record.State = shell.ProcessDone
		record.ExitStatus = 128 + int(status.Signal())
	}

}
