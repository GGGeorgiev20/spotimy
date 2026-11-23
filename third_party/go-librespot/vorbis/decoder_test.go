package vorbis

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	librespot "github.com/devgianlu/go-librespot"
)

// A track decodes to its end: the samples of its last page, decoded along
// with the news that the stream is over, play before Read reports EOF.
// tone-1s.ogg is a generated 440Hz tone, one second (44100 frames) long.
func TestDecodesToTheEnd(t *testing.T) {
	data, err := os.ReadFile("testdata/tone-1s.ogg")
	if err != nil {
		t.Fatal(err)
	}
	d, err := New(&librespot.NullLogger{}, io.NewSectionReader(bytes.NewReader(data), 0, int64(len(data))), nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	buf := make([]float32, 4096)
	samples := 0
	for {
		n, err := d.Read(buf)
		samples += n
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
	}
	// Vorbis may decode a little past the end (the last page's granule trims
	// it), never short of it.
	if frames := samples / 2; frames < 44100 || frames > 44100+2048 {
		t.Errorf("decoded %d frames, want 44100", frames)
	}
}
