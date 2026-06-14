package registry

import "testing"

func TestAllLoadsEmbeddedPorts(t *testing.T) {
	all := All()
	if len(all) < 20 {
		t.Fatalf("expected a base of at least 20 ports, got %d", len(all))
	}
	// A few well-known ports we seeded from the upstream theme projects.
	for _, key := range []string{"alacritty", "kitty", "bat", "btop", "tmux", "helix"} {
		if _, ok := all[key]; !ok {
			t.Errorf("missing expected port %q", key)
		}
	}
}

func TestLookup(t *testing.T) {
	p, ok := Lookup("alacritty")
	if !ok {
		t.Fatal("alacritty not found")
	}
	if p.Target == "" {
		t.Error("alacritty port has no default target")
	}
	if len(p.Families) == 0 {
		t.Error("alacritty port lists no families")
	}
	if _, ok := Lookup("does-not-exist"); ok {
		t.Error("unexpected hit for unknown port")
	}
}

func TestEveryPortHasTarget(t *testing.T) {
	for key, p := range All() {
		if p.Target == "" {
			t.Errorf("port %q has empty target", key)
		}
		if p.Name == "" {
			t.Errorf("port %q has empty name", key)
		}
	}
}
