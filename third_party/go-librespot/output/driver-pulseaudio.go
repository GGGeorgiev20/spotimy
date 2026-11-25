package output

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	librespot "github.com/devgianlu/go-librespot"
	"github.com/jfreymuth/pulse"
	"github.com/jfreymuth/pulse/proto"
)

type pulseAudioOutput struct {
	log librespot.Logger

	sampleRate           int
	reader               librespot.Float32Reader
	client               *pulse.Client
	stream               *pulse.PlaybackStream
	volume               proto.Volume
	volumeLock           sync.Mutex
	externalVolumeUpdate chan float32
	err                  chan error

	// ended is set once the source has run out; see float32Reader.
	ended atomic.Bool

	// corked is a stream paused with what it buffered kept; see Pause.
	corked bool

	done chan struct{} // closed by Close
}

func newPulseAudioOutput(opts *NewOutputOptions) (*pulseAudioOutput, error) {
	// Initialize the PulseAudio client.
	// The device name is shown by PulseAudio volume controls (usually built
	// into a desktop environment), so we might want to use device_name here.
	// We could also maybe change the application icon name by device_type.
	clientopts := []pulse.ClientOption{pulse.ClientApplicationName("go-librespot"), pulse.ClientApplicationIconName("speaker")}

	if opts.RuntimeSocket != "" {
		clientopts = append(clientopts, pulse.ClientServerString(opts.RuntimeSocket))
	}

	client, err := pulse.NewClient(clientopts...)
	if err != nil {
		return nil, err
	}
	out := &pulseAudioOutput{
		log:                  opts.Log,
		sampleRate:           opts.SampleRate,
		reader:               opts.Reader,
		client:               client,
		externalVolumeUpdate: opts.VolumeUpdate,
		err:                  make(chan error, 2),
		done:                 make(chan struct{}),
	}

	// Create a new playback.
	var channelOpt pulse.PlaybackOption
	switch opts.ChannelCount {
	case 1:
		channelOpt = pulse.PlaybackMono
	case 2:
		channelOpt = pulse.PlaybackStereo
	default:
		return nil, fmt.Errorf("cannot play %d channels, pulse only supports mono and stereo", opts.ChannelCount)
	}
	volumeUpdates := make(chan proto.ChannelVolumes, 1)
	lplaybackopts := []pulse.PlaybackOption{
		pulse.PlaybackSampleRate(out.sampleRate),
		pulse.PlaybackVolumeChanges(volumeUpdates),
		channelOpt,
	}

	if opts.Device != "" {
		var lsink *pulse.Sink
		if opts.Device == "default" {
			lsink, err = client.DefaultSink()
		} else {
			lsink, err = client.SinkByID(opts.Device)
		}

		if err != nil {
			client.Close()
			return nil, fmt.Errorf("cannot find pulseaudio sink %s: %w", opts.Device, err)
		}

		lplaybackopts = append(lplaybackopts, pulse.PlaybackSink(lsink))
	}

	out.stream, err = out.client.NewPlayback(pulse.Float32Reader(out.float32Reader), lplaybackopts...)
	if err != nil {
		return nil, err
	}

	// Read the initial volume from PulseAudio.
	// PulseAudio strongly recommends against setting a default volume at
	// startup (especially if it's 100%), so instead we just follow the
	// PulseAudio provided volume.
	cvol, _ := out.stream.Volume()
	out.volume = cvol.Avg()
	sendVolumeUpdate(opts.VolumeUpdate, float32(out.volume.Norm()))

	go out.watchServer()

	// Listen for volume changes (through the volume mixer application, usually
	// built into the desktop environment), and send them back to Spotify.
	go func() {
		for cvol := range volumeUpdates {
			volume := cvol.Avg()

			out.volumeLock.Lock()
			if volume != out.volume {
				sendVolumeUpdate(opts.VolumeUpdate, float32(volume.Norm()))
				out.volume = volume
			}
			out.volumeLock.Unlock()
		}
	}()

	return out, nil
}

// float32Reader feeds PulseAudio from the source. When the source runs out
// the stream is kept running on silence rather than ended: the library ends a
// stream by marking it idle, and then drops PulseAudio's report that playback
// started, which starting it again waits for. A stream that ended before that
// report, as a track resumed in its last two seconds does, or that was started
// again after ending, never started: Resume waited for good and took the
// player with it. The source is not read again until Resume, so that nothing
// of the next track is fed in behind the silence and then flushed with it.
func (out *pulseAudioOutput) float32Reader(buf []float32) (int, error) {
	if out.ended.Load() {
		clear(buf)
		return len(buf), nil
	}
	n, err := out.reader.Read(buf)
	if err != nil {
		if err == io.EOF {
			out.ended.Store(true)
			clear(buf[n:])
			return len(buf), nil
		}

		// Encountered another error. This will result in a stopped player, so
		// send the error back to the player using a non-blocking send.
		select {
		case out.err <- err:
		default:
		}
		return n, err
	}
	return n, err
}

