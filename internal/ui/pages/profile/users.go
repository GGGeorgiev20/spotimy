package profile

import (
	"gioui.org/layout"

	"github.com/GGGeorgiev20/personal-spotify/internal/spotify"
	"github.com/GGGeorgiev20/personal-spotify/internal/ui"
)

// users is /user/<id>/followers or /user/<id>/following: every profile as a
// round card that opens that profile.
type users struct {
	kind string // "followers" or "following"
	res  ui.Res[[]spotify.Card]
	list ui.List
	rows ui.CardRows
}

func newUsers(id, kind string) *users {
	u := &users{kind: kind}
	u.res.Load(func() ([]spotify.Card, error) { return ui.B.ProfileUsers(id, kind) })
	return u
}

func (u *users) Layout(gtx layout.Context) layout.Dimensions {
	switch {
	case !u.res.Done:
		return ui.Loading(gtx)
	case u.res.Err != nil:
		return ui.Unavailable(gtx, "This list", u.res.Err)
	}
	cards := u.res.V
	n := u.rows.Rows(gtx, len(cards))
	return u.list.Layout(gtx, n+2, func(gtx layout.Context, i int) layout.Dimensions {
		switch {
		case i == 0:
			title := "Followers"
			if u.kind == "following" {
				title = "Following"
			}
			return ui.Pad(gtx, 73, 24, 32, 24, func(gtx layout.Context) layout.Dimensions {
				return ui.Text(gtx, title, 32, ui.Bold, ui.ColText)
			})
		case i <= n:
			return u.rows.Row(gtx, cards, i-1, ui.CardOpts{})
		}
		return ui.Gap(0, 24)(gtx)
	})
}
