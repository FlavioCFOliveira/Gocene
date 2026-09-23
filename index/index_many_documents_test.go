// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexManyDocuments.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestIndexManyDocuments(t *testing.T) {
	dir := newFSDirectory(t)
	iwc := index.NewIndexWriterConfig()
	iwc.SetMaxBufferedDocs(nextInt(100, 2000))

	numDocs := atLeast(10000)

	w := mustNewIndexWriter(t, dir, iwc)
	var count atomic.Int32
	var threads sync.WaitGroup
	for i := 0; i < 2; i++ {
		threads.Add(1)
		go func() {
			defer threads.Done()
			for int(count.Add(1)-1) < numDocs {
				doc := document.NewDocument()
				doc.Add(newTextField(t, "field", "text", false))
				if _, err := w.AddDocument(doc); err != nil {
					t.Errorf("addDocument: %v", err)
					return
				}
			}
		}()
	}
	threads.Wait()

	if maxDoc := iwDocStats(t, w).MaxDoc; maxDoc != numDocs {
		t.Fatalf("lost %d documents; maxBufferedDocs=%d", numDocs-maxDoc, iwc.GetMaxBufferedDocs())
	}
	mustClose(t, w)

	r := mustOpenDirectoryReader(t, dir)
	if r.MaxDoc() != numDocs {
		t.Fatalf("expected maxDoc %d, got %d", numDocs, r.MaxDoc())
	}
	mustClose(t, r, dir)
}
