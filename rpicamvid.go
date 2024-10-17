package rpicamvid

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"syscall"
)

type Rpicamvid struct {
	additionalOpts []string
	cancel         context.CancelFunc
	lock           sync.RWMutex
	log            *log.Logger
	streams        map[streamID]chan []byte
	height         int
	width          int
	buffers        sync.Pool
}

func New(log *log.Logger, width, height int, opts ...string) *Rpicamvid {
	return &Rpicamvid{
		additionalOpts: opts,
		log:            log,
		streams:        make(map[streamID]chan []byte),
		height:         height,
		width:          width,
		buffers: sync.Pool{
			New: func() any {
				return bytes.Buffer{}
			},
		},
	}
}

func (r *Rpicamvid) Start() (*Stream, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if len(r.streams) == 0 {
		ctx, cancel := context.WithCancel(context.Background())
		r.cancel = cancel

		startupChan := make(chan error, 2)
		defer func() { startupChan <- nil }()

		// async camera frame reader
		go func() {
			options := []string{
				"--timeout", "0", // no timeout, run until explicitly closed
				"--width", fmt.Sprintf("%d", r.width),
				"--height", fmt.Sprintf("%d", r.height),
				"--nopreview",
				"--codec", "mjpeg",
				"--flush",
				"--output", "-", // stdout
			}
			options = append(options, r.additionalOpts...)
			cmd := exec.Command("rpicam-vid", options...)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				startupChan <- fmt.Errorf("failed to open stdout pipe: %w", err)
				return
			}

			scanner := bufio.NewScanner(stdout)
			// Estimated 24 bits per pixel
			buf := make([]byte, 0, r.width*r.height*3)
			// Use 4k resolution as the max
			scanner.Buffer(buf, 3840*2160*3)
			scanner.Split(mjpegSplitFunc)

			if err := cmd.Start(); err != nil {
				startupChan <- fmt.Errorf("failed to start rpicam-vid: %w", err)
				return
			}

			// Startup completed OK
			startupChan <- nil

			for {
				if ctx.Err() != nil {
					break
				}

				if !scanner.Scan() {
					if err := scanner.Err(); err != nil {
						r.log.Printf("Scan failed: %v\n", err)

						// Try to discard stdout data and recover
						// This might happen if we get an image frame that is too large for the current buffer
						_, _ = io.CopyN(io.Discard, stdout, int64(len(buf)))
						continue
					}
					// Scan returns false with no error when reaching EOF
					break
				}

				// Need to copy the bytes because the scanner may read the next frame into the current buffer before all the
				// stream consumers have had a chance to read it. Therefore the buffer we send to consumers must not be
				// modified.
				bb := scanner.Bytes()
				img := r.buffers.Get().(bytes.Buffer)
				img.Reset()
				img.Write(bb)

				// Send frame to all viewers
				r.lock.RLock()
				for _, stream := range r.streams {
					select {
					case stream <- img.Bytes():
					default:
						// Consumer is slow, drop a frame and add the newest frame
						// The buffer size is 2 to protect against deadlock due to consumer and this frame-dropper both reading
						// at the same time. In that case, both frames are read and we simply add the new frame.
						<-stream
						stream <- img.Bytes()
					}
				}
				r.lock.RUnlock()
				r.buffers.Put(img)
			}
			//r.log.Debug("Sending interrupt signal to rpicam-vid")
			if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
				r.log.Printf("Failed to send interrupt signal to rpicam-vid: %v\n", err)

				if err := cmd.Process.Kill(); err != nil {
					r.log.Printf("Failed to kill rpicam-vid: %v\n", err)
				}
			}

			// Flush stdout so Wait() can finish
			_, _ = io.Copy(io.Discard, stdout)

			r.log.Printf("Waiting for rpicam-vid to exit")
			if err := cmd.Wait(); err != nil {
				r.log.Printf("rpicam-vid wait for exit failed: %v\n", err)
			}
			r.log.Printf("Camera stopped")
		}()

		// Check if the goroutine started up OK
		if err := <-startupChan; err != nil {
			return nil, err
		}
	}

	frames := make(chan []byte, 2)

	s := newStream(frames, r.stop)
	r.streams[s.id] = frames

	return s, nil
}

func (r *Rpicamvid) stop(id streamID) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if frames, ok := r.streams[id]; ok {
		close(frames)
		delete(r.streams, id)
	}
	r.log.Printf("Stream subscriber stopped: %d remaining\n", len(r.streams))

	if len(r.streams) == 0 {
		r.log.Printf("Cancelling camera feed context\n")
		if r.cancel != nil {
			r.cancel()
			r.cancel = nil
		}
	}
}
