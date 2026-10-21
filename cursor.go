package main

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// setCursorTheme fills in XCURSOR_THEME and XCURSOR_SIZE from the desktop's
// settings when the session doesn't export them (Plasma and GNOME don't).
// Gio loads cursors through libwayland-cursor, whose built-in fallback theme
// has no hand cursor.
func setCursorTheme() {
	theme, size := os.Getenv("XCURSOR_THEME"), os.Getenv("XCURSOR_SIZE")
	if theme != "" && size != "" {
		return
	}
	var t, s string
	if dir, err := os.UserConfigDir(); err == nil {
		if f, err := os.Open(filepath.Join(dir, "kcminputrc")); err == nil {
			t, s = kdeCursor(f)
			f.Close()
		}
	}
	gsetting := func(k string) string {
		out, _ := exec.Command("gsettings", "get", "org.gnome.desktop.interface", k).Output()
		return strings.Trim(strings.TrimSpace(string(out)), "'")
	}
	if t == "" && os.Getenv("XDG_CURRENT_DESKTOP") != "KDE" {
		t, s = gsetting("cursor-theme"), gsetting("cursor-size")
	}
	if t == "" {
		t = "breeze_cursors" // Plasma's default
		if os.Getenv("XDG_CURRENT_DESKTOP") != "KDE" {
			t = "Adwaita"
		}
	}
	if s == "" {
		s = "24"
	}
	if theme == "" {
		os.Setenv("XCURSOR_THEME", t)
	}
	if size == "" {
		os.Setenv("XCURSOR_SIZE", s)
	}
}

// kdeCursor reads cursorTheme and cursorSize from kcminputrc's [Mouse] group.
func kdeCursor(f io.Reader) (theme, size string) {
	group := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(l, "[") {
			group = l
			continue
		}
		k, v, ok := strings.Cut(l, "=")
		if !ok || group != "[Mouse]" {
			continue
		}
		switch strings.TrimSpace(k) {
		case "cursorTheme":
			theme = strings.TrimSpace(v)
		case "cursorSize":
			size = strings.TrimSpace(v)
		}
	}
	return
}
