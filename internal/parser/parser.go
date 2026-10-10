package parser

import (
	"fmt"

	"github.com/omar-shahieen/Gbash/internal/shell"
)

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

// Parse builds one command.
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
	start := 0

	for i, token := range tokens {
		if token.Type != Pipe {
			continue
		}

		// Reject a pipe without a command before it.
		if i == start {
			return shell.Pipeline{}, fmt.Errorf("syntax error: empty command before pipe")
		}

		cmd, err := ParseCommand(tokens[start:i])
		if err != nil {
			return shell.Pipeline{}, err
		}

		pipeline.Commands = append(pipeline.Commands, cmd)
		start = i + 1
	}

	// Reject a trailing pipe.
	if start == len(tokens) {
		return shell.Pipeline{}, fmt.Errorf("syntax error: empty command after pipe")
	}

	// Parse the final command (or the only command).
	cmd, err := ParseCommand(tokens[start:])
	if err != nil {
		return shell.Pipeline{}, err
	}

	pipeline.Commands = append(pipeline.Commands, cmd)

	return pipeline, nil
}
