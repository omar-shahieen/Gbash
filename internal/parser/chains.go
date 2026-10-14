package parser

import (
	"fmt"
)

type RunCondition int

const (
	RunAlways RunCondition = iota
	RunOnSuccess
	RunOnFailure
)

type ListEntry struct {
	Tokens    []Token
	Condition RunCondition
}

type CommandChain struct {
	Entries    []ListEntry
	Background bool
}

// ParseChains groups pipelines connected by && or ||. A ; or & ends a chain.
func ParseChains(tokens []Token) ([]CommandChain, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("syntax error: empty list")
	}

	var chains []CommandChain
	var chain CommandChain
	start := 0
	condition := RunAlways
	appendEntry := func(segment []Token) error {
		if _, err := ParsePipeline(segment); err != nil {
			return err
		}
		chain.Entries = append(chain.Entries, ListEntry{Tokens: segment, Condition: condition})
		return nil
	}

	for i, tok := range tokens {
		if tok.Type != AndIf && tok.Type != OrIf && tok.Type != Semicolon && tok.Type != Background {
			continue
		}
		if i == start {
			return nil, fmt.Errorf("syntax error: empty command before %s", tok.Value)
		}
		if err := appendEntry(tokens[start:i]); err != nil {
			return nil, err
		}
		switch tok.Type {
		case AndIf:
			condition = RunOnSuccess
		case OrIf:
			condition = RunOnFailure
		case Semicolon, Background:
			chain.Background = tok.Type == Background
			chains = append(chains, chain)
			chain = CommandChain{}
			condition = RunAlways
		}
		start = i + 1
	}

	if start < len(tokens) {
		if err := appendEntry(tokens[start:]); err != nil {
			return nil, err
		}
		chains = append(chains, chain)
	} else if last := tokens[len(tokens)-1].Type; last != Semicolon && last != Background {
		return nil, fmt.Errorf("syntax error: missing pipeline after conditional operator")
	}
	return chains, nil
}
