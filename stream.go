package rpicamvid

import (
	"errors"
	"sync/atomic"
)

type streamID uint64

var ErrClosed = errors.New("stream closed")
var ErrNoFrames = errors.New("no image frames to read")

var streamId atomic.Uint64

// Stream provides []byte frames.
// It is intended to be used by a single consumer.
type Stream struct {
	closed bool
	frames chan []byte
	id     streamID
	stop   func(id streamID)
}

func newStream(frames chan []byte, stop func(streamID)) *Stream {
	return &Stream{
		id:     streamID(streamId.Add(1)),
		frames: frames,
		stop:   stop,
	}
}

// GetFrame returns a single image frame, blocking to wait until the next frame if necessary.
func (s *Stream) GetFrame() ([]byte, error) {
	if s.closed {
		return nil, ErrClosed
	}

	frame, ok := <-s.frames
	if !ok {
		return nil, ErrNoFrames
	}
	return frame, nil
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
