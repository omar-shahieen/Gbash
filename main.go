package main

import (
	"os"

	"github.com/omar-shahieen/gbash/internal/app"
)

func main() {
	os.Exit(app.Main(os.Args[1:]))
}
