package spotify

// Raw pathfinder GraphQL fragments, shared by the typed methods.

type gqlImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type gqlSources struct {
	Sources []gqlImage `json:"sources"`
}

func (s *gqlSources) images() []Image {
	if s == nil {
		return nil
	}
	out := make([]Image, 0, len(s.Sources))
	for _, i := range s.Sources {
		out = append(out, Image{i.URL, i.Width, i.Height})
	}
	return out
}

type gqlImages struct {
	Items []gqlSources `json:"items"`
}

func (s gqlImages) first() []Image {
	if len(s.Items) == 0 {
		return nil
	}
	return s.Items[0].images()
}

type gqlArtist struct {
	URI     string `json:"uri"`
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
	Visuals *struct {
		AvatarImage *gqlSources `json:"avatarImage"`
	} `json:"visuals"`
}

type gqlArtists struct {
	Items []gqlArtist `json:"items"`
}

func (a gqlArtists) list() []Artist {
	out := make([]Artist, 0, len(a.Items))
	for _, x := range a.Items {
		ar := Artist{URI: x.URI, Name: x.Profile.Name}
		if x.Visuals != nil {
			ar.Images = x.Visuals.AvatarImage.images()
		}
		out = append(out, ar)
	}
	return out
}

func (a gqlArtists) names() string {
	s := ""
	for i, x := range a.Items {
		if i > 0 {
			s += ", "
		}
		s += x.Profile.Name
	}
	return s
}

type gqlDate struct {
	ISOString string `json:"isoString"`
	Year      int    `json:"year"`
}

type gqlUser struct {
	URI      string      `json:"uri"`
	Name     string      `json:"name"`
	Username string      `json:"username"`
	Avatar   *gqlSources `json:"avatar"`
}

func (u gqlUser) user() User {
	name := u.Name
	if name == "" {
		name = u.Username
	}
	return User{URI: u.URI, Name: name, Images: u.Avatar.images()}
}

// gqlTrack covers the Track fields used by playlists, albums and the library.
// Playlists call duration trackDuration; albums and the library call it duration.
type gqlTrack struct {
	Typename     string     `json:"__typename"`
	URI          string     `json:"uri"`
	Name         string     `json:"name"`
	Artists      gqlArtists `json:"artists"`
	AlbumOfTrack *struct {
		URI      string      `json:"uri"`
		Name     string      `json:"name"`
		CoverArt *gqlSources `json:"coverArt"`
	} `json:"albumOfTrack"`
	Duration      *struct{ TotalMilliseconds int } `json:"duration"`
	TrackDuration *struct{ TotalMilliseconds int } `json:"trackDuration"`
	ContentRating struct {
		Label string `json:"label"`
	} `json:"contentRating"`
	Playability struct {
		Playable bool `json:"playable"`
	} `json:"playability"`
	Playcount   string `json:"playcount"`
	TrackNumber int    `json:"trackNumber"`
	DiscNumber  int    `json:"discNumber"`

	// Episode fields (playlists can contain episodes).
	PodcastV2 *struct {
		Data struct {
			URI      string      `json:"uri"`
			Name     string      `json:"name"`
			CoverArt *gqlSources `json:"coverArt"`
		} `json:"data"`
	} `json:"podcastV2"`
	CoverArt *gqlSources `json:"coverArt"`
}

func (t gqlTrack) track() Track {
	out := Track{
		URI:         t.URI,
		Name:        t.Name,
		Artists:     t.Artists.list(),
		Explicit:    t.ContentRating.Label == "EXPLICIT",
		Playable:    t.Playability.Playable,
		Playcount:   t.Playcount,
		TrackNumber: t.TrackNumber,
		DiscNumber:  t.DiscNumber,
	}
	if t.Duration != nil {
		out.DurationMs = t.Duration.TotalMilliseconds
	} else if t.TrackDuration != nil {
		out.DurationMs = t.TrackDuration.TotalMilliseconds
	}
	if t.AlbumOfTrack != nil {
		out.Album = &AlbumRef{URI: t.AlbumOfTrack.URI, Name: t.AlbumOfTrack.Name, Images: t.AlbumOfTrack.CoverArt.images()}
	}
	if t.Typename == "Episode" {
		out.Episode = true
		if t.PodcastV2 != nil {
			p := t.PodcastV2.Data
			out.Album = &AlbumRef{URI: p.URI, Name: p.Name, Images: t.CoverArt.images()}
			out.Artists = []Artist{{URI: p.URI, Name: p.Name}}
		}
	}
	return out
}
