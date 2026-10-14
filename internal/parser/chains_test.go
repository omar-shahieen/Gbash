package parser

import (
	"reflect"
	"testing"
)

func TestParseChains(t *testing.T) {
	for _, tc := range []struct {
		input      string
		conditions [][]RunCondition
		background []bool
	}{
		{"echo a && echo b & echo c", [][]RunCondition{{RunAlways, RunOnSuccess}, {RunAlways}}, []bool{true, false}},
		{"false || echo fallback;", [][]RunCondition{{RunAlways, RunOnFailure}}, []bool{false}},
		{"sleep 1 &", [][]RunCondition{{RunAlways}}, []bool{true}},
		{"echo a", [][]RunCondition{{RunAlways}}, []bool{false}},
		{"echo one;", [][]RunCondition{{RunAlways}}, []bool{false}},
		{"echo one; echo two;", [][]RunCondition{{RunAlways}, {RunAlways}}, []bool{false, false}},
		{"false && echo skipped;", [][]RunCondition{{RunAlways, RunOnSuccess}}, []bool{false}},
		{"false && echo skipped; echo final", [][]RunCondition{{RunAlways, RunOnSuccess}, {RunAlways}}, []bool{false, false}},
		{"sleep 1 & echo ready", [][]RunCondition{{RunAlways}, {RunAlways}}, []bool{true, false}},
		{"sleep 1 & sleep 2 &", [][]RunCondition{{RunAlways}, {RunAlways}}, []bool{true, true}},
		{"echo ready; sleep 1 &", [][]RunCondition{{RunAlways}, {RunAlways}}, []bool{false, true}},
		{"sleep 1 & true && echo ready", [][]RunCondition{{RunAlways}, {RunAlways, RunOnSuccess}}, []bool{true, false}},
		{"true && echo ready; sleep 1 &", [][]RunCondition{{RunAlways, RunOnSuccess}, {RunAlways}}, []bool{false, true}},
		{"false || echo ready; sleep 1 & echo done", [][]RunCondition{{RunAlways, RunOnFailure}, {RunAlways}, {RunAlways}}, []bool{false, true, false}},
		{`export NAME=omar && echo '$HOME' "$NAME"`, [][]RunCondition{{RunAlways, RunOnSuccess}}, []bool{false}},
		{"true && sleep 1 &", [][]RunCondition{{RunAlways, RunOnSuccess}}, []bool{true}},
		{"true || sleep 1 &", [][]RunCondition{{RunAlways, RunOnFailure}}, []bool{true}},
		{"true && sleep 1 & echo ready", [][]RunCondition{{RunAlways, RunOnSuccess}, {RunAlways}}, []bool{true, false}},
		{"false || sleep 1 & echo ready", [][]RunCondition{{RunAlways, RunOnFailure}, {RunAlways}}, []bool{true, false}},
		{"echo first; true && sleep 1 & echo ready", [][]RunCondition{{RunAlways}, {RunAlways, RunOnSuccess}, {RunAlways}}, []bool{false, true, false}},
		{`echo '$HOME' && echo "$!" &`, [][]RunCondition{{RunAlways, RunOnSuccess}}, []bool{true}},
		{"echo a | cat && echo b || echo c &", [][]RunCondition{{RunAlways, RunOnSuccess, RunOnFailure}}, []bool{true}},
		{"echo a; echo b & echo c && echo d;", [][]RunCondition{{RunAlways}, {RunAlways}, {RunAlways, RunOnSuccess}}, []bool{false, true, false}},
	} {
		t.Run(tc.input, func(t *testing.T) {
			tokens, err := Tokenize(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			chains, err := ParseChains(tokens)
			if err != nil {
				t.Fatal(err)
			}
			if len(chains) != len(tc.conditions) {
				t.Fatalf("chains = %d, want %d", len(chains), len(tc.conditions))
			}
			// Every pipeline's original tokens must remain in order, without list separators.
			var words []Token
			for i, chain := range chains {
				if chain.Background != tc.background[i] || len(chain.Entries) != len(tc.conditions[i]) {
					t.Fatalf("chain %d: background = %v, entries = %d", i, chain.Background, len(chain.Entries))
				}
				for j, entry := range chain.Entries {
					if entry.Condition != tc.conditions[i][j] {
						t.Errorf("chain %d entry %d: condition = %v, want %v", i, j, entry.Condition, tc.conditions[i][j])
					}
					pipeline, err := ParsePipeline(entry.Tokens)
					if err != nil || pipeline.Background {
						t.Fatalf("entry pipeline = %+v, error = %v", pipeline, err)
					}
					words = append(words, entry.Tokens...)
				}
			}
			var want []Token
			for _, token := range tokens {
				switch token.Type {
				case AndIf, OrIf, Semicolon, Background:
				default:
					want = append(want, token)
				}
			}
			if !reflect.DeepEqual(words, want) {
				t.Fatalf("entry tokens = %+v, want %+v", words, want)
			}
		})
	}
}

func TestParseChainsPipelineContents(t *testing.T) {
	tokens, err := Tokenize("echo hello | wc -c && echo yes || echo no")
	if err != nil {
		t.Fatal(err)
	}
	chains, err := ParseChains(tokens)
	if err != nil || len(chains) != 1 || len(chains[0].Entries) != 3 {
		t.Fatalf("chains = %+v, error = %v", chains, err)
	}
	entries := chains[0].Entries
	first, err := ParsePipeline(entries[0].Tokens)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Commands) != 2 || first.Commands[0].Name != "echo" ||
		!reflect.DeepEqual(first.Commands[0].Args, []string{"hello"}) ||
		first.Commands[1].Name != "wc" || !reflect.DeepEqual(first.Commands[1].Args, []string{"-c"}) {
		t.Fatalf("first pipeline = %+v", first)
	}
	for i, want := range []string{"yes", "no"} {
		pipeline, err := ParsePipeline(entries[i+1].Tokens)
		if err != nil {
			t.Fatal(err)
		}
		if len(pipeline.Commands) != 1 || pipeline.Commands[0].Name != "echo" ||
			!reflect.DeepEqual(pipeline.Commands[0].Args, []string{want}) {
			t.Fatalf("conditional pipeline = %+v", pipeline)
		}
	}
}

func TestParseChainsErrors(t *testing.T) {
	for _, input := range []string{
		"", "&", ";", "&& echo a", "|| echo a", "echo a &&", "echo a ||",
		"echo a &; echo b", "echo a ;; echo b", "echo a && &", "echo a ||;",
		"echo a & & echo b", "echo a && || echo b", "echo a; cat >",
		"echo a | &", "echo a; | cat", "echo a && cat > &",
		"true || && false", "true && && false", "sleep 1 & && echo yes",
		"; echo one", "true &&;",
	} {
		t.Run(input, func(t *testing.T) {
			tokens, err := Tokenize(input)
			if err != nil {
				t.Fatal(err)
			}
			chains, err := ParseChains(tokens)
			if err == nil || chains != nil {
				t.Fatalf("chains = %+v, error = %v; want nil chains and a syntax error", chains, err)
			}
		})
	}
}
