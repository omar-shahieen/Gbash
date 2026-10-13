package parser

import (
	"strings"
	"unicode"
)

type TokenKind int

const (
	Word TokenKind = iota
	Pipe
	Background
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

		case r == '&':
			flush()
			tokens = append(tokens, Token{Background, "&"})
		case r == '|':
			flush()
			tokens = append(tokens, Token{Pipe, "|"})

		case r == '<':
			flush()
			tokens = append(tokens, Token{RedirIn, "<"})

		case r == '>':
			kind, value := RedirOut, ">"
			stderr := word.String() == "2"
			if stderr {
				word.Reset()
				kind, value = RedirErr, "2>"
			} else {
				flush()
			}
			if i+1 < len(runes) && runes[i+1] == '>' {
				i++
				value += ">"
				kind = RedirAppend
				if stderr {
					kind = RedirErrAppend
				}
			}
			tokens = append(tokens, Token{kind, value})

		default:
			word.WriteRune(r)
		}
	}
	flush()

	return tokens, nil
}
