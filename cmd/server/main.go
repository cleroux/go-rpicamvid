package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cleroux/go-rpicamvid"
)

var (
	flagAddr     = flag.String("addr", "127.0.0.1:8080", "Listen on this address:port for HTTP requests")
	flagHeight   = flag.Int("height", 720, "Video height")
	flagWidth    = flag.Int("width", 1280, "Video width")
	flagRotation = flag.Int("rotation", 0, "Video rotation")
)

func main() {
	flag.Parse()

	l := log.New(os.Stdout, "", log.LstdFlags)

	var opts []string
	if *flagRotation != 0 {
		opts = append(opts, "--rotation", fmt.Sprintf("%d", *flagRotation))
	}

	cam := rpicamvid.New(l, *flagWidth, *flagHeight, opts...)

	// Create a context that will allow us to cancel active video streams
	// We _could_ use this context as the HTTP Server's BaseContext but this would have the side effect of cancelling
	// all in-flight requests, not just our video streams.
	cancelCtx, cancelStreams := context.WithCancel(context.Background())

	// Set up routes and HTTP server
	m := http.NewServeMux()
	m.HandleFunc("GET /", rpicamvid.ContextMiddleware(cancelCtx, rpicamvid.ExactPathMiddleware("/", cam.HTTPHandler)))
	s := http.Server{
		Addr:    *flagAddr,
		Handler: m,
	}
	s.RegisterOnShutdown(cancelStreams)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		<-ctx.Done()

		l.Println("Shutting down HTTP server")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := s.Shutdown(shutdownCtx); err != nil {
			l.Printf("Failed to shut down HTTP server: %v\n", err)
		}
	}()

	l.Printf("HTTP server listening on %s\n", *flagAddr)
	if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		l.Printf("HTTP server failed: %v\n", err)
		stop()
	}

	// Wait for HTTP server to shut down gracefully
	wg.Wait()
}
