package spotify

import "testing"

func TestBiographyKeepsParagraphs(t *testing.T) {
	got := stripHTML(paragraphRE.ReplaceAllString("One <a href=\"x\">link</a>.<br><br>Two.</p><p>Three", "\n"))
	if want := "One link.\n\nTwo.\nThree"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
