package app

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestTerminalEditing(t *testing.T) {
	binary := buildTestShell(t)
	history := filepath.Join(t.TempDir(), "history")
	terminal := startTestTerminal(t, binary, history)
	banner := terminal.readUntil(t, prompt)
	if !strings.Contains(banner, "gbash") || !strings.Contains(banner, "Omar Mohamed") {
		t.Fatalf("missing interactive banner: %q", banner)
	}
	terminal.command(t, "ech\t completion-marker")
	terminal.readUntil(t, "completion-marker\n")
	terminal.readUntil(t, prompt)
	if err := os.WriteFile(filepath.Join(terminal.cmd.Dir, "complete file.txt"), []byte("file-marker\n"), 0600); err != nil {
		t.Fatal(err)
	}
	terminal.command(t, "cat comp\t")
	terminal.readUntil(t, "file-marker\n")
	terminal.readUntil(t, prompt)
	terminal.command(t, "echo history-marker")
	terminal.readUntil(t, "history-marker\n")
	terminal.readUntil(t, prompt)
	terminal.command(t, "\033[A")
	terminal.readUntil(t, "history-marker\n")
	terminal.readUntil(t, prompt)
	terminal.command(t, "echo draft-marker\033[A\033[B")
	terminal.readUntil(t, "draft-marker\n")
	terminal.readUntil(t, prompt)
	terminal.write(t, "echo canceled\x03")
	terminal.readUntil(t, "\n")
	terminal.readUntil(t, prompt)
	terminal.command(t, "echo status=$?")
	terminal.readUntil(t, "status=130\n")
	terminal.readUntil(t, prompt)
	terminal.write(t, "echo clear-marker\x0c")
	terminal.readUntil(t, prompt)
	terminal.command(t, "")
	terminal.readUntil(t, "clear-marker\n")
	terminal.readUntil(t, prompt)
	terminal.command(t, "history")
	historyOutput := terminal.readUntil(t, prompt)
	if !strings.Contains(historyOutput, "echo history-marker") || strings.Contains(historyOutput, "echo canceled") {
		t.Fatalf("unexpected history: %q", historyOutput)
	}
	terminal.write(t, "\x04")
	waitTerminalExit(t, terminal, 0)
	saved, err := os.ReadFile(history)
	if err != nil || !strings.Contains(string(saved), "echo history-marker\n") || strings.Contains(string(saved), "echo canceled") {
		t.Fatalf("saved history = %q, error = %v", saved, err)
	}

	// A fresh process loads the same file and recalls its last command (history).
	restarted := startTestTerminal(t, binary, history)
	restarted.readUntil(t, prompt)
	restarted.command(t, "\033[A")
	recalled := restarted.readUntil(t, prompt)
	if !strings.Contains(recalled, "echo history-marker") {
		t.Fatalf("history did not survive restart: %q", recalled)
	}
	restarted.write(t, "\x04")
	waitTerminalExit(t, restarted, 0)
}

func TestEditorReleasesForegroundInput(t *testing.T) {
	terminal := startTestTerminal(t, buildTestShell(t))
	terminal.readUntil(t, prompt)
	terminal.command(t, "cat")
	terminal.waitForeground(t, true)
	terminal.write(t, "child-input\n")
	terminal.readUntil(t, "child-input\n") // Terminal echo.
	terminal.readUntil(t, "child-input\n") // Output from cat.
	terminal.write(t, "\x04")
	terminal.readUntil(t, prompt)
	terminal.jobPGID = 0
	terminal.write(t, "\x04")
	waitTerminalExit(t, terminal, 0)
}

func TestEditorShutdownRestoresTerminal(t *testing.T) {
	terminal := startTestTerminal(t, buildTestShell(t))
	terminal.readUntil(t, prompt)
	if err := terminal.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	waitTerminalExit(t, terminal, 143)
	var modes syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, terminal.master.Fd(), syscall.TCGETS, uintptr(unsafe.Pointer(&modes)))
	if errno != 0 {
		t.Fatalf("read restored terminal mode: %v", errno)
	}
	mask := uint32(syscall.ECHO | syscall.ICANON | syscall.ISIG)
	if modes.Lflag&mask != mask {
		t.Fatalf("terminal still in raw mode after shutdown: flags=%x", modes.Lflag)
	}
}

func waitTerminalExit(t *testing.T, terminal *testTerminal, status int) {
	t.Helper()
	select {
	case <-terminal.exited:
		if terminal.cmd.ProcessState.ExitCode() != status {
			t.Fatalf("exit status = %d, want %d", terminal.cmd.ProcessState.ExitCode(), status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("terminal shell did not exit")
	}
}
