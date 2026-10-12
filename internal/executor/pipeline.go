package executor

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/omar-shahieen/Gbash/internal/builtins"
	"github.com/omar-shahieen/Gbash/internal/shell"
)

func startExternalCommand(
	cmd shell.Command,
	in io.Reader,
	out, errOut io.Writer,
	pg *ProcessGroup,
) (*os.Process, error) {
	path, err := exec.LookPath(cmd.Name)
	if err != nil {
		return nil, err
	}
	inFile, ok := in.(*os.File)
	if !ok {
		return nil, fmt.Errorf("stdin must be an *os.File")
	}
	outFile, ok := out.(*os.File)
	if !ok {
		return nil, fmt.Errorf("stdOut must be an *os.File")
	}
	errFile, ok := errOut.(*os.File)
	if !ok {
		return nil, fmt.Errorf("stdErrOut must be an *os.File")
	}

	args := append([]string{cmd.Name}, cmd.Args...)

	return pg.launch(
		path,
		args,
		[]*os.File{inFile, outFile, errFile},
	)
}

func startCommand(
	sh *shell.Shell,
	cmd shell.Command,
	in io.Reader,
	out, errOut io.Writer,
	pg *ProcessGroup,
	record *shell.Process,
	jobID int,
	stage int,
) func() int {

	in, out, errOut, cleanup, err := prepareRedirections(
		cmd, in, out, errOut,
	)
	if err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: %v\n", err)
		if record != nil {
			record.ExitStatus = 1
			record.State = shell.ProcessDone
		}
		return func() int {
			return 1
		}
	}
	if cmd.Name == "" {
		defer cleanup()
		return func() int { return 0 }
	}

	if fn, ok := builtins.Lookup(cmd.Name); ok {
		return func() int {
			defer cleanup()
			return fn(sh, cmd.Args, out, errOut)
		}
	}
	proc, err := startExternalCommand(cmd, in, out, errOut, pg)

	if err != nil {
		defer cleanup()

		var execErr *exec.Error
		var status int
		if errors.As(err, &execErr) &&
			errors.Is(execErr.Err, exec.ErrNotFound) {
			fmt.Fprintf(errOut,
				"minishell: %s: command not found\n", cmd.Name)
			status = 127
			record.ExitStatus = status
			record.State = shell.ProcessDone
			return func() int { return status }

		}

		fmt.Fprintf(errOut, "minishell: %s: %v\n", cmd.Name, err)
		status = 126
		record.ExitStatus = status
		record.State = shell.ProcessDone
		return func() int { return status }
	}
	record.PID = proc.Pid
	record.State = shell.ProcessRunning
	return func() int {

		defer cleanup()

		return watchExternalCommand(
			sh, proc, jobID, stage, errOut,
		)

	}
}

func startPipeline(sh *shell.Shell, pipeline shell.Pipeline) func() int {
	cmds := pipeline.Commands
	n := len(cmds)
	if n == 0 {
		return func() int { return 0 }
	}
	job := &shell.Job{
		Pipeline:      pipeline,
		StageDone:     make([]bool, n),
		StageStatuses: make([]int, n),
		Background:    pipeline.Background,
	}
	job.JobID = sh.NextJobID
	sh.NextJobID++
	if !job.Background {
		sh.ForegroundJob = job
	}
	sh.Jobs[job.JobID] = job
	cleanUp := func() {
		sh.ForegroundJob = nil
		if jobComplete(job) {
			delete(sh.Jobs, job.JobID)
		}
	}
	pg := &ProcessGroup{onLeader: func(pgid int) { job.PGID = pgid }}
	if sh.Interactive {
		pg.foreground = !pipeline.Background
		pg.ttyFD = int(sh.Terminal.Fd())
	}
	records := make([]*shell.Process, n)
	for i, cmd := range cmds {
		if cmd.Name == "" {
			continue
		}
		if _, builtin := builtins.Lookup(cmd.Name); builtin {
			continue
		}
		record := &shell.Process{Command: cmd, Stage: i}
		records[i] = record
		job.Processes = append(job.Processes, record)
	}

	ins := make([]io.Reader, n)
	outs := make([]io.Writer, n)
	// Each pipe end belongs to one stage, which closes it when done.
	owned := make([][]io.Closer, n)
	ins[0] = sh.Stdin
	outs[n-1] = sh.Stdout
	for i := 0; i < n-1; i++ {
		r, w, err := os.Pipe()
		if err != nil {
			for _, group := range owned {
				for _, closer := range group {
					_ = closer.Close()
				}
			}
			fmt.Fprintf(sh.Stderr, "minishell: pipe: %v\n", err)
			sh.ForegroundJob = nil
			delete(sh.Jobs, job.JobID)
			return func() int { return 1 }
		}
		outs[i] = w
		ins[i+1] = r
		owned[i] = append(owned[i], w)
		owned[i+1] = append(owned[i+1], r)
	}

	finishes := make([]func() int, n)
	for i := range cmds {
		finishes[i] = startCommand(sh, cmds[i], ins[i], outs[i], sh.Stderr, pg, records[i], job.JobID, i)
	}

	startStageWorkers(sh, job, finishes, owned)
	if pipeline.Background {
		fmt.Fprintf(sh.Stdout, "[%d] %d\n", job.JobID, job.PGID)
		return func() int { return 0 }
	}
	return func() int {
		defer cleanUp()
		status := waitForegroundJob(sh, job)
		if sh.Interactive {
			if err := setTerminalForegroundPGID(sh.Terminal.Fd(), sh.ShellPGID); err != nil {
				fmt.Fprintf(sh.Stderr, "minishell: restore terminal: %v\n", err)
				return 1
			}
			if job.State() == shell.ProcessStopped {
				fmt.Fprintf(sh.Stdout, "[%d] Stopped\n", job.JobID)
			}
		}
		return status
	}
}

func startStageWorkers(sh *shell.Shell, job *shell.Job, finishes []func() int, owned [][]io.Closer) {
	for i := range finishes {
		go func(i int) {
			status := finishes[i]()
			for _, closer := range owned[i] {
				_ = closer.Close()
			}
			sh.JobEvents <- shell.JobEvent{
				JobID:      job.JobID,
				Stage:      i,
				State:      shell.ProcessDone,
				ExitStatus: status,
			}
		}(i)
	}
}
