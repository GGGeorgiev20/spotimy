//go:build test_unit

package output

import (
	"io"
	"testing"
)

type countedReader struct {
	left, reads int
}

func (r *countedReader) Read(b []float32) (int, error) {
	r.reads++
	if r.left <= 0 {
		return 0, io.EOF
	}
	n := min(len(b), r.left)
	for i := range b[:n] {
		b[i] = 1
	}
	r.left -= n
	return n, nil
}

// At the end of the source the stream goes on with silence, without reading
// the source again, until Resume starts it over.
func TestPulseReaderEndsOnSilence(t *testing.T) {
	src := &countedReader{left: 3}
	out := &pulseAudioOutput{reader: src}
	buf := make([]float32, 8)
	if n, err := out.float32Reader(buf); n != 3 || err != nil {
		t.Fatalf("got %d %v", n, err)
	}
	if n, err := out.float32Reader(buf); n != 8 || err != nil || buf[0] != 0 {
		t.Fatalf("at the end: %d %v %v", n, err, buf)
	}
	reads := src.reads
	if n, err := out.float32Reader(buf); n != 8 || err != nil || src.reads != reads {
		t.Fatalf("after the end: %d %v, %d reads", n, err, src.reads-reads)
	}
}
