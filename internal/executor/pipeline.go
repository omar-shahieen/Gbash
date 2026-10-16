package executor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/omar-shahieen/gbash/internal/builtins"
	"github.com/omar-shahieen/gbash/internal/shell"
)

// pipelineStage keeps one command, its process record, and its pipe ownership together.
type pipelineStage struct {
	index    int
	command  shell.Command
	process  *shell.Process
	streams  commandStreams
	pipeEnds []io.Closer
	finish   func() int
}

func (stage pipelineStage) closePipes() {
	for _, end := range stage.pipeEnds {
		_ = end.Close()
	}
}

func executePipeline(sh *shell.Shell, pipeline shell.Pipeline) int {
	if len(pipeline.Commands) == 0 {
		return 0
	}
	job := &shell.Job{
		Pipeline:      pipeline,
		StageDone:     make([]bool, len(pipeline.Commands)),
		StageStatuses: make([]int, len(pipeline.Commands)),
		Background:    pipeline.Background,
		CommandText:   pipeline.CommandText,
	}
	registerJob(sh, job)

	stages, err := preparePipeline(sh, job)
	if err != nil {
		delete(sh.Jobs, job.JobID)
		fmt.Fprintf(sh.Stderr, "minishell: pipe: %v\n", err)
		return 1
	}
	group := &processGroup{id: sh.ChildPGID}
	if sh.Interactive {
		group.foreground = !job.Background
		group.ttyFD = int(sh.Terminal.Fd())
	}
	// Launch all children before any worker starts waiting for one to exit.
	for i := range stages {
		stages[i].finish = startStage(sh, job, stages[i], group)
	}
	job.PGID = group.id
	startStageWorkers(sh, job.JobID, stages)

	if job.Background {
		for i := len(job.Processes) - 1; i >= 0; i-- {
			if job.Processes[i].PID > 0 {
				sh.LastBackgroundPID = job.Processes[i].PID
				break
			}
		}
		fmt.Fprintf(sh.Stdout, "[%d] %d\n", job.JobID, job.PGID)
		return 0
	}
	return runForegroundJob(sh, job, sh.Stdout, sh.Stderr)
}

func preparePipeline(sh *shell.Shell, job *shell.Job) ([]pipelineStage, error) {
	stages := make([]pipelineStage, len(job.Pipeline.Commands))
	for i, cmd := range job.Pipeline.Commands {
		stages[i] = pipelineStage{index: i, command: cmd, streams: shellStreams(sh)}
		if cmd.Name != "" {
			process := &shell.Process{Command: cmd, Stage: i}
			stages[i].process = process
			job.Processes = append(job.Processes, process)
		}
	}
	for i := 0; i < len(stages)-1; i++ {
		reader, writer, err := os.Pipe()
		if err != nil {
			for _, stage := range stages {
				stage.closePipes()
			}
			return nil, err
		}
		stages[i].streams.stdout = writer
		stages[i+1].streams.stdin = reader
		stages[i].pipeEnds = append(stages[i].pipeEnds, writer)
		stages[i+1].pipeEnds = append(stages[i+1].pipeEnds, reader)
	}
	return stages, nil
}

// startStage launches a program or built-in as a child and returns its one-use
// completion function, which waits for termination and closes redirection files.
func startStage(sh *shell.Shell, job *shell.Job, stage pipelineStage, group *processGroup) func() int {
	streams, err := prepareRedirections(stage.command, stage.streams)
	if err != nil {
		fmt.Fprintf(sh.Stderr, "minishell: %v\n", err)
		return failedStage(stage.process, 1)
	}
	if stage.command.Name == "" {
		streams.close()
		return func() int { return 0 }
	}
	process, err := startChildCommand(stage.command, streams, group, sh)
	if err != nil {
		status := reportLaunchError(stage.command.Name, err, streams.stderr)
		streams.close()
		return failedStage(stage.process, status)
	}
	stage.process.PID = process.Pid
	stage.process.State = shell.ProcessRunning
	return func() int {
		defer streams.close()
		return watchExternalCommand(sh, process, job.JobID, stage.index, streams.stderr)
	}
}

func failedStage(process *shell.Process, status int) func() int {
	if process != nil {
		process.State = shell.ProcessDone
		process.ExitStatus = status
	}
	return func() int { return status }
}

func startChildCommand(cmd shell.Command, streams commandStreams, group *processGroup, sh *shell.Shell) (*os.Process, error) {
	var path string
	var err error
	var args []string
	if _, ok := builtins.Lookup(cmd.Name); ok {
		path, err = os.Executable()
		if err != nil {
			return nil, err
		}
		args = append(
			[]string{path, "--builtin-child",
				strconv.Itoa(sh.LastStatus),
				cmd.Name},
			cmd.Args...,
		)
	} else {
		path, err = exec.LookPath(cmd.Name)
		if err != nil {
			return nil, err
		}
		args = append([]string{cmd.Name}, cmd.Args...)
	}

	files, err := streams.childFiles()
	if err != nil {
		return nil, err
	}
	if cmd.Name == "history" || cmd.InheritHistory {
		// An unlinked file carries the in-memory snapshot without argv size limits,
		// environment leakage, or a pipe writer that can block during child startup.
		history, err := os.CreateTemp("", "gbash-child-history-*")
		if err != nil {
			return nil, err
		}
		defer history.Close()
		if err := os.Remove(history.Name()); err != nil {
			return nil, err
		}
		if err := json.NewEncoder(history).Encode(sh.History); err != nil {
			return nil, err
		}
		if _, err := history.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		files = append(files, history) // Child descriptor 3.
		args = append([]string{args[0], "--history-child"}, args[1:]...)
	}
	return group.launch(path, args, files)
}

func reportLaunchError(name string, err error, output io.Writer) int {
	var launchErr *exec.Error
	if errors.As(err, &launchErr) && errors.Is(launchErr.Err, exec.ErrNotFound) {
		fmt.Fprintf(output, "minishell: %s: command not found\n", name)
		return 127
	}
	fmt.Fprintf(output, "minishell: %s: %v\n", name, err)
	return 126
}

func startStageWorkers(sh *shell.Shell, jobID int, stages []pipelineStage) {
	for _, stage := range stages {
		go func(stage pipelineStage) {
			status := stage.finish()
			stage.closePipes()
			sh.JobEvents <- shell.JobEvent{
				JobID: jobID, Stage: stage.index, State: shell.ProcessDone, ExitStatus: status,
			}
		}(stage)
	}
}
