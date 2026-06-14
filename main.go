// Command lumos switches colour themes across many programs at once.
//
// Install with:
//
//	go install github.com/CuriousFurBytes/lumos@latest
//
// See the README for theme bundle format and usage.
package main

import (
	"os"

	"github.com/CuriousFurBytes/lumos/internal/cli"
)

func main() {
	os.Exit(cli.New().Run(os.Args[1:]))
}
