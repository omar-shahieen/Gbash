package executor

import (
	"fmt"
	"io"
	"os"

	"github.com/omar-shahieen/Gbash/internal/shell"
)

func prepareRedirections(
	cmd shell.Command,
	in io.Reader,
	out, errOut io.Writer,
) (
	io.Reader,
	io.Writer,
	io.Writer,
	func(),
	error,
) {
	var files []*os.File

	cleanup := func() {
		for _, f := range files {
			_ = f.Close()
		}
	}

	for _, r := range cmd.Redirections {
		f, err := openRedirection(r)
		if err != nil {
			cleanup()
			return nil, nil, nil, nil,
				fmt.Errorf("%s: %w", r.Filename, err)
		}
		files = append(files, f)

		switch r.FD {
		case 0:
			in = f
		case 1:
			out = f
		case 2:
			errOut = f
		}
	}

	return in, out, errOut, cleanup, nil

}

func openRedirection(r shell.Redirection) (*os.File, error) {
	switch r.Mode {
	case shell.ModeRead:
		return os.Open(r.Filename)
	case shell.ModeAppend:
		return os.OpenFile(r.Filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	default: // ModeTruncate
		return os.OpenFile(r.Filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	}
}
