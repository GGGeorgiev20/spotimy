package search

import "testing"

func TestTabPath(t *testing.T) {
	p := &page{query: "daft punk/live"}
	for tab, want := range map[string]string{
		"all":    "/search/daft%20punk%2Flive",
		"albums": "/search/daft%20punk%2Flive/albums",
	} {
		if got := p.tabPath(tab); got != want {
			t.Errorf("tabPath(%q) = %q, want %q", tab, got, want)
		}
	}
}

func TestTypeWord(t *testing.T) {
	if typeWord("artist") != "Artist" || typeWord("") != "" {
		t.Error("typeWord")
	}
}
