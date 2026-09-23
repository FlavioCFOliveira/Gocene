// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestFilterLeafReader.java
// (Apache Lucene 10.5.0). The Java class uses the test-framework
// RandomIndexWriter (tests/index imports index), so the port lives in the
// external index_test package.

package index_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
)

// TestFilterLeafReaderFilterIndexReader ports testFilterIndexReader, which
// wraps a TestReader (a FilterLeafReader) with SlowCodecReaderWrapper.wrap and
// adds it to a second index through IndexWriter#addIndexes(CodecReader...).
// Gocene's IndexWriter only declares addIndexes(Directory...), and
// BaseDirectoryWrapper#setCrossCheckTermVectorsOnClose is not ported.
func TestFilterLeafReaderFilterIndexReader(t *testing.T) {
	t.Fatal("org.apache.lucene.index.IndexWriter#addIndexes(CodecReader...) and " +
		"org.apache.lucene.tests.store.BaseDirectoryWrapper#setCrossCheckTermVectorsOnClose(boolean) " +
		"are not ported")
}

// TestFilterLeafReaderOverrideMethods ports testOverrideMethods, which walks
// the public methods of FilterLeafReader, FilterFields, FilterTerms,
// FilterTermsEnum and FilterPostingsEnum through java.lang.reflect and fails
// when a filter class overrides a superclass method that has a default
// implementation. Java class-hierarchy reflection has no Go counterpart.
func TestFilterLeafReaderOverrideMethods(t *testing.T) {
	t.Fatal("testOverrideMethods relies on java.lang.reflect (Class#getMethods, " +
		"Method#getDeclaringClass) over the FilterLeafReader class hierarchy; no Go rendering exists")
}

// unwrapTestFilterLeafReader renders the anonymous FilterLeafReader subclass
// of testUnwrap, whose cache helpers delegate to the wrapped reader.
type unwrapTestFilterLeafReader struct {
	*index.FilterLeafReader
	in index.LeafReader
}

func (r *unwrapTestFilterLeafReader) GetCoreCacheHelper() index.CacheHelper {
	return r.in.GetCoreCacheHelper()
}

func (r *unwrapTestFilterLeafReader) GetReaderCacheHelper() index.CacheHelper {
	return r.in.GetReaderCacheHelper()
}

func TestFilterLeafReaderUnwrap(t *testing.T) {
	dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	w, err := testindex.NewRandomIndexWriter(rand.New(rand.NewSource(rand.Int63())), dir)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}
	if _, err := w.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	dr, err := w.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	leaves, err := dr.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	r := leaves[0].LeafReader()
	r2 := &unwrapTestFilterLeafReader{FilterLeafReader: index.NewFilterLeafReader(r), in: r}
	if r2.GetDelegate() != r {
		t.Fatal("assertEquals(r, r2.getDelegate())")
	}
	if index.UnwrapFilterLeafReader(r2) != r {
		t.Fatal("assertEquals(r, FilterLeafReader.unwrap(r2))")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := dr.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	if err := dir.Close(); err != nil {
		t.Fatalf("close dir: %v", err)
	}
}
