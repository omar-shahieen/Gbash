package parser

import (
	"reflect"
	"testing"
)

func TestSingleQuotes(t *testing.T) {
	cases := []struct {
		input string
		want  []Token
	}{
		{"echo 'hello world'", []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "hello world"}}},
		{"echo 'a | b & c'", []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "a | b & c"}}},
		{"echo ''", []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: ""}}},
		{"echo pre'hello world'post", []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "prehello worldpost"}}},
		{"echo ''tail", []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "tail"}}},
		{"echo 'one''two'", []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "onetwo"}}},
		{"echo '2'>out", []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "2"}, {Type: RedirOut, Value: ">"}, {Type: Word, Value: "out"}}},
		{"echo 2''>out", []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "2"}, {Type: RedirOut, Value: ">"}, {Type: Word, Value: "out"}}},
		{"echo 2>out", []Token{{Type: Word, Value: "echo"}, {Type: RedirErr, Value: "2>"}, {Type: Word, Value: "out"}}},
		{"echo ''|cat", []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: ""}, {Type: Pipe, Value: "|"}, {Type: Word, Value: "cat"}}},
		{"echo 'back\\slash'", []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "back\\slash"}}},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := Tokenize(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if !sameTokenText(got, tc.want) {
				t.Fatalf("tokens = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestUnclosedSingleQuote(t *testing.T) {
	for _, input := range []string{"echo 'unfinished", "echo pre'", "'"} {
		tokens, err := Tokenize(input)
		if err == nil || tokens != nil {
			t.Fatalf("Tokenize(%q) = %#v, %v", input, tokens, err)
		}
	}
}

func TestDoubleQuotes(t *testing.T) {
	cases := []struct {
		input string
		want  []Token
	}{
		{`echo "hello world"`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "hello world"}}},
		{`echo ""`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: ""}}},
		{`echo pre"hello world"post`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "prehello worldpost"}}},
		{`echo "it's fine"`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "it's fine"}}},
		{`echo 'say "hello"'`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: `say "hello"`}}},
		{`echo "a | b & c"`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "a | b & c"}}},
		{`echo "2">out`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "2"}, {Type: RedirOut, Value: ">"}, {Type: Word, Value: "out"}}},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := Tokenize(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if !sameTokenText(got, tc.want) {
				t.Fatalf("tokens = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestUnclosedDoubleQuote(t *testing.T) {
	for _, input := range []string{`echo "unfinished`, `echo pre"`, `"`} {
		tokens, err := Tokenize(input)
		if err == nil || tokens != nil {
			t.Fatalf("Tokenize(%q) = %#v, %v", input, tokens, err)
		}
		if err.Error() != `syntax error: unclosed " quote` {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestUnquotedBackslashEscapes(t *testing.T) {
	cases := []struct {
		input string
		want  []Token
	}{
		{`echo hello\ world`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "hello world"}}},
		{`echo a\|b`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "a|b"}}},
		{`echo \&`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "&"}}},
		{`echo \"hello\"`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: `"hello"`}}},
		{`echo \'`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "'"}}},
		{`echo \\`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: `\`}}},
		{`echo \>`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: ">"}}},
		{`echo \2>out`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "2"}, {Type: RedirOut, Value: ">"}, {Type: Word, Value: "out"}}},
		{`echo 'a\ b'`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: `a\ b`}}},
		{`echo "a\ b"`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: `a\ b`}}},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := Tokenize(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if !sameTokenText(got, tc.want) {
				t.Fatalf("tokens = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestTrailingBackslash(t *testing.T) {
	for _, input := range []string{`\`, `echo hello\`} {
		tokens, err := Tokenize(input)
		if err == nil || tokens != nil {
			t.Fatalf("Tokenize(%q) = %#v, %v", input, tokens, err)
		}
		if err.Error() != "syntax error: trailing backslash" {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestDoubleQuoteEscapes(t *testing.T) {
	cases := []struct{ input, value string }{
		{`echo "say \"hello\""`, `say "hello"`},
		{`echo "a\\b"`, `a\b`},
		{`echo "\$HOME"`, `$HOME`},
		{"echo \"\\`literal\\`\"", "`literal`"},
		{`echo "a\ b"`, `a\ b`},
		{`echo "a\qb"`, `a\qb`},
		{`echo "\n"`, `\n`},
		{`echo 'say \"hello\"'`, `say \"hello\"`},
		{`echo pre"a\"b"post`, `prea"bpost`},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := Tokenize(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			want := []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: tc.value}}
			if !sameTokenText(got, want) {
				t.Fatalf("tokens = %#v, want %#v", got, want)
			}
		})
	}
}

func TestEscapedQuoteDoesNotCloseWord(t *testing.T) {
	for _, input := range []string{`echo "unfinished\"`, `echo "unfinished\`} {
		tokens, err := Tokenize(input)
		if err == nil || tokens != nil {
			t.Fatalf("Tokenize(%q) = %#v, %v", input, tokens, err)
		}
	}
}

// Text tests verify syntax independently of expansion metadata.
func sameTokenText(got, want []Token) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].Type != want[i].Type || got[i].Value != want[i].Value {
			return false
		}
	}
	return true
}

func TestWordParts(t *testing.T) {
	cases := []struct {
		input string
		value string
		parts []WordPart
	}{
		{`$HOME`, `$HOME`, []WordPart{{Text: "$HOME", Expand: true}}},
		{`'$HOME'`, `$HOME`, []WordPart{{Text: "$HOME", Expand: false}}},
		{`"$HOME"`, `$HOME`, []WordPart{{Text: "$HOME", Expand: true}}},
		{`\$HOME`, `$HOME`, []WordPart{{Text: "$", Expand: false}, {Text: "HOME", Expand: true}}},
		{`pre'$HOME'"$USER"`, `pre$HOME$USER`, []WordPart{
			{Text: "pre", Expand: true},
			{Text: "$HOME", Expand: false},
			{Text: "$USER", Expand: true},
		}},
		{`''`, "", []WordPart{{Text: "", Expand: false}}},
		{`""`, "", []WordPart{{Text: "", Expand: true}}},
		{`"$HO"ME`, `$HOME`, []WordPart{{Text: "$HO", Expand: true}, {Text: "ME", Expand: true}}},
		{`"\$HOME"`, `$HOME`, []WordPart{{Text: "", Expand: true}, {Text: "$", Expand: false}, {Text: "HOME", Expand: true}}},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			tokens, err := Tokenize(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if len(tokens) != 1 || tokens[0].Type != Word || tokens[0].Value != tc.value {
				t.Fatalf("tokens = %#v, want one word %q", tokens, tc.value)
			}
			if !reflect.DeepEqual(tokens[0].Parts, tc.parts) {
				t.Fatalf("parts = %#v, want %#v", tokens[0].Parts, tc.parts)
			}
		})
	}
}

func TestSemicolonTokens(t *testing.T) {
	cases := []struct {
		input string
		want  []Token
	}{
		{"echo one;echo two", []Token{
			{Type: Word, Value: "echo"}, {Type: Word, Value: "one"},
			{Type: Semicolon, Value: ";"},
			{Type: Word, Value: "echo"}, {Type: Word, Value: "two"},
		}},
		{"echo 'a;b'", []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "a;b"}}},
		{`echo a\;b`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: "a;b"}}},
		{`echo ";"`, []Token{{Type: Word, Value: "echo"}, {Type: Word, Value: ";"}}},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := Tokenize(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if !sameTokenText(got, tc.want) {
				t.Fatalf("tokens = %#v, want %#v", got, tc.want)
			}
		})
	}
}
