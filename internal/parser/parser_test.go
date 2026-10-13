package parser

import (
	"testing"

	"github.com/omar-shahieen/gbash/internal/shell"
)

func TestPipelineSyntax(t *testing.T) {
	cases := []struct {
		input      string
		stages     int
		background bool
		invalid    bool
	}{
		{input: "sleep 30", stages: 1},
		{input: "sleep 30&", stages: 1, background: true},
		{input: "sleep 30 | cat &", stages: 2, background: true},
		{input: "> out", stages: 1},
		{input: "", invalid: true},
		{input: "&", invalid: true},
		{input: "sleep & echo hello", invalid: true},
		{input: "sleep && echo hello", invalid: true},
		{input: "sleep & &", invalid: true},
		{input: "| cat", invalid: true},
		{input: "cat |", invalid: true},
		{input: "cat || wc", invalid: true},
		{input: "echo > &", invalid: true},
		{input: "echo > | cat", invalid: true},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			tokens, err := Tokenize(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			pipeline, err := ParsePipeline(tokens)
			if (err != nil) != tc.invalid {
				t.Fatalf("error = %v, invalid = %v", err, tc.invalid)
			}
			if tc.invalid {
				return
			}
			if len(pipeline.Commands) != tc.stages || pipeline.Background != tc.background {
				t.Fatalf("pipeline = %+v", pipeline)
			}
		})
	}
}

func TestRedirectionOperators(t *testing.T) {
	cases := []struct {
		input string
		fd    int
		mode  shell.RedirMode
	}{
		{"cat < file", 0, shell.ModeRead},
		{"echo hi > file", 1, shell.ModeTruncate},
		{"echo hi >> file", 1, shell.ModeAppend},
		{"cat 2> file", 2, shell.ModeTruncate},
		{"cat 2>> file", 2, shell.ModeAppend},
		{"echo 2 > file", 1, shell.ModeTruncate},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			tokens, _ := Tokenize(tc.input)
			cmd, err := ParseCommand(tokens)
			if err != nil {
				t.Fatal(err)
			}
			if len(cmd.Redirections) != 1 {
				t.Fatalf("redirections = %+v", cmd.Redirections)
			}
			got := cmd.Redirections[0]
			if got.FD != tc.fd || got.Mode != tc.mode || got.Filename != "file" {
				t.Fatalf("redirection = %+v", got)
			}
		})
	}
}
