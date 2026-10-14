package parser

import "testing"

func TestTrailingSemicolon(t *testing.T) {
	cases := []struct {
		input      string
		conditions []RunCondition
	}{
		{"echo one;", []RunCondition{RunAlways}},
		{"echo one; echo two;", []RunCondition{RunAlways, RunAlways}},
		{"false && echo skipped;", []RunCondition{RunAlways, RunOnSuccess}},
		{"false && echo skipped; echo final", []RunCondition{RunAlways, RunOnSuccess, RunAlways}},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			tokens, err := Tokenize(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			list, err := ParseList(tokens)
			if err != nil {
				t.Fatal(err)
			}
			if len(list) != len(tc.conditions) {
				t.Fatalf("entries = %d, want %d", len(list), len(tc.conditions))
			}
			for i, want := range tc.conditions {
				if list[i].Condition != want {
					t.Errorf("entry %d condition = %v, want %v", i, list[i].Condition, want)
				}
			}
		})
	}
}

func TestInvalidSequenceSeparators(t *testing.T) {
	for _, input := range []string{";", "; echo one", "echo one;; echo two", "true &&", "true ||", "true &&;", "sleep 1 &; echo ready"} {
		t.Run(input, func(t *testing.T) {
			tokens, err := Tokenize(input)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseList(tokens); err == nil {
				t.Fatal("expected a syntax error")
			}
		})
	}
}