// watchServer reports the sound server going away (PipeWire restarted by an
// update, say). The library only marks the stream lost and stops asking for
// samples, so nothing else would: the output went on taking tracks and
// playing none of them, silent until the app was restarted. Reported, the
// output is closed, and the next track opens a new one.
func (out *pulseAudioOutput) watchServer() {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-out.done:
			return
		case <-tick.C:
			if out.stream.Closed() {
				select {
				case out.err <- pulse.ErrConnectionClosed:
				default:
				}
				return
			}
		}
	}
}

// errNotStarted is a stream that didn't start playing in time.
var errNotStarted = errors.New("pulseaudio: playback did not start")

// startTimeout bounds how long Resume waits for playback to start. The library
// waits for the server's word that it has, which never comes if the stream
// fails first, and the player, waiting on Resume, would wait with it for good.
const startTimeout = 10 * time.Second

// Pause corks the stream: it stops at once and keeps the two seconds it has
// buffered, to play on from the very same sample. It used to stop and flush
// them, and play then had to seek back to where it paused, which Vorbis can
// only do to a page boundary: every resume replayed a fraction of a second.
// The cork is asked for directly, not through the library's Pause and Resume:
// its Resume forgets an underflow, and the server's word that playback started
// again after it would then block the library's reader for good.
func (out *pulseAudioOutput) Pause() error {
	if !out.stream.Running() || out.corked {
		return nil
	}
	err := out.client.RawRequest(&proto.CorkPlaybackStream{
		StreamIndex: out.stream.StreamIndex(),
		Corked:      true,
	}, nil)
	if err != nil {
		return fmt.Errorf("Pause: could not cork playback: %w", err)
	}
	out.corked = true
	return nil
}

// Holding reports whether the stream is paused with its buffer kept.
func (out *pulseAudioOutput) Holding() bool {
	return out.corked
}

func (out *pulseAudioOutput) Resume() error {
	if out.stream.Closed() {
		return pulse.ErrConnectionClosed
	}
	if out.corked {
		// Plays on what it kept, the end of a track that ran out included.
		err := out.client.RawRequest(&proto.CorkPlaybackStream{
			StreamIndex: out.stream.StreamIndex(),
			Corked:      false,
		}, nil)
		if err != nil {
			return fmt.Errorf("Resume: could not uncork playback: %w", err)
		}
		out.corked = false
		return nil
	}

	// Start the stream. This will start reading samples from out.reader and
	// push it to PulseAudio. It will do nothing if the playback is already
	// started.
	if out.ended.Load() {
		// Playing silence since the source ran out: flush it so the next
		// source starts at once.
		if err := out.Drop(); err != nil {
			return err
		}
		out.ended.Store(false)
	}
	started := make(chan struct{})
	go func() {
		out.stream.Start()
		close(started)
	}()
	select {
	case <-started:
		return nil
	case <-time.After(startTimeout):
		return errNotStarted
	}
}

func (out *pulseAudioOutput) Drop() error {
	// A corked stream is left corked: Start uncorks it.
	out.corked = false
	if out.stream.Running() {
		// Stop and flush the buffer. We do not restart here: the caller
		// resumes once the new source is set. Restarting raced with the
		// source switch and clipped the start of the track (#292).
		out.stream.Stop()
		err := out.client.RawRequest(&proto.FlushPlaybackStream{
			StreamIndex: out.stream.StreamIndex(),
		}, nil)
		if err != nil {
			return fmt.Errorf("Drop: could not flush playback: %e", err)
		}
	} else {
		// Already stopped.
	}
	return nil
}

func (out *pulseAudioOutput) DelayMs() (int64, error) {
	samples := out.stream.BufferSize()
	delay := int64(samples) * 1000 / int64(out.sampleRate)
	return delay, nil
}

func (out *pulseAudioOutput) SetVolume(vol float32) {
	volume := proto.NormVolume(float64(vol))

	out.volumeLock.Lock()
	if volume == out.volume {
		out.volumeLock.Unlock()
		return
	}
	out.volume = volume
	sendVolumeUpdate(out.externalVolumeUpdate, vol)
	out.volumeLock.Unlock()

	cvol := proto.ChannelVolumes{volume}
	err := out.stream.SetVolume(cvol)
	if err != nil {
		out.log.WithError(err).Warn("failed to set volume")
	}
}

func (out *pulseAudioOutput) Error() <-chan error {
	return out.err
}

func (out *pulseAudioOutput) Close() error {
	select {
	case <-out.done:
	default:
		close(out.done)
	}
	out.stream.Close()
	out.client.Close()
	return nil
}
