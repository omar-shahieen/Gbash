package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/omar-shahieen/gbash/internal/builtins"
)

type shellCompleter struct{}

// Do returns suffixes to insert at the cursor, preserving the rest of the line.
func (shellCompleter) Do(line []rune, pos int) ([][]rune, int) {
	if pos < 0 || pos > len(line) {
		return nil, 0
	}
	context := completionContext(line[:pos])
	prefix := context.word
	if context.variable && context.quote != '\'' {
		return completeVariable(prefix), context.length
	}
	choices := map[string]bool{} // value is whether the candidate is a directory.
	command := context.command && !context.redirection
	if command && !strings.ContainsRune(prefix, '/') {
		for _, name := range builtins.Names() {
			if strings.HasPrefix(name, prefix) {
				choices[name] = false
			}
		}
		for _, directory := range filepath.SplitList(os.Getenv("PATH")) {
			if directory == "" {
				directory = "."
			}
			entries, _ := os.ReadDir(directory)
			for _, entry := range entries {
				if !strings.HasPrefix(entry.Name(), prefix) {
					continue
				}
				info, err := os.Stat(filepath.Join(directory, entry.Name()))
				if err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
					choices[entry.Name()] = false
				}
			}
		}
	}
	directory, base := filepath.Split(prefix)
	searchDirectory := directory
	if searchDirectory == "" {
		searchDirectory = "."
	}
	entries, _ := os.ReadDir(searchDirectory)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, base) || (strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".")) {
			continue
		}
		info, err := os.Stat(filepath.Join(searchDirectory, name))
		if err != nil {
			continue
		}
		if context.commandName == "cd" && !context.redirection && !info.IsDir() {
			continue
		}
		if command && !info.IsDir() && (directory == "" || info.Mode()&0111 == 0) {
			continue
		}
		value := directory + name
		if info.IsDir() {
			value += "/"
		}
		choices[value] = info.IsDir()
	}
	names := make([]string, 0, len(choices))
	for name := range choices {
		names = append(names, name)
	}
	slices.Sort(names)
	var candidates [][]rune
	for _, name := range names {
		suffix := escapeCompletion(strings.TrimPrefix(name, prefix), context.quote)
		if !choices[name] && pos == len(line) {
			if context.quote != 0 {
				suffix += string(context.quote)
			}
			suffix += " "
		}
		candidates = append(candidates, []rune(suffix))
	}
	return candidates, context.length
}

type wordContext struct {
	word        string
	length      int
	quote       rune
	command     bool
	commandName string
	redirection bool
	variable    bool
}

// This scanner accepts unfinished quotes and escapes, unlike the execution lexer.
func completionContext(line []rune) wordContext {
	context := wordContext{command: true}
	var word strings.Builder
	started, escaped := false, false
	start := len(line)
	finish := func() {
		if !started {
			return
		}
		if context.redirection {
			context.redirection = false
		} else if context.command {
			context.commandName = word.String()
			context.command = false
		}
		word.Reset()
		started = false
		context.variable = false
	}
	for i, char := range line {
		if escaped {
			word.WriteRune(char)
			escaped = false
			continue
		}
		if context.quote != 0 {
			if char == context.quote {
				context.quote = 0
				continue
			}
			if char == '\\' && context.quote == '"' && i+1 < len(line) && strings.ContainsRune("\"\\$`", line[i+1]) {
				escaped = true
				continue
			}
		} else {
			if unicode.IsSpace(char) {
				finish()
				continue
			}
			if strings.ContainsRune("|&;<>", char) {
				finish()
				if char == '<' || char == '>' {
					context.redirection = true
				} else {
					context.command, context.redirection, context.commandName = true, false, ""
				}
				continue
			}
			if !started {
				started, start = true, i
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == '\'' || char == '"' {
				context.quote = char
				continue
			}
		}
		if char == '$' && word.Len() == 0 && context.quote != '\'' {
			context.variable = true
		}
		word.WriteRune(char)
	}
	context.word = word.String()
	if started {
		context.length = len(line) - start
	}
	return context
}

func completeVariable(prefix string) [][]rune {
	name := strings.TrimPrefix(prefix, "$")
	braced := strings.HasPrefix(name, "{")
	if braced {
		name = strings.TrimPrefix(name, "{")
	}
	var names []string
	for _, pair := range os.Environ() {
		key, _, _ := strings.Cut(pair, "=")
		if strings.HasPrefix(key, name) {
			names = append(names, key)
		}
	}
	slices.Sort(names)
	var candidates [][]rune
	for _, key := range names {
		suffix := strings.TrimPrefix(key, name)
		if braced {
			suffix += "}"
		}
		candidates = append(candidates, []rune(suffix))
	}
	return candidates
}

func escapeCompletion(text string, quote rune) string {
	var escaped strings.Builder
	for _, char := range text {
		if quote == '\'' && char == '\'' {
			escaped.WriteString("'\\''")
			continue
		}
		if (quote == 0 && (unicode.IsSpace(char) || strings.ContainsRune("\\'\"$`|&;<>()[{}]*?!", char))) ||
			(quote == '"' && strings.ContainsRune("\\\"$`", char)) {
			escaped.WriteRune('\\')
		}
		escaped.WriteRune(char)
	}
	return escaped.String()
}
