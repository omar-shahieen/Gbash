package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestBanner(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var output bytes.Buffer
	printBanner(&output)
	for _, text := range []string{"gbash", version, "Omar Mohamed", "github.com/omar-shahieen/gbash", "TAB", "history", "Ctrl+D", "Tip:"} {
		if !strings.Contains(output.String(), text) {
			t.Fatalf("banner missing %q", text)
		}
	}
	if strings.Contains(output.String(), "\033[") {
		t.Fatal("NO_COLOR banner contains ANSI escapes")
	}
}
