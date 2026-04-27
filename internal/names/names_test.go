package names

import "testing"

func TestPackNonEmpty(t *testing.T) {
	if len(Pack) == 0 {
		t.Fatal("ship-name Pack is empty")
	}
}

func TestPackUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, n := range Pack {
		if seen[n] {
			t.Errorf("duplicate ship-name %q", n)
		}
		seen[n] = true
	}
}

func TestDefaultIsInPack(t *testing.T) {
	d := Default()
	for _, n := range Pack {
		if n == d {
			return
		}
	}
	t.Errorf("Default() %q not present in Pack", d)
}

func TestDefaultIsStable(t *testing.T) {
	a, b := Default(), Default()
	if a != b {
		t.Errorf("Default() not stable: %q vs %q", a, b)
	}
}
