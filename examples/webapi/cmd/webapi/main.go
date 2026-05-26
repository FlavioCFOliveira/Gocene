// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Command webapi runs the Gocene CRUD demo HTTP API.
//
// Default behaviour: open (or create) an on-disk index under the OS temp
// directory, seed it from the embedded golden corpus if it is empty, and
// serve the REST endpoints on :8080.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/FlavioCFOliveira/Gocene/examples/webapi"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dataDir := flag.String("data", "", "directory for the Gocene index (default: <os.TempDir>/gocene-webapi-index)")
	flag.Parse()

	if *dataDir == "" {
		*dataDir = filepath.Join(os.TempDir(), "gocene-webapi-index")
	}

	store, err := webapi.OpenBookStore(*dataDir)
	if err != nil {
		log.Fatalf("open book store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("close book store: %v", err)
		}
	}()

	seeded, err := webapi.SeedIfEmpty(store)
	if err != nil {
		log.Fatalf("seed: %v", err)
	}
	if seeded > 0 {
		log.Printf("seeded %d books from the embedded golden corpus", seeded)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           webapi.NewServer(store),
		ReadHeaderTimeout: 5 * time.Second,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Printf("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("server shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	log.Printf("Gocene webapi demo listening on %s (index at %s)", *addr, *dataDir)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
	<-idleConnsClosed
}
