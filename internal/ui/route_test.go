package ui

import "testing"

func TestRouteQuery(t *testing.T) {
	for path, want := range map[string]string{
		"/search":                 "",
		"/search/daft%20punk":     "daft punk",
		"/search/daft%20punk/all": "daft punk",
		"/album/abc":              "",
		"/":                       "",
	} {
		if got := routeQuery(path); got != want {
			t.Errorf("routeQuery(%q) = %q, want %q", path, got, want)
		}
	}
}
