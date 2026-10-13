# MiniBash

A small Linux shell written in Go, organized around reading commands, launching
pipelines, and tracking jobs.

## Run and check

```bash
go build -o Gbash .
./Gbash
go test ./...
```

Build the whole package (`.`), since startup, the input loop, and signal handling
live in separate files.

## Where to read the code

Start with `main.go`, then follow `runREPL` in `repl.go` and `executor.Execute`.

| Location | Responsibility |
| --- | --- |
| `main.go` | Start the shell and arrange shutdown cleanup |
| `repl.go` | Prompt, read one line, parse, execute, repeat |
| `signals.go` | Convert SIGTERM into shutdown notification and handle prompt signals |
| `internal/parser/lexer.go` | Turn characters into words and operator tokens |
| `internal/parser/parser.go` | Build commands, redirections, and a pipeline |
| `internal/shell/pipeline.go` | Parsed command and pipeline types |
| `internal/shell/job.go` | Process records, job state, completion, and event updates |
| `internal/shell/shell.go` | Mutable shell state and ordered job IDs |
| `internal/builtins/builtins.go` | Built-in registration and lookup |
| `internal/builtins/commands.go` | Directory, output, exit, type, and jobs commands |
| `internal/builtins/environment.go` | Export, unset, and environment-name validation |
| `internal/executor/executor.go` | Choose built-in, job-control, or pipeline execution |
| `internal/executor/pipeline.go` | Prepare stages, launch programs, start completion workers |
| `internal/executor/redirections.go` | Named command streams and redirection-file ownership |
| `internal/executor/job_control.go` | fg, bg, wait argument handling and resume operations |
| `internal/executor/jobs.go` | Receive events, wait, notify, and shut down jobs |
| `internal/executor/process_linux.go` | Linux process groups, terminal ownership, and Wait4 |
| `examples/signals/main.go` | Standalone signal experiment, separate from the shell |

## The execution flow

1. The REPL reads one line. It does not continuously read while a foreground
   command may need terminal input.
2. The lexer and parser produce a pipeline.
3. The executor runs standalone built-ins in the shell or prepares pipeline stages.
4. Each external stage joins the pipeline's process group.
5. Each stage has one worker. That worker waits for its child and publishes events.
6. The main shell consumes events and updates job records.
7. Foreground execution waits; background execution returns the prompt.

## Resource and state ownership

- A command's streams own files opened for its redirections.
- A pipeline stage owns its parent-side pipe ends.
- Files and pipes close when that stage finishes, before its completion event.
- Only the main event receiver updates shared process records after launch.
- `Job.State()` describes child-process state. `Job.Complete()` checks every
  pipeline stage, including built-ins without child processes.
- Completed background records remain available for `wait`; notification alone
  does not discard their exit status.

## Current boundaries

Quotes, expansions, and conditional operators are not implemented yet.
Background built-ins and job-control commands inside pipelines are rejected.
Built-ins inside foreground pipelines still run in shell goroutines, so stateful
built-in isolation remains a separate implementation task.

Shutdown sends SIGHUP, resumes stopped groups, and waits up to two seconds.
Programs that ignore SIGHUP may survive that deadline. SIGKILL cannot be handled.
