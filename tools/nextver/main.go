// Command nextver prints the next release tag given the latest tag and a
// bump level. It is used by the release workflow to auto-generate tags.
//
// Usage:
//
//	nextver -latest v0.0.1-alpha.1 -bump prerelease
//	nextver -latest v0.0.1-alpha.1 -message "feat: thing [minor]"
//
// With -bump "auto" (the default) the level is taken from -message.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/CuriousFurBytes/lumos/internal/version"
)

func main() {
	latest := flag.String("latest", "", "the latest existing tag (empty if none)")
	bump := flag.String("bump", "auto", "bump level: auto|prerelease|patch|minor|major|stable")
	message := flag.String("message", "", "commit message, used when -bump=auto")
	flag.Parse()

	level, err := resolveLevel(*bump, *message)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nextver:", err)
		os.Exit(1)
	}
	next, err := version.Next(*latest, level)
	if err != nil {
		fmt.Fprintln(os.Stderr, "nextver:", err)
		os.Exit(1)
	}
	fmt.Println(next)
}

func resolveLevel(bump, message string) (version.Level, error) {
	if bump == "" || bump == "auto" {
		return version.BumpFromMessage(message), nil
	}
	return version.ParseLevel(bump)
}
