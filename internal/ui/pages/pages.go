// Package pages links every page and panel package into the binary; each
// registers itself with the ui router or shell in init.
package pages

import (
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/pages/album"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/pages/artist"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/pages/collection"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/pages/episode"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/pages/genre"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/pages/home"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/pages/lyrics"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/pages/playlist"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/pages/profile"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/pages/search"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/pages/section"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/pages/show"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/panels/devices"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/panels/friends"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/panels/library"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/panels/npv"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/panels/playerbar"
	_ "github.com/GGGeorgiev20/personal-spotify/internal/ui/panels/queue"
)
