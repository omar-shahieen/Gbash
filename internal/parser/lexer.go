package parser

import (
	"fmt"
	"strings"
	"unicode"
)

type TokenKind int

const (
	Word TokenKind = iota
	Pipe
	AndIf
	OrIf
	Semicolon
	Background
	RedirOut
	RedirAppend
	RedirIn
	RedirErr
	RedirErrAppend
)

type WordPart struct {
	Text   string
	Expand bool
}

type Token struct {
	Type  TokenKind
	Value string
	Parts []WordPart
}

// Tokenize separates words and operators while preserving expansion boundaries.
func Tokenize(input string) ([]Token, error) {
	runes := []rune(input)
	var tokens []Token
	var word wordBuilder
	flush := func() {
		if token, ok := word.take(); ok {
			tokens = append(tokens, token)
		}
	}

	for i := 0; i < len(runes); {
		char := runes[i]
		switch {
		case unicode.IsSpace(char):
			flush()
			i++
		case char == '\'' || char == '"':
			next, err := readQuotedWord(runes, i, &word)
			if err != nil {
				return nil, err
			}
			i = next
		case char == '\\':
			if i+1 == len(runes) {
				return nil, fmt.Errorf("syntax error: trailing backslash")
			}
			word.appendEscaped(runes[i+1])
			i += 2
		case char == '&' || char == '|' || char == '<' || char == ';':
			kind := map[rune]TokenKind{
				'&': Background, '|': Pipe, '<': RedirIn, ';': Semicolon,
			}[char]
			value := string(char)

			if i+1 < len(runes) && runes[i+1] == char {
				switch char {
				case '&':
					kind, value = AndIf, "&&"
					i++
				case '|':
					kind, value = OrIf, "||"
					i++
				}
			}
			flush()
			tokens = append(tokens, Token{Type: kind, Value: value})
			i++
		case char == '>':
			stderr := word.text.String() == "2" && !word.protected
			if stderr {
				word.reset() // The descriptor belongs to the operator, not a word.
			} else {
				flush()
			}
			token, next := readOutputOperator(runes, i, stderr)
			tokens = append(tokens, token)
			i = next
		default:
			word.append(string(char), true)
			i++
		}
	}
	flush()
	return tokens, nil
}

// wordBuilder keeps combined text and per-segment expansion metadata in sync.
type wordBuilder struct {
	text      strings.Builder
	parts     []WordPart
	started   bool
	protected bool
	partOpen  bool
}

func (word *wordBuilder) append(text string, expand bool) {
	word.text.WriteString(text)
	last := len(word.parts) - 1
	if word.partOpen && last >= 0 && word.parts[last].Expand == expand {
		word.parts[last].Text += text
	} else {
		word.parts = append(word.parts, WordPart{Text: text, Expand: expand})
	}
	word.started = true
	word.partOpen = true
}

func (word *wordBuilder) appendEscaped(char rune) {
	word.protected = true
	word.partOpen = false
	word.append(string(char), false)
	word.partOpen = false
}

func (word *wordBuilder) take() (Token, bool) {
	if !word.started {
		return Token{}, false
	}
	token := Token{Type: Word, Value: word.text.String(), Parts: word.parts}
	word.reset()
	return token, true
}

func (word *wordBuilder) reset() {
	*word = wordBuilder{}
}

// readQuotedWord returns the index after the closing quote. Quotes never flush a word.
func readQuotedWord(runes []rune, start int, word *wordBuilder) (int, error) {
	quote := runes[start]
	expand := quote == '"'
	word.protected = true
	word.partOpen = false
	word.append("", expand) // Empty quotes still form an argument.

	for i := start + 1; i < len(runes); i++ {
		char := runes[i]
		if char == quote {
			word.partOpen = false
			return i + 1, nil
		}
		if quote == '"' && char == '\\' && i+1 < len(runes) &&
			strings.ContainsRune("\"\\$`", runes[i+1]) {
			i++
			word.appendEscaped(runes[i])
		} else {
			word.append(string(char), expand)
		}
	}
	return 0, fmt.Errorf("syntax error: unclosed %c quote", quote)
}

func readOutputOperator(runes []rune, start int, stderr bool) (Token, int) {
	kind, value := RedirOut, ">"
	if stderr {
		kind, value = RedirErr, "2>"
	}
	next := start + 1
	if next < len(runes) && runes[next] == '>' {
		next++
		value += ">"
		kind = RedirAppend
		if stderr {
			kind = RedirErrAppend
		}
	}
	return Token{Type: kind, Value: value}, next
}
