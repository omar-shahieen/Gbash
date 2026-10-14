package parser

import (
	"testing"
)

func TestParseList(t *testing.T) {
	tokens, err := Tokenize("echo hello | wc -c && echo yes || echo no")
	if err != nil {
		t.Fatal(err)
	}
	list, err := ParseList(tokens)
	if err != nil {
		t.Fatal(err)
	}
	conditions := []RunCondition{RunAlways, RunOnSuccess, RunOnFailure}
	if len(list) != len(conditions) {
		t.Fatalf("entries = %d, want %d", len(list), len(conditions))
	}
	for i, condition := range conditions {
		if list[i].Condition != condition {
			t.Errorf("entry %d condition = %v, want %v", i, list[i].Condition, condition)
		}
	}
	first, err := ParsePipeline(list[0].Tokens)
	if err != nil || len(first.Commands) != 2 || first.Commands[0].Args[0] != "hello" {
		t.Fatalf("first pipeline = %+v, error = %v", first, err)
	}
	for i, want := range []string{"yes", "no"} {
		pipeline, err := ParsePipeline(list[i+1].Tokens)
		if err != nil || pipeline.Commands[0].Args[0] != want {
			t.Fatalf("conditional pipeline = %+v, error = %v", pipeline, err)
		}
	}
}

func TestParseListStandaloneBackground(t *testing.T) {
	tokens, _ := Tokenize("sleep 1 &")
	list, err := ParseList(tokens)
	if err != nil || len(list) != 1 || list[0].Condition != RunAlways {
		t.Fatalf("list = %+v, error = %v", list, err)
	}
	pipeline, err := ParsePipeline(list[0].Tokens)
	if err != nil || !pipeline.Background {
		t.Fatalf("pipeline = %+v, error = %v", pipeline, err)
	}
}

func TestParseListPreservesExpansionParts(t *testing.T) {
	tokens, err := Tokenize(`export NAME=omar && echo '$HOME' "$NAME"`)
	if err != nil {
		t.Fatal(err)
	}
	list, err := ParseList(tokens)
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v, error = %v", list, err)
	}
	words := list[1].Tokens
	if words[1].Value != "$HOME" || len(words[1].Parts) != 1 || words[1].Parts[0].Expand {
		t.Fatalf("literal metadata lost: %+v", words[1])
	}
	if words[2].Value != "$NAME" || len(words[2].Parts) != 1 || !words[2].Parts[0].Expand {
		t.Fatalf("expansion metadata lost: %+v", words[2])
	}
}

func TestParseListErrors(t *testing.T) {
	for _, input := range []string{
		"", "&& true", "|| true", "true &&", "true ||",
		"true && || false", "true || && false", "true && && false",
		"sleep 1 & && echo yes", "true && sleep 1 &", "true || sleep 1 &",
	} {
		t.Run(input, func(t *testing.T) {
			tokens, err := Tokenize(input)
			if err != nil {
				t.Fatal(err)
			}
			list, err := ParseList(tokens)
			if err == nil {
				t.Fatal("expected a syntax error")
			}
			if list != nil {
				t.Fatal("invalid input returned a partial list")
			}
		})
	}
}
