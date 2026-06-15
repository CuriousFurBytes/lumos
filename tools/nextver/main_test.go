package main

import (
	"testing"

	"github.com/CuriousFurBytes/lumos/internal/version"
)

func TestResolveLevel(t *testing.T) {
	cases := []struct {
		bump, message string
		want          version.Level
	}{
		{"auto", "fix things", version.Prerelease},
		{"auto", "feat [minor]", version.Minor},
		{"", "[major]", version.Major},
		{"stable", "ignored", version.Stable},
		{"patch", "", version.Patch},
	}
	for _, c := range cases {
		got, err := resolveLevel(c.bump, c.message)
		if err != nil {
			t.Errorf("resolveLevel(%q,%q): %v", c.bump, c.message, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolveLevel(%q,%q) = %v, want %v", c.bump, c.message, got, c.want)
		}
	}
	if _, err := resolveLevel("nonsense", ""); err == nil {
		t.Error("expected error for unknown bump")
	}
}
