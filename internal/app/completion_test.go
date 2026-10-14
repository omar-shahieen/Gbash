package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestShellCompletion(t *testing.T) {
	directory := t.TempDir()
	t.Chdir(directory)
	bin := filepath.Join(directory, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("GOBASH_COMPLETION_TEST", "value")
	for _, name := range []string{"plain.txt", "my file.txt", "say\"hi.txt", ".hidden"} {
		if err := os.WriteFile(name, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir("docs", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "uniquecommand"), nil, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "unique-not-executable"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		line string
		want []string
	}{
		{"ech", []string{"o "}},
		{"echo first && ech", []string{"o "}},
		{"echo first | ech", []string{"o "}},
		{"uni", []string{"quecommand "}},
		{"./bin/uni", []string{"quecommand "}},
		{"cat pl", []string{"ain.txt "}},
		{"cat < pl", []string{"ain.txt "}},
		{"echo my\\ f", []string{"ile.txt "}},
		{`echo "my f`, []string{`ile.txt" `}},
		{`echo 'my f`, []string{`ile.txt' `}},
		{"echo say", []string{`\"hi.txt `}},
		{"cd d", []string{"ocs/"}},
		{"cd pl", nil},
		{"echo .h", []string{"idden "}},
		{"echo $GOBASH_COMPLETION_T", []string{"EST"}},
		{`echo "${GOBASH_COMPLETION_T`, []string{"EST}"}},
		{"echo '$GOBASH_COMPLETION_T", nil},
		{"echo \\$GOBASH_COMPLETION_T", nil},
	} {
		t.Run(tc.line, func(t *testing.T) {
			candidates, _ := (shellCompleter{}).Do([]rune(tc.line), len([]rune(tc.line)))
			var got []string
			for _, candidate := range candidates {
				got = append(got, string(candidate))
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("suffixes = %q, want %q", got, tc.want)
			}
		})
	}
	line := []rune("echo pl; echo later")
	candidates, _ := (shellCompleter{}).Do(line, len([]rune("echo pl")))
	if len(candidates) != 1 || string(candidates[0]) != "ain.txt" {
		t.Fatalf("completion in middle = %q", candidates)
	}
}
