package parser

import "strings"

func Tokenize(input string) []string {
	return strings.Fields(input)
}
