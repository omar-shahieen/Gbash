package parser

import (
	"strings"
	"unicode"
)

type TokenKind int

const (
	Word TokenKind = iota
	Pipe
	RedirOut
	RedirAppend
	RedirIn
	RedirErr
	RedirErrAppend
)

type Token struct {
	Type  TokenKind
	Value string
}

func Tokenize(input string) ([]Token, error) {
	var tokens []Token
	var word strings.Builder

	flush := func() {
		if word.Len() > 0 {
			tokens = append(tokens, Token{Word, word.String()})
			word.Reset()
		}
	}

	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		switch {
		case unicode.IsSpace(r):
			flush()

		case r == '|':
			flush()
			tokens = append(tokens, Token{Pipe, "|"})

		case r == '<':
			flush()
			tokens = append(tokens, Token{RedirIn, "<"})

		case r == '>':
			isAppend := i+1 < len(runes) && runes[i+1] == '>'

			// A lone "2" glued to ">" is the stderr operator.
			if word.String() == "2" {
				word.Reset()
				if isAppend {
					tokens = append(tokens, Token{RedirErrAppend, "2>>"})
					i++
				} else {
					tokens = append(tokens, Token{RedirErr, "2>"})
				}
				continue
			}

			flush()
			if isAppend {
				tokens = append(tokens, Token{RedirAppend, ">>"})
				i++
			} else {
				tokens = append(tokens, Token{RedirOut, ">"})
			}

		default:
			word.WriteRune(r)
		}
	}
	flush()

	return tokens, nil
}
