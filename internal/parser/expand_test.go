package parser

import "testing"

func TestExpandText(t *testing.T) {
	values := map[string]string{
		"HOME": "/home/omar", "USER": "omar", "UNSET": "defined",
		"_VAR2": "ok", "NESTED": "$HOME",
	}
	cases := []struct{ input, want string }{
		{"hello", "hello"}, {"", ""},
		{"$HOME", "/home/omar"}, {"${HOME}", "/home/omar"},
		{"pre$HOME", "pre/home/omar"}, {"$HOME/bin", "/home/omar/bin"},
		{"$HOME-$USER", "/home/omar-omar"}, {"${HOME}${USER}", "/home/omaromar"},
		{"$?", "7"}, {"status=$?", "status=7"},
		{"$MISSING", ""}, {"$UNSET", "defined"},
		{"$", "$"}, {"$1", "$1"}, {"$-", "$-"},
		{"$_VAR2", "ok"}, {"$NESTED", "$HOME"}, {"مرحبا $USER", "مرحبا omar"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := expandText(tc.input, func(name string) string { return values[name] }, 7)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestExpandTextInvalidBraces(t *testing.T) {
	for _, input := range []string{"${", "${HOME", "prefix${HOME", "${}", "${1}", "${HOME-name}"} {
		t.Run(input, func(t *testing.T) {
			_, err := expandText(input, func(string) string { return "value" }, 0)
			if err == nil {
				t.Fatal("expected a syntax error")
			}
		})
	}
}
