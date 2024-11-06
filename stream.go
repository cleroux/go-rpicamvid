package rpicamvid

import (
	"errors"
	"sync"
	"sync/atomic"
)

type streamID uint64

var ErrClosed = errors.New("stream closed")
var ErrNoFrames = errors.New("no image frames to read")

var streamId atomic.Uint64

type Frame struct {
	img []byte
	wg  *sync.WaitGroup
}

func (f *Frame) GetBytes() []byte {
	return f.img
}

func (f *Frame) Close() error {
	f.wg.Done()
	return nil
}

// Stream provides []byte frames.
// It is intended to be used by a single consumer.
type Stream struct {
	closed bool
	frames chan Frame
	id     streamID
	stop   func(id streamID)
}

// func newStream(frames chan []byte, stop func(streamID)) *Stream {
func newStream(frames chan Frame, stop func(streamID)) *Stream {
	return &Stream{
		id:     streamID(streamId.Add(1)),
		frames: frames,
		stop:   stop,
	}
}

// GetFrame returns a single image frame, blocking to wait until the next frame if necessary.
func (s *Stream) GetFrame() (*Frame, error) {
	if s.closed {
		return nil, ErrClosed
	}

	f, ok := <-s.frames
	if !ok {
		return nil, ErrNoFrames
	}
	return &f, nil
}

// Close closes the stream.
func (s *Stream) Close() {
	if s.closed {
		return
	}
	s.closed = true
	if s.stop != nil {
		s.stop(s.id)
	}
}
