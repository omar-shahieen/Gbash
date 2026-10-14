package parser

import "testing"

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
