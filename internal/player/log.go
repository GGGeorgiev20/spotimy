package player

import (
	"fmt"
	"log/slog"
	"strings"

	librespot "github.com/devgianlu/go-librespot"
)

// logger adapts slog to go-librespot's Logger interface.
type logger struct {
	l *slog.Logger
	// onLoginURL is called with the OAuth URL that go-librespot only logs.
	onLoginURL func(string)
}

// ponytail: go-librespot exposes the interactive login URL only through its log
// line; if that wording changes, login falls back to the URL in the log output.
const loginURLPrefix = "to complete authentication visit the following link: "

func (g logger) Tracef(f string, a ...any) {}
func (g logger) Debugf(f string, a ...any) { g.l.Debug(fmt.Sprintf(f, a...)) }
func (g logger) Infof(f string, a ...any) {
	msg := fmt.Sprintf(f, a...)
	if url, ok := strings.CutPrefix(msg, loginURLPrefix); ok && g.onLoginURL != nil {
		g.onLoginURL(url)
	}
	g.l.Info(msg)
}
func (g logger) Warnf(f string, a ...any)  { g.l.Warn(fmt.Sprintf(f, a...)) }
func (g logger) Errorf(f string, a ...any) { g.l.Error(fmt.Sprintf(f, a...)) }
func (g logger) Trace(a ...any)            {}
func (g logger) Debug(a ...any)            { g.l.Debug(fmt.Sprint(a...)) }
func (g logger) Info(a ...any)             { g.l.Info(fmt.Sprint(a...)) }
func (g logger) Warn(a ...any)             { g.l.Warn(fmt.Sprint(a...)) }
func (g logger) Error(a ...any)            { g.l.Error(fmt.Sprint(a...)) }

func (g logger) WithField(k string, v any) librespot.Logger {
	return logger{g.l.With(k, v), g.onLoginURL}
}
func (g logger) WithError(err error) librespot.Logger {
	return logger{g.l.With("err", err), g.onLoginURL}
}
