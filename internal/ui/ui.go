// Package ui is the native Gio interface: the app shell, the shared widget
// kit and the stores pages read from. Pages and panels live in subpackages
// and register themselves in init.
//
// Threading: everything in this package runs on the UI goroutine. Background
// work goes through Async (or Post), which hands results back to the UI
// goroutine before the next frame, so no UI state is ever touched concurrently.
package ui

import (
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/GGGeorgiev20/personal-spotify/internal/backend"
)

var (
	// B is the backend: session, player and internal API.
	B   *backend.App
	Log *slog.Logger

	postMu     sync.Mutex
	posted     []func()
	invalidate atomic.Pointer[func()]
	inflight   atomic.Int32 // Async calls not yet delivered (the harness waits for 0)
)

// Invalidate asks for a new frame. Safe from any goroutine.
func Invalidate() {
	if f := invalidate.Load(); f != nil {
		(*f)()
	}
}

// Post runs f on the UI goroutine before the next frame. Safe from any goroutine.
func Post(f func()) {
	postMu.Lock()
	posted = append(posted, f)
	postMu.Unlock()
	Invalidate()
}

func runPosted() {
	for {
		postMu.Lock()
		fs := posted
		posted = nil
		postMu.Unlock()
		if len(fs) == 0 {
			return
		}
		for _, f := range fs {
			f()
		}
	}
}

// Async runs work on a goroutine and done with its result on the UI goroutine.
func Async[T any](work func() (T, error), done func(T, error)) {
	inflight.Add(1)
	go func() {
		v, err := work()
		Post(func() {
			inflight.Add(-1)
			done(v, err)
		})
	}()
}

// Go runs work on a goroutine and reports an error as a toast (prefix: what failed).
func Go(prefix string, work func() error, then func()) {
	Async(func() (struct{}, error) { return struct{}{}, work() }, func(_ struct{}, err error) {
		if err != nil {
			Toast(prefix + ": " + err.Error())
			return
		}
		if then != nil {
			then()
		}
	})
}

// Res holds the result of an async load. The zero value is ready to use.
type Res[T any] struct {
	V       T
	Err     error
	Done    bool
	started bool
	gen     int
}

// Once starts f the first time it's called; later calls do nothing.
func (r *Res[T]) Once(f func() (T, error)) {
	if !r.started {
		r.Load(f)
	}
}

// Load (re)starts f. A result from an older Load is dropped.
func (r *Res[T]) Load(f func() (T, error)) {
	r.started = true
	r.gen++
	gen := r.gen
	Async(f, func(v T, err error) {
		if gen != r.gen {
			return
		}
		r.V, r.Err, r.Done = v, err, true
	})
}

// Reload starts f again but keeps showing the current value until it arrives.
func (r *Res[T]) Reload(f func() (T, error)) {
	r.Load(f)
}

// Ok reports a successful, finished load.
func (r *Res[T]) Ok() bool { return r.Done && r.Err == nil }
