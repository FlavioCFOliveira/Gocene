// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/join/src/test/org/apache/lucene/search/join/TestQueryBitSetProducer.java
// (Apache Lucene 10.5.0).

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func qbspGetBitSet(t *testing.T, producer *QueryBitSetProducer, ctx *index.LeafReaderContext) util.BitSet {
	t.Helper()
	bs, err := producer.GetBitSet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return bs
}

func TestQueryBitSetProducerSimple(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(index.NewNoMergePolicy()) // NoMergePolicy.INSTANCE
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	mustAddDocument(t, w, document.NewDocument())
	reader := mustGetReader(t, w)

	producer := NewQueryBitSetProducer(search.MatchNoDocsQueryInstance)
	if bs := qbspGetBitSet(t, producer, mustLeaves(t, reader)[0]); bs != nil {
		t.Fatalf("expected null bit set, got %v", bs)
	}
	if got := len(producer.cache); got != 1 {
		t.Fatalf("cache size = %d, want 1", got)
	}

	producer = NewQueryBitSetProducer(search.Instance)
	bitSet := qbspGetBitSet(t, producer, mustLeaves(t, reader)[0])
	if got := bitSet.Length(); got != 1 {
		t.Fatalf("length = %d, want 1", got)
	}
	if !bitSet.Get(0) {
		t.Fatal("expected bit 0 set")
	}
	if got := len(producer.cache); got != 1 {
		t.Fatalf("cache size = %d, want 1", got)
	}

	mustClose(t, reader, w, dir)
}

func TestQueryBitSetProducerReaderNotSuitedForCaching(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(index.NewNoMergePolicy()) // NoMergePolicy.INSTANCE
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	mustAddDocument(t, w, document.NewDocument())
	// DirectoryReader reader = new DummyDirectoryReader(w.getReader()), a
	// FilterDirectoryReader whose leaves (FilterLeafReaders) and itself return
	// null cache helpers.
	t.Fatal(filterDirectoryReaderSubReaderWrapperBlocker)
	reader := mustGetReader(t, w)

	producer := NewQueryBitSetProducer(search.MatchNoDocsQueryInstance)
	if bs := qbspGetBitSet(t, producer, mustLeaves(t, reader)[0]); bs != nil {
		t.Fatalf("expected null bit set, got %v", bs)
	}
	if got := len(producer.cache); got != 0 {
		t.Fatalf("cache size = %d, want 0", got)
	}

	producer = NewQueryBitSetProducer(search.Instance)
	bitSet := qbspGetBitSet(t, producer, mustLeaves(t, reader)[0])
	if got := bitSet.Length(); got != 1 {
		t.Fatalf("length = %d, want 1", got)
	}
	if !bitSet.Get(0) {
		t.Fatal("expected bit 0 set")
	}
	if got := len(producer.cache); got != 0 {
		t.Fatalf("cache size = %d, want 0", got)
	}

	mustClose(t, reader, w, dir)
}
