// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90DocValuesFormatMergeInstance.java
//
// In Lucene, TestLucene90DocValuesFormatMergeInstance extends
// TestLucene90DocValuesFormat and overrides shouldTestMergeInstance() to
// return true, causing the BaseIndexFileFormatTestCase harness to wrap
// every DirectoryReader with MergingDirectoryReaderWrapper so the
// inherited DocValuesProducer tests exercise the merge-optimised instance
// returned by DocValuesProducer.getMergeInstance().
//
// Gocene does not yet expose GetMergeInstance on DocValuesProducer
// (deferred to Sprint 22 along with the heavy per-field doc values
// encoding bodies). Until then the merge-instance variant is
// observationally identical to the regular producer, so this file
// re-runs the Phase 1 structural round-trip in "merge-instance mode" to
// keep the test-surface 1:1 with upstream and to act as a pin: when
// GetMergeInstance lands, this test will be extended to call it and
// assert identical framing.

package codecs_test

import (
	"crypto/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene90DocValuesFormatMergeInstance_EmptyRoundTrip is the
// merge-instance counterpart of the parent Lucene90DocValuesFormat
// round-trip. It mirrors the Java subclass which only flips
// shouldTestMergeInstance to true and otherwise reuses every parent
// assertion.
func TestLucene90DocValuesFormatMergeInstance_EmptyRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	si := index.NewSegmentInfo("_0", 10, dir)
	if err := si.SetID(id); err != nil {
		t.Fatal(err)
	}

	state := &codecs.SegmentWriteState{
		Directory:     dir,
		SegmentInfo:   si,
		SegmentSuffix: "",
	}

	format := codecs.NewLucene90DocValuesFormat()
	consumer, err := format.FieldsConsumer(state)
	if err != nil {
		t.Fatalf("FieldsConsumer: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("Close consumer: %v", err)
	}

	if !dir.FileExists("_0.dvd") {
		t.Error("expected _0.dvd to exist")
	}
	if !dir.FileExists("_0.dvm") {
		t.Error("expected _0.dvm to exist")
	}

	readState := &codecs.SegmentReadState{
		Directory:     dir,
		SegmentInfo:   si,
		SegmentSuffix: "",
	}
	producer, err := format.FieldsProducer(readState)
	if err != nil {
		t.Fatalf("FieldsProducer: %v", err)
	}

	// Merge-instance lens. Lucene exposes DocValuesProducer.getMergeInstance(),
	// defaulting to "return this". Gocene does not yet expose this hook
	// (Sprint 22); the producer itself is the merge-instance equivalent.
	// When GetMergeInstance lands, replace this aliasing with the real
	// call and assert framing parity against the regular producer.
	mergeInstance := producer

	if err := mergeInstance.CheckIntegrity(); err != nil {
		t.Fatalf("merge-instance CheckIntegrity: %v", err)
	}

	if err := producer.Close(); err != nil {
		t.Fatalf("Close producer: %v", err)
	}
}
