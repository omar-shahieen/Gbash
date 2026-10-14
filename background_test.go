package main

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackgroundSequences(t *testing.T) {
	// Use the real binary so child builtins enter --builtin-child mode.
	binary := filepath.Join(t.TempDir(), "minibash")
	buildContext, cancelBuild := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelBuild()
	if output, err := exec.CommandContext(buildContext, "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build minibash: %v\n%s", err, output)
	}

	for _, tc := range []struct {
		name, input string
		messages    []string
	}{
		{"continue then wait", "sleep 0.1 & echo ready; wait; echo finished", []string{"ready", "finished"}},
		{"wait removes jobs", "sleep 0.1 & sleep 0.1 & wait; jobs", nil},
		{
			name: "background export stays in child",
			input: "export MINIBASH_BG_CHECK=parent; " +
				"export MINIBASH_BG_CHECK=child & wait; " +
				"echo \"$MINIBASH_BG_CHECK\"",
			messages: []string{"parent"},
		},
		{"background exit keeps parent alive", "exit 7 & wait; echo alive", []string{"alive"}},
		{"wait preserves child exit status", "exit 7 & wait %1; echo $?; jobs", []string{"7"}},
		{"background child inherits last status", "type minibash_unknown_command_875 2>/dev/null; exit & wait %1; echo $?", []string{"1"}},
		{"background unset stays in child", "export MINIBASH_BG_CHECK=parent; unset MINIBASH_BG_CHECK & wait; echo \"$MINIBASH_BG_CHECK\"", []string{"parent"}},
		{"background cd stays in child", "cd / & wait; pwd", []string{"WORKDIR"}},
		{"background builtin pipeline", "echo piped | cat & wait; jobs", []string{"piped"}},
		{"background output redirection", "echo redirected > result & wait; cat result", []string{"redirected"}},
		{"wait without jobs", "wait; echo $?", []string{"0"}},
		{"wait using background PID", "exit 7 & wait $!; echo status=$?; jobs", []string{"status=7"}},
		{"wait using last pipeline PID", "sh -c 'exit 3' | sh -c 'exit 9' & wait $!; echo status=$?; wait; jobs", []string{"status=9"}},
		{"latest background PID", "exit 3 & exit 9 & wait $!; echo status=$?; wait; jobs", []string{"status=9"}},
		{"foreground preserves background PID", "exit 7 & export SAVED_PID=$!; true; test \"$!\" = \"$SAVED_PID\"; echo same=$?; wait $!; echo status=$?", []string{"same=0", "status=7"}},
		{"wait preserves background PID", "exit 7 & export SAVED_PID=$!; wait $!; test \"$!\" = \"$SAVED_PID\"; echo same=$?", []string{"same=0"}},
		{"failed launch preserves background PID", "exit 7 & export SAVED_PID=$!; minibash_unknown_command_875 2>/dev/null & test \"$!\" = \"$SAVED_PID\"; echo same=$?; wait", []string{"same=0"}},
		{"failed final stage uses last launched PID", "exit 7 | minibash_unknown_command_875 2>/dev/null & wait $!; echo status=$?; wait", []string{"status=7"}},
		{"no background PID", "echo \"pid=[$!]\"", []string{"pid=[]"}},
		{"quoted background PID stays literal", `exit 7 & echo '$!'; echo "\$!"; wait`, []string{"$!", "$!"}},
		// The child writes a marker before exiting. This checks that the shell
		// continues before completion and that wait delays the final command.
		{"completion order", "sh -c 'sleep 0.1; echo completed' & echo ready; wait; echo finished", []string{"ready", "completed", "finished"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary)
			cmd.Dir = t.TempDir()
			cmd.Stdin = strings.NewReader(tc.input + "\n")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			cmd.WaitDelay = time.Second
			if err := cmd.Run(); err != nil {
				t.Fatalf("run minibash: %v\nstdout: %s\nstderr: %s", err, &stdout, &stderr)
			}
			if stderr.Len() != 0 {
				t.Fatalf("unexpected stderr: %s", &stderr)
			}
			output := stdout.String()
			if strings.Contains(output, "pgid=") {
				t.Fatalf("jobs remained after wait: %s", output)
			}
			// Ignore prompts and job launch notices when finding command output.
			remaining := "\n" + strings.ReplaceAll(output, prompt, "")
			for _, message := range tc.messages {
				if message == "WORKDIR" {
					message = cmd.Dir
				}
				marker := "\n" + message + "\n"
				index := strings.Index(remaining, marker)
				if index < 0 {
					t.Fatalf("missing or out-of-order %q in output: %s", message, output)
				}
				remaining = remaining[index+len(marker)-1:]
			}
		})
	}
}
