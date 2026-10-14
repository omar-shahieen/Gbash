package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/omar-shahieen/gbash/internal/parser"
)

// Child modes need the real shell executable rather than the Go test binary.
func buildTestShell(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "minibash")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "go", "build", "-o", binary, "../..").CombinedOutput(); err != nil {
		t.Fatalf("build minibash: %v\n%s", err, output)
	}
	return binary
}

func TestChainChild(t *testing.T) {
	binary := buildTestShell(t)
	for _, tc := range []struct {
		name, status, pid, line, output string
		wantStatus                      int
	}{
		{"explicit exit", "0", "0", "exit 7", "", 7},
		{"inherited status", "9", "0", "exit $?", "", 9},
		{"inherited background PID", "0", "1234", `echo "$!"`, "1234\n", 0},
		{"no inherited background PID", "0", "0", `echo "pid=[$!]"`, "pid=[]\n", 0},
		{"conditional fallback", "0", "0", "false || exit 9", "", 9},
		{"skip expansion", "0", "0", "true || echo ${", "", 0},
		{"maximum inherited status", "255", "0", "exit", "", 255},
		{"shared process group", "0", "0", `sh -c 'test "$(ps -o pgid= -p $$)" = "$(ps -o pgid= -p $PPID)"'`, "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokens, err := parser.Tokenize(tc.line)
			if err != nil {
				t.Fatal(err)
			}
			chains, err := parser.ParseChains(tokens)
			if err != nil || len(chains) != 1 {
				t.Fatalf("chains = %+v, error = %v", chains, err)
			}
			payload, err := json.Marshal(chains[0].Entries)
			if err != nil {
				t.Fatal(err)
			}
			output, diagnostics, status := runTestChainChild(t, binary, []string{tc.status, tc.pid, string(payload)})
			if status != tc.wantStatus || output != tc.output || diagnostics != "" {
				t.Fatalf("status=%d, stdout=%q, stderr=%q; want status=%d, stdout=%q, empty stderr", status, output, diagnostics, tc.wantStatus, tc.output)
			}
		})
	}
}

func TestChainChildInvalidArguments(t *testing.T) {
	binary := buildTestShell(t)
	for _, tc := range []struct {
		name       string
		args       []string
		diagnostic string
	}{
		{"missing arguments", nil, "expected status, background PID, and entries"},
		{"extra argument", []string{"0", "0", "[]", "extra"}, "expected status, background PID, and entries"},
		{"nonnumeric status", []string{"bad", "0", "[]"}, "invalid status"},
		{"negative status", []string{"-1", "0", "[]"}, "invalid status"},
		{"status above maximum", []string{"256", "0", "[]"}, "invalid status"},
		{"nonnumeric PID", []string{"0", "bad", "[]"}, "invalid background PID"},
		{"negative PID", []string{"0", "-1", "[]"}, "invalid background PID"},
		{"malformed JSON", []string{"0", "0", "["}, "invalid entries"},
		{"wrong JSON type", []string{"0", "0", "{}"}, "invalid entries"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, diagnostics, status := runTestChainChild(t, binary, tc.args)
			if status != 1 || output != "" || !strings.Contains(diagnostics, tc.diagnostic) {
				t.Fatalf("status=%d, stdout=%q, stderr=%q; want status=1, empty stdout, diagnostic %q", status, output, diagnostics, tc.diagnostic)
			}
		})
	}
}

func runTestChainChild(t *testing.T, binary string, args []string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, append([]string{"--chain-child"}, args...)...)
	// Keep child-mode commands in a separate group, as the parent shell does.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Dir = t.TempDir()
	cmd.WaitDelay = time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("chain child timed out: %v", ctx.Err())
	}
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("run chain child: %v", err)
		}
	}
	return stdout.String(), stderr.String(), cmd.ProcessState.ExitCode()
}
