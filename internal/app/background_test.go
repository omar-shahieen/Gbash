package app

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestBackgroundSequences(t *testing.T) {
	binary := buildTestShell(t)

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
		{"foreground does not consume job ID", "true; exit 7 & wait %1; echo status=$?", []string{"status=7"}},
		{"empty table restarts job IDs", "exit 3 & wait %1; exit 9 & wait %1; echo status=$?; jobs", []string{"status=9"}},
		{"retained job ID stays stable", "sleep 0.1 & true; exit 9 & wait %2; echo second=$?; wait %1; echo first=$?; jobs", []string{"second=9", "first=0"}},
		{"background conditional fallback", "false || echo fallback & wait", []string{"fallback"}},
		{"background chain environment isolation", "export MINIBASH_BG_CHECK=parent; export MINIBASH_BG_CHECK=child && echo \"$MINIBASH_BG_CHECK\" & wait; echo \"$MINIBASH_BG_CHECK\"", []string{"child", "parent"}},
		{"background chain exit status", "true && exit 7 & wait $!; echo status=$?; jobs", []string{"status=7"}},
		{"background chain skips expansion", "true || echo ${ & wait $!; echo status=$?", []string{"status=0"}},
		{"independent background chain statuses", "true && exit 3 & true && exit 9 & wait %2; echo second=$?; wait %1; echo first=$?; jobs", []string{"second=9", "first=3"}},
		{"independent background chain environments", "export MINIBASH_BG_CHECK=parent; export MINIBASH_BG_CHECK=first && echo \"$MINIBASH_BG_CHECK\" > first & export MINIBASH_BG_CHECK=second && echo \"$MINIBASH_BG_CHECK\" > second & wait; cat first second; echo \"$MINIBASH_BG_CHECK\"", []string{"first", "second", "parent"}},
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
