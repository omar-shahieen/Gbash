package executor

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"unsafe"

	"github.com/omar-shahieen/Gbash/internal/shell"
)

// ProcessGroup launches a pipeline sequentially from the main shell.
// The first child creates the group; later children join it.
type ProcessGroup struct {
	id int

	foreground bool
	ttyFD      int
}

func (g *ProcessGroup) launch(
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

func InitTerminal(sh *shell.Shell) error {
	terminal, ok := sh.Stdin.(*os.File)
	if !ok {
		return nil
	}
	_, err := terminalForegroundPGID(terminal.Fd())
	if errors.Is(err, syscall.ENOTTY) {
		// Not a terminal, so nothing to do.
		return nil
	}
	if err != nil {
		return err
	}

	sh.Interactive = true
	sh.Terminal = terminal
	sh.ShellPGID = syscall.Getpgrp()
	return nil
}

func terminalForegroundPGID(fd uintptr) (int, error) {
	var pgid int32

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TIOCGPGRP),
		uintptr(unsafe.Pointer(&pgid)),
	)
	if errno != 0 {
		return 0, errno
	}

	return int(pgid), nil
}

func setTerminalForegroundPGID(fd uintptr, pgid int) error {
	group := int32(pgid)

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TIOCSPGRP),
		uintptr(unsafe.Pointer(&group)),
	)
	if errno != 0 {
		return errno
	}

	return nil
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
