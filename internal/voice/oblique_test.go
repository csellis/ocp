package voice

import "testing"

func TestObliquePackNonEmpty(t *testing.T) {
	if len(ObliquePack) == 0 {
		t.Fatal("ObliquePack is empty")
	}
}

func TestPickCardFromPack(t *testing.T) {
	in := map[string]bool{}
	for _, c := range ObliquePack {
		in[c] = true
	}
	for i := 0; i < 50; i++ {
		got := PickCard()
		if !in[got] {
			t.Fatalf("PickCard returned %q, not in ObliquePack", got)
		}
	}
}
