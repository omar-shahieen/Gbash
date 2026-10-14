package parser

import (
	"fmt"

	"github.com/omar-shahieen/gbash/internal/shell"
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

// redirTable maps each operator token to its fd and mode, once.
var redirTable = map[TokenKind]struct {
	fd   int
	mode shell.RedirMode
}{
	RedirIn:        {0, shell.ModeRead},
	RedirOut:       {1, shell.ModeTruncate},
	RedirAppend:    {1, shell.ModeAppend},
	RedirErr:       {2, shell.ModeTruncate},
	RedirErrAppend: {2, shell.ModeAppend},
}

func ParseRedirections(tokens []Token) ([]string, []shell.Redirection, error) {
	var words []string
	var redirs []shell.Redirection

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]

		if tok.Type == Word {
			words = append(words, tok.Value)
			continue
		}

		info, ok := redirTable[tok.Type]
		if !ok {
			if tok.Type == Background {
				return nil, nil, fmt.Errorf("syntax error: & must appear at the end of a pipeline")
			}
			return nil, nil, fmt.Errorf("syntax error: unexpected token %q", tok.Value)
		}

		if i+1 >= len(tokens) || tokens[i+1].Type != Word {
			return nil, nil, fmt.Errorf("syntax error: missing filename after %s", tok.Value)
		}

		redirs = append(redirs, shell.Redirection{
			FD:       info.fd,
			Mode:     info.mode,
			Filename: tokens[i+1].Value,
		})
		i++ // skip the filename
	}

	return words, redirs, nil
}

// ParseCommand builds one command from its arguments and redirections.
func ParseCommand(tokens []Token) (shell.Command, error) {
	words, redirs, err := ParseRedirections(tokens)
	if err != nil {
		return shell.Command{}, err
	}

	cmd := shell.Command{Redirections: redirs}
	if len(words) > 0 {
		cmd.Name = words[0]
		cmd.Args = words[1:]
	}
	return cmd, nil
}

func ParsePipeline(tokens []Token) (shell.Pipeline, error) {
	var pipeline shell.Pipeline

	if len(tokens) == 0 {
		return shell.Pipeline{}, fmt.Errorf("syntax error: empty pipeline")
	}

	if tokens[len(tokens)-1].Type == Background {
		pipeline.Background = true
		tokens = tokens[:len(tokens)-1]
	}

	if len(tokens) == 0 {
		return shell.Pipeline{},
			fmt.Errorf("syntax error: missing command before &")
	}
	start := 0
	// Treat the end of input like a separator, so every stage follows one path.
	for i := 0; i <= len(tokens); i++ {
		if i < len(tokens) && tokens[i].Type != Pipe {
			continue
		}
		if i == start {
			if i == len(tokens) {
				return shell.Pipeline{}, fmt.Errorf("syntax error: empty command after pipe")
			}
			return shell.Pipeline{}, fmt.Errorf("syntax error: empty command before pipe")
		}
		cmd, err := ParseCommand(tokens[start:i])
		if err != nil {
			return shell.Pipeline{}, err
		}
		pipeline.Commands = append(pipeline.Commands, cmd)
		start = i + 1
	}
	return pipeline, nil
}

func ParseList(tokens []Token) ([]ListEntry, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("syntax error: empty list")
	}

	var list []ListEntry
	start := 0
	condition := RunAlways
	chainHasConditional := false
	appendEntry := func(segment []Token) error {
		_, err := ParsePipeline(segment)
		if err != nil {
			return err
		}
		list = append(list, ListEntry{Tokens: segment, Condition: condition})
		return nil
	}

	for i, tok := range tokens {
		if tok.Type != AndIf && tok.Type != OrIf && tok.Type != Semicolon && tok.Type != Background {
			continue
		}
		if i == start {
			return nil, fmt.Errorf("syntax error: empty command before %s", tok.Value)
		}
		var err error
		if tok.Type == Background {
			if chainHasConditional {
				return nil, fmt.Errorf("syntax error: background conditional chains are not supported")
			}
			err = appendEntry(tokens[start : i+1])
		} else {
			err = appendEntry(tokens[start:i])
		}

		if err != nil {
			return nil, err
		}
		switch tok.Type {
		case AndIf:
			chainHasConditional = true
			condition = RunOnSuccess
		case OrIf:
			chainHasConditional = true
			condition = RunOnFailure
		default:
			chainHasConditional = false
			condition = RunAlways
		}
		start = i + 1
	}

	if start < len(tokens) {
		if err := appendEntry(tokens[start:]); err != nil {
			return nil, err
		}
	} else if last := tokens[len(tokens)-1].Type; last != Semicolon && last != Background {
		return nil, fmt.Errorf("syntax error: missing pipeline after conditional operator")
	}

	return list, nil
}
