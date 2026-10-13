package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/omar-shahieen/Gbash/internal/shell"
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
