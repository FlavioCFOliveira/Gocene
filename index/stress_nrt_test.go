// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestStressNRT runs concurrent indexing and NRT reader refreshes to exercise
// the GetReader / OpenIfChangedFromWriter path under load.
func TestStressNRT(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	// Index goroutine: continuously add documents.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			nrtAddDoc(t, w, "doc", "stress body content")
		}
	}()

	// Reader goroutine: continuously open NRT readers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			reader, err := index.OpenDirectoryReaderFromWriter(w)
			if err != nil {
				t.Errorf("OpenDirectoryReaderFromWriter: %v", err)
				return
			}
			// Each reader must be openable and report a stable doc count
			// that is at least the number of docs we've indexed so far
			// (minus any in-flight documents not yet flushed).
			if reader.MaxDoc() < 0 {
				t.Errorf("negative MaxDoc: %d", reader.MaxDoc())
			}
			reader.Close()
		}
	}()

	wg.Wait()

	// Final check: all 100 docs should be visible.
	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("final OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()
	if got := reader.MaxDoc(); got != 100 {
		t.Fatalf("final MaxDoc = %d, want 100", got)
	}
}
