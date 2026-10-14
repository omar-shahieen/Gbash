package executor

import (
	"errors"
	"os"
	"syscall"
	"unsafe"

	"github.com/omar-shahieen/gbash/internal/shell"
)

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
