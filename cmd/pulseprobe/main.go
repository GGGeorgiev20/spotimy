package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	librespot "github.com/devgianlu/go-librespot"
	"github.com/devgianlu/go-librespot/output"
)

type short struct{ left int }

func (s *short) Read(b []float32) (int, error) {
	if s.left <= 0 {
		return 0, io.EOF
	}
	n := min(len(b), s.left)
	clear(b[:n])
	s.left -= n
	return n, nil
}

func main() {
	ms, _ := strconv.Atoi(os.Args[1])
	gap, _ := strconv.Atoi(os.Args[2])
	src := &short{}
	out, err := output.NewOutput(&output.NewOutputOptions{Log: &librespot.NullLogger{}, Backend: "pulseaudio", Reader: src, SampleRate: 44100, ChannelCount: 2, InitialVolume: 1, VolumeUpdate: make(chan float32, 8)})
	if err != nil {
		panic(err)
	}
	for round := 0; round < 3; round++ {
		src.left = 44100 * 2 * ms / 1000
		done := make(chan struct{})
		go func() { _ = out.Drop(); _ = out.Resume(); close(done) }()
		select {
		case <-done:
		case <-time.After(4 * time.Second):
			fmt.Println(ms, gap, "round", round, "HUNG")
			os.Exit(0)
		}
		time.Sleep(time.Duration(gap) * time.Millisecond)
	}
	fmt.Println(ms, gap, "ok")
	os.Exit(0)
}
