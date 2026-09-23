// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestSortingStoredFieldsConsumer.java
// (Apache Lucene 10.5.0). The Java class uses
// Lucene90CompressingStoredFieldsWriter.FIELDS_EXTENSION from
// org.apache.lucene.codecs.lucene90.compressing, which imports index; the
// port lives in the external index_test package.

package index_test

import (
	"math/rand"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90/compressing"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// seekCountingDirectory renders the anonymous FilterDirectory subclass whose
// openInput wraps the stored fields data file in a seek-counting
// FilterIndexInput.
type seekCountingDirectory struct {
	*store.FilterDirectory
	numSeeks *atomic.Int32
}

func (d *seekCountingDirectory) OpenInput(name string, ctx store.IOContext) (store.IndexInput, error) {
	if strings.Contains(name, compressing.FieldsExtension) {
		in, err := d.FilterDirectory.OpenInput(name, ctx)
		if err != nil {
			return nil, err
		}
		return &seekCountingIndexInput{FilterIndexInput: store.NewFilterIndexInput(name, in), numSeeks: d.numSeeks}, nil
	}
	return d.FilterDirectory.OpenInput(name, ctx)
}

type seekCountingIndexInput struct {
	*store.FilterIndexInput
	numSeeks *atomic.Int32
}

// SetPosition overrides FilterIndexInput.seek(long).
func (in *seekCountingIndexInput) SetPosition(pos int64) error {
	in.numSeeks.Add(1)
	return in.FilterIndexInput.SetPosition(pos)
}

// identityDocMap renders the anonymous identity Sorter.DocMap.
type identityDocMap struct{ numDocs int }

func (m identityDocMap) OldToNew(docID int) int { return docID }
func (m identityDocMap) NewToOld(docID int) int { return docID }
func (m identityDocMap) Size() int              { return m.numDocs }

// TestSortingStoredFieldsConsumerFlushWithNoStoredFieldsSkipsPerDocumentSeeks
// verifies that when no stored fields are written, flushing the
// SortingStoredFieldsConsumer skips per-document seeks and reads on the stored
// fields file. Only the initial seek for checksum retrieval should occur.
func TestSortingStoredFieldsConsumerFlushWithNoStoredFieldsSkipsPerDocumentSeeks(t *testing.T) {
	dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	defer dir.Close()
	iwc := index.NewIndexWriterConfig()
	codec := iwc.GetCodec()
	// new SegmentInfo(dir, Version.LATEST, null, "_0", -1, false, false, codec,
	//     Collections.emptyMap(), StringHelper.randomId(), new HashMap<>(), null)
	si := index.NewSegmentInfo("_0", -1, dir)
	si.SetVersion(util.Latest.String())
	si.SetCodec(codec)
	si.SetDiagnostics(map[string]string{})
	if err := si.SetID(util.RandomId()); err != nil {
		t.Fatalf("setID: %v", err)
	}
	si.SetAttributes(map[string]string{})

	var numSeeks atomic.Int32
	trackingDir := &seekCountingDirectory{FilterDirectory: store.NewFilterDirectory(dir), numSeeks: &numSeeks}

	consumer := index.NewSortingStoredFieldsConsumer(codec, trackingDir, si)

	numDocs := 1 + rand.Intn(100)
	for i := 0; i < numDocs; i++ {
		if err := consumer.StartDocument(i); err != nil {
			t.Fatalf("startDocument: %v", err)
		}
		if err := consumer.FinishDocument(); err != nil {
			t.Fatalf("finishDocument: %v", err)
		}
	}
	if err := consumer.Finish(numDocs); err != nil {
		t.Fatalf("finish: %v", err)
	}

	si.SetMaxDoc(numDocs)
	state := index.NewSegmentWriteState(dir, si, index.NewFieldInfos(), nil,
		store.IOContextFlush(store.NewFlushInfo(numDocs, 10)))

	// Identity mapping: doc order is unchanged
	if err := consumer.Flush(state, identityDocMap{numDocs: numDocs}); err != nil {
		t.Fatalf("flush: %v", err)
	}

	reader, err := codec.StoredFieldsFormat().FieldsReader(dir, si, state.FieldInfos, store.IOContextReadOnce)
	if err != nil {
		t.Fatalf("fieldsReader: %v", err)
	}
	for i := 0; i < numDocs; i++ {
		// StoredFields.document(int): visit with a DocumentStoredFieldVisitor.
		visitor := document.NewDocumentStoredFieldVisitor()
		if err := reader.VisitDocument(i, visitor); err != nil {
			t.Fatalf("document(%d): %v", i, err)
		}
		doc := visitor.GetDocument()
		if doc == nil {
			t.Fatal("assertNotNull(document)")
		}
		if len(doc.GetFields()) != 0 {
			t.Fatalf("document %d: expected no fields, got %d", i, len(doc.GetFields()))
		}
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}

	if got := numSeeks.Load(); got != 1 {
		t.Fatalf("Expected only 1 seek (for checksum retrieval), not one per document: got %d", got)
	}
}
