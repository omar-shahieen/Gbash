package executor

import (
	"fmt"
	"io"
	"os"

	"github.com/omar-shahieen/Gbash/internal/shell"
)

// commandStreams names a command's streams and owns only its redirection files.
// Pipeline pipe ends are owned separately by their stages.
type commandStreams struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	opened []*os.File
}

func shellStreams(sh *shell.Shell) commandStreams {
	return commandStreams{stdin: sh.Stdin, stdout: sh.Stdout, stderr: sh.Stderr}
}

func (streams commandStreams) close() {
	for _, file := range streams.opened {
		_ = file.Close()
	}
}

func prepareRedirections(cmd shell.Command, streams commandStreams) (commandStreams, error) {
	for _, redirection := range cmd.Redirections {
		file, err := openRedirection(redirection)
		if err != nil {
			streams.close()
			return commandStreams{}, fmt.Errorf("%s: %w", redirection.Filename, err)
		}
		streams.opened = append(streams.opened, file)
		switch redirection.FD {
		case 0:
			streams.stdin = file
		case 1:
			streams.stdout = file
		case 2:
			streams.stderr = file
		}
	}
	return streams, nil
}

// childFiles converts the streams to the descriptors required by os.StartProcess.
func (streams commandStreams) childFiles() ([]*os.File, error) {
	inputs := []struct {
		name   string
		stream any
	}{
		{"stdin", streams.stdin},
		{"stdout", streams.stdout},
		{"stderr", streams.stderr},
	}
	files := make([]*os.File, len(inputs))
	for i, input := range inputs {
		file, ok := input.stream.(*os.File)
		if !ok || file == nil {
			return nil, fmt.Errorf("%s must be an *os.File", input.name)
		}
		files[i] = file
	}
	return files, nil
}

func openRedirection(redirection shell.Redirection) (*os.File, error) {
	switch redirection.Mode {
	case shell.ModeRead:
		return os.Open(redirection.Filename)
	case shell.ModeAppend:
		return os.OpenFile(redirection.Filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	default:
		return os.OpenFile(redirection.Filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	}
}
