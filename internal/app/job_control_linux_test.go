package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestTerminalJobControl(t *testing.T) {
	binary := buildTestShell(t)
	for _, tc := range []struct {
		name, command string
		background    bool
	}{
		{"command", "sleep 30", false},
		{"pipeline", "sleep 30 | cat", false},
		{"conditional chain", "sleep 30 && echo unreachable &", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			terminal := startTestTerminal(t, binary)
			terminal.readUntil(t, prompt)
			terminal.write(t, "\x03") // Ctrl+C at the prompt must preserve the shell.
			terminal.readUntil(t, "\n")
			terminal.readUntil(t, prompt)
			terminal.command(t, tc.command)
			if tc.background {
				terminal.readUntil(t, prompt)
				terminal.command(t, "fg")
			}
			terminal.waitForeground(t, true)
			terminal.write(t, "\x1a") // Ctrl+Z must stop the whole foreground group.
			stopped := terminal.readUntil(t, "[1] Stopped\n")
			terminal.readUntil(t, prompt)
			if !strings.Contains(stopped, "[1] Stopped") {
				t.Fatalf("missing stopped notification: %q", stopped)
			}
			terminal.waitForeground(t, false)
			terminal.command(t, "jobs")
			jobs := terminal.readUntil(t, prompt)
			match := regexp.MustCompile(`\[1\] stopped pgid=(\d+)`).FindStringSubmatch(jobs)
			if len(match) != 2 {
				t.Fatalf("missing stopped job: %q", jobs)
			}
			terminal.jobPGID, _ = strconv.Atoi(match[1])
			terminal.command(t, "bg")
			resumed := terminal.readUntil(t, prompt)
			if !strings.Contains(resumed, "[1] Running") {
				t.Fatalf("missing bg notification: %q", resumed)
			}
			terminal.waitForeground(t, false)
			terminal.command(t, "fg")
			terminal.waitForeground(t, true)
			terminal.write(t, "\x03") // Ctrl+C must terminate the job, not the shell.
			finished := terminal.readUntil(t, prompt)
			if strings.Contains(finished, "\nunreachable\n") {
				t.Fatalf("terminated chain continued: %q", finished)
			}
			terminal.waitForeground(t, false)
			terminal.command(t, "echo status=$?; jobs")
			output := terminal.readUntil(t, prompt)
			if !strings.Contains("\n"+output, "\nstatus=130\n") || strings.Contains(output, "pgid=") {
				t.Fatalf("unexpected interrupt status or retained job: %q", output)
			}
			terminal.jobPGID = 0
			terminal.write(t, "exit 0\n")
			select {
			case err := <-terminal.exited:
				if err != nil {
					t.Fatalf("shell exit: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("shell did not exit")
			}
		})
	}
}

type testTerminal struct {
	master  *os.File
	cmd     *exec.Cmd
	output  chan string
	exited  chan error
	pending string
	jobPGID int
}

func startTestTerminal(t *testing.T, binary string, historyFile ...string) *testTerminal {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("open pseudo-terminal: %v", err)
	}
	t.Cleanup(func() { master.Close() })
	var unlock int32
	if err := terminalIOCTL(master, syscall.TIOCSPTLCK, &unlock); err != nil {
		t.Fatal(err)
	}
	var number int32
	if err := terminalIOCTL(master, syscall.TIOCGPTN, &number); err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer slave.Close()
	cmd := exec.Command(binary)
	cmd.Dir = t.TempDir()
	path := filepath.Join(cmd.Dir, "history")
	if len(historyFile) > 0 {
		path = historyFile[0]
	}
	cmd.Env = append(os.Environ(), "HISTFILE="+path, "TERM=xterm-256color", "NO_COLOR=1")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start terminal shell: %v", err)
	}
	terminal := &testTerminal{master: master, cmd: cmd, output: make(chan string, 64), exited: make(chan error, 1)}
	go func() {
		terminal.exited <- cmd.Wait()
		close(terminal.exited)
	}()
	go func() {
		defer close(terminal.output)
		buffer := make([]byte, 4096)
		for {
			n, err := master.Read(buffer)
			if n > 0 {
				terminal.output <- strings.ReplaceAll(string(buffer[:n]), "\r", "")
			}
			if err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		// Clean up long-running jobs even if an assertion fails before jobs is read.
		var foreground int32
		if terminalIOCTL(master, syscall.TIOCGPGRP, &foreground) == nil && foreground > 0 && int(foreground) != cmd.Process.Pid {
			syscall.Kill(-int(foreground), syscall.SIGKILL)
		}
		if terminal.jobPGID > 0 {
			syscall.Kill(-terminal.jobPGID, syscall.SIGKILL)
		}
		cmd.Process.Kill()
		select {
		case <-terminal.exited:
		case <-time.After(3 * time.Second):
			t.Error("terminal shell was not reaped during cleanup")
		}
	})
	return terminal
}

func (terminal *testTerminal) write(t *testing.T, input string) {
	t.Helper()
	if _, err := terminal.master.WriteString(input); err != nil {
		t.Fatalf("terminal input: %v", err)
	}
}

// Consume the echoed command before looking for results. An editor redraw can
// contain the prompt string while the user is still typing.
func (terminal *testTerminal) command(t *testing.T, input string) {
	t.Helper()
	terminal.write(t, input+"\n")
	terminal.readUntil(t, "\n")
}

func (terminal *testTerminal) readUntil(t *testing.T, marker string) string {
	t.Helper()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		if index := strings.Index(terminal.pending, marker); index >= 0 {
			end := index + len(marker)
			output := terminal.pending[:end]
			terminal.pending = terminal.pending[end:]
			return output
		}
		select {
		case chunk, ok := <-terminal.output:
			if !ok {
				t.Fatalf("terminal closed while waiting for %q: %q", marker, terminal.pending)
			}
			terminal.pending += chunk
		case <-timer.C:
			t.Fatalf("timed out waiting for %q: %q", marker, terminal.pending)
		}
	}
}

func (terminal *testTerminal) waitForeground(t *testing.T, job bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var group int32
		if err := terminalIOCTL(terminal.master, syscall.TIOCGPGRP, &group); err != nil {
			t.Fatal(err)
		}
		if group > 0 && (int(group) != terminal.cmd.Process.Pid) == job {
			if job {
				if terminal.jobPGID > 0 && terminal.jobPGID != int(group) {
					t.Fatalf("fg selected group %d, want original group %d", group, terminal.jobPGID)
				}
				terminal.jobPGID = int(group)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("terminal foreground ownership did not change (job=%v)", job)
}

func terminalIOCTL(file *os.File, request uintptr, value *int32) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), request, uintptr(unsafe.Pointer(value)))
	if errno != 0 {
		return errno
	}
	return nil
}
