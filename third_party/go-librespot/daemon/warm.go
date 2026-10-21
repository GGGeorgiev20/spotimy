package daemon

import (
	"context"
	"io"
	"sync"
	"time"

	librespot "github.com/devgianlu/go-librespot"
	"github.com/devgianlu/go-librespot/player"
)

// warmStreams holds the stream of the track a play request names, created
// while the request's context resolves rather than after it: the two are
// independent network round trips, and the load that follows the resolve
// picks the stream up instead of creating its own.
type warmStreams struct {
	mu  sync.Mutex
	cur *warmStream
}

type warmStream struct {
	uri    string
	done   chan struct{}
	stream *player.Stream
	err    error
}

// warmTimeout bounds a warm stream that no load comes to collect.
const warmTimeout = 30 * time.Second

// warmStream starts creating the stream for uri at its beginning, replacing
// any warm stream not yet taken. Only tracks are warmed: episodes may resume
// mid-way, which is only known at load time.
func (p *AppPlayer) warmStream(uri string) {
	id, err := librespot.SpotifyIdFromUri(uri)
	if err != nil || id.Type() != librespot.SpotifyIdTypeTrack {
		return
	}
	w := &warmStream{uri: id.Uri(), done: make(chan struct{})}
	p.warm.mu.Lock()
	old := p.warm.cur
	p.warm.cur = w
	p.warm.mu.Unlock()
	if old != nil {
		go old.discard()
	}
	go func() {
		defer close(w.done)
		ctx, cancel := context.WithTimeout(context.Background(), warmTimeout)
		defer cancel()
		w.stream, w.err = p.player.NewStream(ctx, p.app.client, *id, p.app.cfg.Bitrate, 0)
	}()
	time.AfterFunc(warmTimeout, func() {
		p.warm.mu.Lock()
		stale := p.warm.cur == w
		if stale {
			p.warm.cur = nil
		}
		p.warm.mu.Unlock()
		if stale {
			w.discard()
		}
	})
}

// takeWarm returns the warm stream for id once it is ready, or nil when there
// is none for it or it failed (the caller then creates the stream itself).
func (p *AppPlayer) takeWarm(ctx context.Context, id librespot.SpotifyId) *player.Stream {
	p.warm.mu.Lock()
	w := p.warm.cur
	if w == nil || w.uri != id.Uri() {
		p.warm.mu.Unlock()
		return nil
	}
	p.warm.cur = nil
	p.warm.mu.Unlock()

	select {
	case <-w.done:
	case <-ctx.Done():
		go w.discard()
		return nil
	}
	if w.err != nil {
		p.app.log.WithError(w.err).WithField("uri", w.uri).Debugf("warm stream failed, loading again")
		return nil
	}
	p.app.log.WithField("uri", w.uri).Debugf("using the stream warmed while the context resolved")
	return w.stream
}

// discard closes a warm stream nobody took, once it is done being created.
func (w *warmStream) discard() {
	<-w.done
	if w.stream != nil {
		if c, ok := w.stream.Source.(io.Closer); ok {
			_ = c.Close()
		}
	}
}
