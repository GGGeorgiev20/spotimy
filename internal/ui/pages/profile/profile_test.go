package profile

import "testing"

func TestPlural(t *testing.T) {
	for _, c := range []struct {
		n    int
		want string
	}{{0, "0 Followers"}, {1, "1 Follower"}, {4, "4 Followers"}} {
		if got := plural(c.n, "Follower"); got != c.want {
			t.Errorf("plural(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
