package ui

import (
	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
)

// CardRows lays a long list of cards out as a grid with one List item per
// row, so only the visible rows are drawn (artist discography, followers).
type CardRows struct {
	cards []Card
	cols  int
}

// Rows returns how many List items n cards take at the current width.
func (r *CardRows) Rows(gtx layout.Context, n int) int {
	r.cols = max(1, (gtx.Constraints.Max.X-Dp(gtx, 24))/Dp(gtx, 176))
	return (n + r.cols - 1) / r.cols
}

// Row draws row i; call it from the List item func (after Rows).
func (r *CardRows) Row(gtx layout.Context, items []spotify.Card, i int, o CardOpts) layout.Dimensions {
	if len(r.cards) < len(items) {
		r.cards = append(r.cards, make([]Card, len(items)-len(r.cards))...)
	}
	first := i * r.cols
	n := min(r.cols, len(items)-first)
	return Pad(gtx, 0, 12, 0, 12, func(gtx layout.Context) layout.Dimensions {
		return CardGrid(gtx, n, 176, 1, func(gtx layout.Context, k int) layout.Dimensions {
			return r.cards[first+k].Layout(gtx, items[first+k], o)
		})
	})
}
