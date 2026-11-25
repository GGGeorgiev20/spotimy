package vorbis

import (
	"bytes"
	"errors"
	"io"
	"os"
	"slices"
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

// Audio after a seek is the track's own: none of what was decoded before the
// seek is blended into it.
func TestSeekStartsClean(t *testing.T) {
	data, err := os.ReadFile("testdata/tone-1s.ogg")
	if err != nil {
		t.Fatal(err)
	}
	open := func() *Decoder {
		m := &MetadataPage{seekSamples: 44100, seekSize: uint32(len(data)), seekTable: make([]int32, 100)}
		for k := range m.seekTable {
			m.seekTable[k] = int32((k + 1) * 65536 / 100)
		}
		d, err := New(&librespot.NullLogger{}, io.NewSectionReader(bytes.NewReader(data), 0, int64(len(data))), m, 1)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(d.Close)
		return d
	}
	read := func(d *Decoder, max int) []float32 {
		var out []float32
		buf := make([]float32, 1024)
		for len(out) < max {
			n, err := d.Read(buf)
			out = append(out, buf[:n]...)
			if err != nil {
				break
			}
		}
		return out
	}

	ref := read(open(), 1<<20)
	d := open()
	read(d, 30000) // decoding, as a playing track is
	if err := d.SetPositionMs(500); err != nil {
		t.Fatal(err)
	}
	got := read(d, 8192)
	// Find where in the track the seek landed by its later samples, then
	// compare from the first one.
	for off := 0; off+len(got) <= len(ref); off += 2 {
		if slices.Equal(ref[off+4096:off+len(got)], got[4096:]) {
			if !slices.Equal(ref[off:off+4096], got[:4096]) {
				t.Error("the samples right after the seek differ from the track's")
			}
			return
		}
	}
	t.Fatal("audio after the seek isn't the track's")
}

// The position is how far reading has got, not the end of the page decoded.
func TestPositionIsWhatWasRead(t *testing.T) {
	data, err := os.ReadFile("testdata/tone-1s.ogg")
	if err != nil {
		t.Fatal(err)
	}
	d, err := New(&librespot.NullLogger{}, io.NewSectionReader(bytes.NewReader(data), 0, int64(len(data))), nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	buf := make([]float32, 2*22050) // half a second of stereo
	for read := 0; read < len(buf); {
		n, err := d.Read(buf[read:])
		if err != nil {
			t.Fatal(err)
		}
		read += n
	}
	if pos := d.PositionMs(); pos < 490 || pos > 510 {
		t.Errorf("half a second read, position %dms", pos)
	}
}
