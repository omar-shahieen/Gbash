package parser

import (
	"fmt"
	"strings"

	"github.com/omar-shahieen/Gbash/internal/shell"
)

func Tokenize(input string) []string {
	return strings.Fields(input)
}

func ParseRedirections(args []string) ([]string, []shell.Redirection, error) {

	cleanArgs := make([]string, 0, len(args))
	redirections := make([]shell.Redirection, 0, len(args))

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case ">", ">>", "<", "2>", "2>>":
			operator := args[i]

			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf(
					"syntax error: missing filename after %s",
					operator,
				)
			}

			filename := args[i+1]
			if filename == "" {
				return nil, nil, fmt.Errorf(
					"syntax error: empty filename after %s",
					operator,
				)
			}

			redirections = append(redirections, shell.Redirection{
				Operator: operator,
				Filename: filename,
			})

			i++ // Skip the filename.
		default:
			cleanArgs = append(cleanArgs, args[i])
		}
	}
	return cleanArgs, redirections, nil

}
