package app

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/omar-shahieen/gbash/internal/shell"
)

func TestReadLineCanStopWhileInputIsBlocked(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	shutdown := make(chan struct{})
	close(shutdown)
	result := readLine(bufio.NewReader(reader), shutdown)
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("error = %v, want cancellation", result.err)
	}
}

func TestConditionalExecution(t *testing.T) {
	const missing = "minibash_unknown_command_875"
	cases := []struct {
		line, output string
		status       int
		exiting      bool
	}{
		{line: "echo yes && echo next", output: "yes\nnext\n"},
		{line: "type " + missing + " && echo skipped", status: 1},
		{line: "type " + missing + " || echo $?", output: "1\n"},
		{line: "echo yes || echo skipped && echo final", output: "yes\nfinal\n"},
		{line: "export MINIBASH_CONDITIONAL_CHECK=omar && echo \"$MINIBASH_CONDITIONAL_CHECK\"", output: "omar\n"},
		{line: "echo yes || echo ${", output: "yes\n"},
		{line: "echo before &&", status: 2},
		{line: "echo before || cat >", status: 2},
		{line: "exit 7 && echo unreachable", status: 7, exiting: true},
		{line: "echo one; echo two", output: "one\ntwo\n"},
		{line: "type " + missing + "; echo next", output: "next\n"},
		{line: "echo one;", output: "one\n"},
		{line: "echo before;; echo after", status: 2},
		{line: "exit 7; echo unreachable", status: 7, exiting: true},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			t.Setenv("MINIBASH_CONDITIONAL_CHECK", "initial")
			sh := shell.New()
			var output, diagnostics bytes.Buffer
			sh.Stdout, sh.Stderr = &output, &diagnostics
			var running atomic.Bool
			executeLine(sh, tc.line, &running)
			if output.String() != tc.output || sh.LastStatus != tc.status || sh.Exiting != tc.exiting {
				t.Fatalf("output=%q, status=%d, exiting=%v; want %q, %d, %v",
					output.String(), sh.LastStatus, sh.Exiting, tc.output, tc.status, tc.exiting)
			}
			if running.Load() {
				t.Fatal("execution flag remained set")
			}
		})
	}
}

func TestRunExecutesFinalLineWithoutNewline(t *testing.T) {
	sh := shell.New()
	var output, diagnostics bytes.Buffer
	sh.Stdin = strings.NewReader("echo final")
	sh.Stdout, sh.Stderr = &output, &diagnostics
	if status := run(sh); status != 0 {
		t.Fatalf("status = %d", status)
	}
	if !strings.Contains(output.String(), "final\n") {
		t.Fatalf("output = %q", output.String())
	}
	if diagnostics.Len() != 0 {
		t.Fatalf("diagnostics = %q", diagnostics.String())
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("input failed") }

func TestRunReturnsOnReadError(t *testing.T) {
	sh := shell.New()
	var output, diagnostics bytes.Buffer
	sh.Stdin = failingReader{}
	sh.Stdout, sh.Stderr = &output, &diagnostics
	if status := run(sh); status != 1 {
		t.Fatalf("status = %d", status)
	}
	if !strings.Contains(diagnostics.String(), "input failed") {
		t.Fatalf("diagnostics = %q", diagnostics.String())
	}
}
