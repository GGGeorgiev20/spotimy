package ui

// TasteExcluded holds which playlists are left out of the user's taste
// profile, by URI; missing = unknown.
var TasteExcluded = map[string]bool{}

// LoadTasteExcluded looks up unknown playlists.
func LoadTasteExcluded(uris ...string) {
	var missing []string
	for _, u := range uris {
		if _, ok := TasteExcluded[u]; !ok && u != "" {
			missing = append(missing, u)
			TasteExcluded[u] = false // until the answer arrives, so a menu doesn't ask twice
		}
	}
	if len(missing) == 0 {
		return
	}
	Async(func() ([]bool, error) { return B.TasteExcluded(missing) }, func(res []bool, err error) {
		if err != nil || len(res) != len(missing) {
			for _, u := range missing {
				delete(TasteExcluded, u)
			}
			return
		}
		for i, u := range missing {
			TasteExcluded[u] = res[i]
		}
	})
}

// ToggleTasteExcluded excludes a playlist from the taste profile, or includes it again.
func ToggleTasteExcluded(uri string) {
	next := !TasteExcluded[uri]
	TasteExcluded[uri] = next
	Async(func() (struct{}, error) { return struct{}{}, B.SetTasteExcluded(uri, next) }, func(_ struct{}, err error) {
		if err != nil {
			TasteExcluded[uri] = !next
			Toast("Couldn't update your taste profile: " + err.Error())
			return
		}
		if next {
			Toast("Listening to this playlist will have less impact on your taste profile and recommendations.")
		} else {
			Toast("Listening to this playlist will impact your taste profile and recommendations.")
		}
	})
}
