// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

// Port of org.apache.lucene.index.TestNRTReaderWithThreads (Lucene 10.4.0).
//
// Concurrent indexing threads exercise the near-real-time reader path:
// "adder" threads add documents while "reader" threads repeatedly open an
// NRT reader from the live IndexWriter (DirectoryReader.open(writer) ->
// OpenDirectoryReaderFromWriter), count a random id term, delete it, and
// close the reader. The test passes when no thread observes an error.
//
// Deviations from the reference, all immaterial to what the test asserts
// (no thread throws):
//   - MockDirectoryWrapper.setAssertNoDeleteOpenFile is unavailable; a
//     plain ByteBuffersDirectory is used.
//   - DocHelper.createDocument's term vectors are omitted (this test never
//     reads term vectors); the "id" field used for delete/count is faithful.
//   - MockAnalyzer is replaced by the WhitespaceAnalyzer.

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// nrtCreateDocument mirrors DocHelper.createDocument(n, indexName, numFields):
// an "id" StringField, an "indexname" StringField, and field1..fieldN
// TextFields whose text grows with n.
func nrtCreateDocument(t *testing.T, n int, indexName string, numFields int) index.Document {
	t.Helper()
	doc := document.NewDocument()
	idF, err := document.NewStringField("id", strconv.Itoa(n), true)
	if err != nil {
		t.Fatalf("id field: %v", err)
	}
	doc.Add(idF)
	nameF, err := document.NewStringField("indexname", indexName, true)
	if err != nil {
		t.Fatalf("indexname field: %v", err)
	}
	doc.Add(nameF)
	text := "a" + strconv.Itoa(n)
	f1, err := document.NewTextField("field1", text, true)
	if err != nil {
		t.Fatalf("field1: %v", err)
	}
	doc.Add(f1)
	text += " b" + strconv.Itoa(n)
	for i := 1; i < numFields; i++ {
		f, err := document.NewTextField("field"+strconv.Itoa(i+1), text, true)
		if err != nil {
			t.Fatalf("field%d: %v", i+1, err)
		}
		doc.Add(f)
	}
	return doc
}

func TestNRTReaderWithThreadsIndexing(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetMaxBufferedDocs(10)
	writer, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Open and close one NRT reader up front (Lucene "starts pooling readers").
	r0, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("initial OpenDirectoryReaderFromWriter: %v", err)
	}
	if err := r0.Close(); err != nil {
		t.Fatalf("initial reader Close: %v", err)
	}

	var seq atomic.Int64
	seq.Store(1)

	const numThreads = 2
	const numIterations = 50

	var (
		wg       sync.WaitGroup
		failMu   sync.Mutex
		failures []error
	)
	recordFailure := func(err error) {
		failMu.Lock()
		failures = append(failures, err)
		failMu.Unlock()
	}

	for x := 0; x < numThreads; x++ {
		typ := x % 2
		wg.Add(1)
		go func(typ int) {
			defer wg.Done()
			for iter := 0; iter < numIterations; iter++ {
				if typ == 0 {
					i := int(seq.Add(1))
					if err := writer.AddDocument(nrtCreateDocument(t, i, "index1", 10)); err != nil {
						recordFailure(err)
						return
					}
				} else {
					reader, err := index.OpenDirectoryReaderFromWriter(writer)
					if err != nil {
						recordFailure(err)
						return
					}
					id := int(seq.Load())
					idStr := strconv.Itoa(id)
					searcher := search.NewIndexSearcher(reader)
					if _, err := searcher.Search(search.NewTermQuery(index.NewTerm("id", idStr)), 1000); err != nil {
						_ = reader.Close()
						recordFailure(err)
						return
					}
					if err := writer.DeleteDocuments(index.NewTerm("id", idStr)); err != nil {
						_ = reader.Close()
						recordFailure(err)
						return
					}
					if err := reader.Close(); err != nil {
						recordFailure(err)
						return
					}
				}
			}
		}(typ)
	}

	wg.Wait()

	if err := writer.Close(); err != nil {
		t.Fatalf("writer Close: %v", err)
	}

	failMu.Lock()
	defer failMu.Unlock()
	for _, f := range failures {
		t.Errorf("thread failure: %v", f)
	}
}
