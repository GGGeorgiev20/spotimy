package artist

import "testing"

func TestCompact(t *testing.T) {
	for n, want := range map[int]string{
		0: "0", 999: "999", 5432: "5,432", 12345: "12.3K", 123456: "123K",
		1_000_000: "1M", 46_442_786: "46.4M", 105_983_595: "106M", 2_500_000_000: "2.5B",
	} {
		if got := compact(n); got != want {
			t.Errorf("compact(%d) = %q, want %q", n, got, want)
		}
	}
}
