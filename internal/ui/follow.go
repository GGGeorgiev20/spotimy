package ui

// ToggleFollow follows or unfollows a user profile. Like ToggleSaved it flips
// Saved[uri] right away and puts it back if the call fails.
func ToggleFollow(uri string) {
	next := !Saved[uri]
	Saved[uri] = next
	Async(func() (struct{}, error) { return struct{}{}, B.SetFollowing(uri, next) }, func(_ struct{}, err error) {
		if err != nil {
			Saved[uri] = !next
			Toast("Couldn't update follow: " + err.Error())
		}
	})
}
