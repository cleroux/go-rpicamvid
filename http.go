package rpicamvid

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"syscall"
)

// ContextMiddleware replaces the request context with the given context.
// It can be used to set a cancellable context only for streaming endpoints since the HTTP Server will not cancel
// in-flight requests on shutdown.
func ContextMiddleware(ctx context.Context, next http.HandlerFunc) http.HandlerFunc {
	if ctx == nil {
		return next
	}

	return func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// HTTPHandler is an HTTP route handler that responds with an MJPEG video stream from the rpicam-vid application.
func (r *Rpicamvid) HTTPHandler(w http.ResponseWriter, req *http.Request) {
	stream, err := r.Start()
	if err != nil {
		r.log.Printf("Failed to start camera: %v\n", err)
		http.Error(w, "Failed to start camera: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	mimeWriter := multipart.NewWriter(w)
	defer mimeWriter.Close()
	contentType := fmt.Sprintf("multipart/x-mixed-replace; boundary=%s", mimeWriter.Boundary())
	w.Header().Set("Content-Type", contentType)

	partHeader := make(textproto.MIMEHeader, 1)
	partHeader.Add("Content-Type", "image/jpeg")

	ctx := req.Context()

	for {
		if ctx.Err() != nil {
			return
		}

		err := func() error {
			f, err := stream.GetFrame()
			if err != nil {
				r.log.Printf("Failed to get camera frame: %v\n", err)
				return nil // continue for loop
			}
			defer f.Close()

			partWriter, err := mimeWriter.CreatePart(partHeader)
			if err != nil {
				r.log.Printf("Failed to create multi-part section: %v\n", err)
				return err
			}

			if _, err := partWriter.Write(f.GetBytes()); err != nil {
				if errors.Is(err, syscall.EPIPE) {
					// Client went away
					return err
				}

				switch err.Error() {
				case "http2: stream closed", "client disconnected":
					// Client went away
					return err
				}
				r.log.Printf("Failed to write video frame: %v\n", err)
				return err
			}
			return nil
		}()
		if err != nil {
			break
		}
	}
}
