package app

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHistoryPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	if history, err := loadHistory(path); err != nil || len(history) != 0 {
		t.Fatalf("new history = %v, %v", history, err)
	}
	for _, line := range []string{"echo first", "echo first", "", "  echo second  "} {
		if err := appendHistory(path, line); err != nil {
			t.Fatal(err)
		}
	}
	history, err := loadHistory(path)
	if err != nil || !reflect.DeepEqual(history, []string{"echo first", "echo second"}) {
		t.Fatalf("history = %v, %v", history, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("history permissions = %o", info.Mode().Perm())
	}
	if err := appendHistory("", "disabled"); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	var contents strings.Builder
	for i := 0; i < historyLimit+10; i++ {
		fmt.Fprintf(&contents, "echo %d\n", i)
	}
	if err := os.WriteFile(path, []byte(contents.String()), 0600); err != nil {
		t.Fatal(err)
	}
	history, err := loadHistory(path)
	if err != nil || len(history) != historyLimit || history[0] != "echo 10" {
		t.Fatalf("history length = %d, error = %v", len(history), err)
	}
}

func TestHistoryPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HISTFILE", "")
	if path, err := historyPath(); err != nil || path != "" {
		t.Fatalf("disabled path = %q, %v", path, err)
	}
	if err := os.Unsetenv("HISTFILE"); err != nil {
		t.Fatal(err)
	}
	if path, err := historyPath(); err != nil || path != filepath.Join(home, ".gbash_history") {
		t.Fatalf("default path = %q, %v", path, err)
	}
	t.Chdir(home)
	t.Setenv("HISTFILE", "custom_history")
	if path, err := historyPath(); err != nil || path != filepath.Join(home, "custom_history") {
		t.Fatalf("custom path = %q, %v", path, err)
	}
}
