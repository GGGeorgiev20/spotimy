package playlist

import "testing"

func TestText(t *testing.T) {
	in := `Fresh <a href="spotify:artist:1">Muse</a> &amp; more`
	if got := text(in); got != "Fresh Muse & more" {
		t.Errorf("text = %q", got)
	}
}
