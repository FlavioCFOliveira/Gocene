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

// TestNRTThreads verifies that concurrent indexing and NRT reader acquisition
// does not deadlock or produce inconsistent document counts.
func TestNRTThreads(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	var wg sync.WaitGroup
	// Start 3 indexing goroutines.
	for g := 0; g < 3; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 33; i++ {
				nrtAddDoc(t, w, "thread-doc", "threaded indexing content")
			}
		}(g)
	}

	// Start 2 reader goroutines.
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				reader, err := index.OpenDirectoryReaderFromWriter(w)
				if err != nil {
					t.Errorf("OpenDirectoryReaderFromWriter: %v", err)
					return
				}
				reader.Close()
			}
		}()
	}

	wg.Wait()

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()
	if got := reader.MaxDoc(); got != 99 {
		t.Fatalf("MaxDoc = %d, want 99", got)
	}
}
