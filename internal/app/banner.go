package app

import (
	"fmt"
	"io"
	"os"
	"time"
)

const version = "1.0.0"

func printBanner(out io.Writer) {
	reset, bold, dim, cyan, brightCyan, white, gray, yellow :=
		"\033[0m", "\033[1m", "\033[2m", "\033[36m", "\033[96m", "\033[97m", "\033[90m", "\033[33m"
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled || os.Getenv("TERM") == "dumb" {
		reset, bold, dim, cyan, brightCyan, white, gray, yellow = "", "", "", "", "", "", "", ""
	}
	const logo = `       _               _
  __ _| |__   __ _ ___| |__
 / _` + "`" + ` | '_ \ / _` + "`" + ` / __| '_ \
| (_| | |_) | (_| \__ \ | | |
 \__, |_.__/ \__,_|___/_| |_|
 |___/
`
	fmt.Fprintf(out, "\n%s%s%s", bold+cyan, logo, reset)
	fmt.Fprintf(out, "\n %sgbash%s  %s%s%s\n", bold+white, reset, dim+gray, version, reset)
	fmt.Fprintf(out, " %sA shell written in Go. Type %shelp%s for available commands.%s\n\n", gray, cyan, gray, reset)
	fmt.Fprintf(out, " %s%-10s%s %s%s%s\n", cyan, "Author", reset, gray, "Omar Mohamed", reset)
	fmt.Fprintf(out, " %s%-10s%s %s%s%s\n\n", cyan, "Source", reset, gray, "github.com/omar-shahieen/gbash", reset)
	for i, hint := range []struct{ key, description string }{
		{"TAB", "complete"}, {"↑ ↓", "history"}, {"Ctrl+C", "cancel"}, {"Ctrl+D", "exit"}, {"Ctrl+L", "clear"},
	} {
		if i > 0 {
			fmt.Fprintf(out, "  %s│%s", gray, reset)
		}
		fmt.Fprintf(out, " %s%s%s %s%s%s", brightCyan, hint.key, reset, dim+gray, hint.description, reset)
	}
	tips := []string{
		"Use && to run the next command only after success",
		"Redirect output with > file.txt or >> to append",
		"Run jobs in background with & — check them with jobs",
		"Use $? for the last exit status and $! for the last background PID",
		"Pipe commands together: ls | grep .go | wc -l",
	}
	fmt.Fprintf(out, "\n\n %s✦ Tip:%s %s%s%s\n\n", yellow, reset, dim+gray, tips[time.Now().Unix()%int64(len(tips))], reset)
}
