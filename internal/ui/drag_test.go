package ui

import "testing"

func TestReorderDrop(t *testing.T) {
	cases := []struct {
		from, to int
		after    bool
		noop     bool
	}{
		{2, 2, false, true},
		{2, 2, true, true},
		{2, 1, true, true},  // just below the row above = same place
		{2, 3, false, true}, // just above the row below
		{2, 1, false, false},
		{2, 3, true, false},
	}
	for _, c := range cases {
		r := Reorder{From: c.from, To: c.to, After: c.after, Active: true}
		if r.Noop() != c.noop {
			t.Errorf("%+v: Noop = %v, want %v", c, r.Noop(), c.noop)
		}
	}
	r := Reorder{From: 5, To: 1, After: true, Active: true}
	if r.DropLine(1) != 1 || r.DropLine(2) != 0 {
		t.Errorf("DropLine = %d, %d", r.DropLine(1), r.DropLine(2))
	}
}

func TestFmtMins(t *testing.T) {
	for ms, want := range map[int]string{20000: "1 min", 45 * 60000: "45 min", 2*3600000 + 24*60000: "2 hr 24 min", 3600000: "1 hr"} {
		if got := FmtMins(ms); got != want {
			t.Errorf("FmtMins(%d) = %q, want %q", ms, got, want)
		}
	}
}
