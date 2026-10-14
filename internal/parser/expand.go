package parser

import (
	"fmt"
	"strconv"
	"strings"
)

func ExpandTokens(
	tokens []Token,
	lookup func(string) string,
	lastStatus int,
	lastBackgroundPID int,
) ([]Token, error) {
	var expanded []Token
	for _, token := range tokens {
		if token.Type != Word {
			expanded = append(expanded, token)
			continue
		}
		value, err := expandWord(token, lookup, lastStatus, lastBackgroundPID)
		if err != nil {
			return nil, err
		}
		token.Value = value
		token.Parts = nil
		expanded = append(expanded, token)
	}
	return expanded, nil
}
func expandWord(
	token Token,
	lookup func(string) string,
	lastStatus int,
	lastBackgroundPID int,
) (string, error) {
	if len(token.Parts) == 0 {
		return token.Value, nil
	}
	var result strings.Builder
	for _, part := range token.Parts {
		if part.Expand {
			expanded, err := expandText(part.Text, lookup, lastStatus, lastBackgroundPID)
			if err != nil {
				return "", err
			}
			result.WriteString(expanded)
		} else {
			result.WriteString(part.Text)
		}
	}
	return result.String(), nil
}
func expandText(
	text string,
	lookup func(string) string,
	lastStatus int,
	lastBackgroundPID int,
) (string, error) {
	var result strings.Builder
	for i := 0; i < len(text); {
		if text[i] != '$' || i+1 == len(text) {
			result.WriteByte(text[i])
			i++
			continue
		}
		switch text[i+1] {
		case '?':
			result.WriteString(strconv.Itoa(lastStatus))
			i += 2
		case '!':
			if lastBackgroundPID > 0 {
				result.WriteString(strconv.Itoa(lastBackgroundPID))
			}
			i += 2
		case '{':
			start := i + 2
			closing := strings.IndexByte(text[start:], '}')
			if closing < 0 {
				return "", fmt.Errorf("syntax error: missing closing brace in variable expansion")
			}
			end := start + closing
			name := text[start:end]
			if !validVariableName(name) {
				return "", fmt.Errorf("syntax error: invalid variable name %q", name)
			}
			result.WriteString(lookup(name))
			i = end + 1
		default:
			if !variableNameStart(text[i+1]) {
				result.WriteByte('$')
				i++
				continue
			}
			start := i + 1
			end := start + 1
			for end < len(text) && variableNameContinue(text[end]) {
				end++
			}
			result.WriteString(lookup(text[start:end]))
			i = end
		}
	}
	return result.String(), nil
}

func variableNameStart(char byte) bool {
	return char == '_' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
}

func variableNameContinue(char byte) bool {
	return variableNameStart(char) || char >= '0' && char <= '9'
}

func validVariableName(name string) bool {
	if len(name) == 0 || !variableNameStart(name[0]) {
		return false
	}
	for i := 1; i < len(name); i++ {
		if !variableNameContinue(name[i]) {
			return false
		}
	}
	return true
}
