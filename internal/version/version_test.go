package version

import "testing"

func TestNextFromEmpty(t *testing.T) {
	cases := map[Level]string{
		Prerelease: "v0.0.1-alpha.1",
		Patch:      "v0.0.1-alpha.1",
		Minor:      "v0.1.0-alpha.1",
		Major:      "v1.0.0-alpha.1",
		Stable:     "v0.0.1",
	}
	for level, want := range cases {
		got, err := Next("", level)
		if err != nil {
			t.Errorf("Next(\"\", %v): %v", level, err)
			continue
		}
		if got != want {
			t.Errorf("Next(\"\", %v) = %q, want %q", level, got, want)
		}
	}
}

func TestNextPrerelease(t *testing.T) {
	cases := map[string]string{
		"v0.0.1-alpha.1": "v0.0.1-alpha.2", // bump the alpha counter
		"v0.0.1-alpha.9": "v0.0.1-alpha.10",
		"v0.1.0":         "v0.1.1-alpha.1", // from stable -> next patch alpha
		"v1.2.3-beta.4":  "v1.2.3-beta.5",  // preserve a non-alpha identifier
	}
	for in, want := range cases {
		got, err := Next(in, Prerelease)
		if err != nil {
			t.Errorf("Next(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("Next(%q, Prerelease) = %q, want %q", in, got, want)
		}
	}
}

func TestNextBumps(t *testing.T) {
	latest := "v0.3.4-alpha.5"
	cases := map[Level]string{
		Patch:  "v0.3.5-alpha.1",
		Minor:  "v0.4.0-alpha.1",
		Major:  "v1.0.0-alpha.1",
		Stable: "v0.3.4", // drop the prerelease, promoting the current numbers
	}
	for level, want := range cases {
		got, err := Next(latest, level)
		if err != nil {
			t.Errorf("Next(%q, %v): %v", latest, level, err)
			continue
		}
		if got != want {
			t.Errorf("Next(%q, %v) = %q, want %q", latest, level, got, want)
		}
	}
}

func TestStableFromStableBumpsPatch(t *testing.T) {
	got, err := Next("v1.0.0", Stable)
	if err != nil {
		t.Fatal(err)
	}
	if got != "v1.0.1" {
		t.Errorf("got %q, want v1.0.1", got)
	}
}

func TestNextInvalid(t *testing.T) {
	for _, in := range []string{"1.0", "vabc", "v1.2.x"} {
		if _, err := Next(in, Prerelease); err == nil {
			t.Errorf("Next(%q) expected error", in)
		}
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]Level{
		"prerelease": Prerelease,
		"patch":      Patch,
		"minor":      Minor,
		"major":      Major,
		"stable":     Stable,
	}
	for s, want := range cases {
		got, err := ParseLevel(s)
		if err != nil || got != want {
			t.Errorf("ParseLevel(%q) = %v,%v want %v", s, got, err, want)
		}
	}
	if _, err := ParseLevel("bogus"); err == nil {
		t.Error("expected error for bogus level")
	}
}

func TestBumpFromMessage(t *testing.T) {
	cases := map[string]Level{
		"fix typo":                         Prerelease, // default
		"feat: thing [minor]":              Minor,
		"breaking change [major] you know": Major,
		"release [stable]":                 Stable,
		"chore [patch]":                    Patch,
	}
	for msg, want := range cases {
		if got := BumpFromMessage(msg); got != want {
			t.Errorf("BumpFromMessage(%q) = %v, want %v", msg, got, want)
		}
	}
}
