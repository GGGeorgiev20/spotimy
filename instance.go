package main

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// Only one instance may run: two would share the device id, the player
// state file and the keyring entry. A second launch (e.g. a spotify: link
// from the browser) hands its arguments to the running instance and exits.

func socketPath() string { return runtimePath("spotimy.sock") }

// runtimePath is name in the user's runtime dir.
func runtimePath(name string) string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, name)
}

// forwardToRunning sends args to a running instance; false if none answers.
// A copy still running under the app's old name (personal-spotify) counts:
// two would play as the same device.
func forwardToRunning(args []string) bool {
	for _, p := range []string{socketPath(), runtimePath("personal-spotify.sock")} {
		c, err := net.Dial("unix", p)
		if err != nil {
			continue
		}
		_, err = c.Write([]byte(strings.Join(args, "\n") + "\n"))
		c.Close()
		return err == nil
	}
	return false
}

// listenForInstances accepts arguments from later launches and passes each
// one to open. The socket is only reachable by this user (0700 runtime dir,
// 0600 socket).
func listenForInstances(open func(arg string)) (stop func(), err error) {
	p := socketPath()
	_ = os.Remove(p) // a stale socket from a crash; a live instance answered Dial already
	ln, err := net.Listen("unix", p)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(p, 0o600)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				s := bufio.NewScanner(c)
				for s.Scan() {
					if arg := strings.TrimSpace(s.Text()); arg != "" {
						open(arg)
					}
				}
			}()
		}
	}()
	return func() { ln.Close(); os.Remove(p) }, nil
}

// spotifyURI turns a spotify: URI or an open.spotify.com link into a URI.
func spotifyURI(arg string) string {
	if strings.HasPrefix(arg, "spotify:") {
		return arg
	}
	for _, pre := range []string{"https://open.spotify.com/", "http://open.spotify.com/"} {
		if rest, ok := strings.CutPrefix(arg, pre); ok {
			rest, _, _ = strings.Cut(rest, "?")
			parts := strings.Split(strings.Trim(rest, "/"), "/")
			if len(parts) >= 2 && strings.HasPrefix(parts[0], "intl-") {
				parts = parts[1:]
			}
			if len(parts) >= 2 {
				return "spotify:" + parts[0] + ":" + parts[1]
			}
		}
	}
	return ""
}
