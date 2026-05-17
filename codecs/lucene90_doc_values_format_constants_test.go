// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Phase 1 structural tests for Lucene90DocValuesFormat. Per-field
// encoding is deferred to Sprint 22.

package codecs_test

import (
	"crypto/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene90DocValuesFormat_Constants pins the codec, extension,
// version and type-byte constants.
func TestLucene90DocValuesFormat_Constants(t *testing.T) {
	for _, c := range []struct {
		name, got, want string
	}{
		{"DataCodec", codecs.Lucene90DocValuesDataCodec, "Lucene90DocValuesData"},
		{"MetaCodec", codecs.Lucene90DocValuesMetaCodec, "Lucene90DocValuesMetadata"},
		{"DataExtension", codecs.Lucene90DocValuesDataExtension, "dvd"},
		{"MetaExtension", codecs.Lucene90DocValuesMetaExtension, "dvm"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
	if codecs.Lucene90DocValuesVersionStart != 0 || codecs.Lucene90DocValuesVersionCurrent != 0 {
		t.Errorf("VersionStart=%d, VersionCurrent=%d; want both 0",
			codecs.Lucene90DocValuesVersionStart, codecs.Lucene90DocValuesVersionCurrent)
	}
	// Type-byte sentinels.
	cases := []struct {
		name string
		got  byte
		want byte
	}{
		{"Numeric", codecs.Lucene90DocValuesTypeNumeric, 0},
		{"Binary", codecs.Lucene90DocValuesTypeBinary, 1},
		{"Sorted", codecs.Lucene90DocValuesTypeSorted, 2},
		{"SortedSet", codecs.Lucene90DocValuesTypeSortedSet, 3},
		{"SortedNumeric", codecs.Lucene90DocValuesTypeSortedNumeric, 4},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("TypeByte %s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

// TestLucene90DocValuesFormat_BlockShifts verifies the block-shift and
// derived size/mask constants used by the dictionary / numeric block
// encoders.
func TestLucene90DocValuesFormat_BlockShifts(t *testing.T) {
	if codecs.Lucene90DocValuesNumericBlockShift != 14 ||
		codecs.Lucene90DocValuesNumericBlockSize != 16384 {
		t.Errorf("NumericBlockShift=%d Size=%d; want 14 / 16384",
			codecs.Lucene90DocValuesNumericBlockShift, codecs.Lucene90DocValuesNumericBlockSize)
	}
	if codecs.Lucene90DocValuesTermsDictBlockLZ4Size != 64 ||
		codecs.Lucene90DocValuesTermsDictBlockLZ4Mask != 63 {
		t.Errorf("TermsDictBlockLZ4Size=%d Mask=%d; want 64 / 63",
			codecs.Lucene90DocValuesTermsDictBlockLZ4Size, codecs.Lucene90DocValuesTermsDictBlockLZ4Mask)
	}
	if codecs.Lucene90DocValuesTermsDictReverseIndexSize != 1024 ||
		codecs.Lucene90DocValuesTermsDictReverseIndexMask != 1023 {
		t.Errorf("TermsDictReverseIndexSize=%d Mask=%d; want 1024 / 1023",
			codecs.Lucene90DocValuesTermsDictReverseIndexSize, codecs.Lucene90DocValuesTermsDictReverseIndexMask)
	}
}

// TestLucene90DocValuesFormat_SkipIndexJumpLengthPerLevel verifies the
// per-level jump table the skip-index uses. The Lucene 10.4.0 static
// initialiser computes the table at class load; we mirror it lazily.
func TestLucene90DocValuesFormat_SkipIndexJumpLengthPerLevel(t *testing.T) {
	jumps := codecs.Lucene90DocValuesSkipIndexJumpLengthPerLevel()
	// Expected values derived from the Lucene formula:
	//   jumps[0] = 29 - 5 = 24
	//   jumps[1] = 24 + (8 * 29) - 1   = 255
	//   jumps[2] = 255 + (64 * 29) - 8 = 2103
	//   jumps[3] = 2103 + (512 * 29) - 64 = 16887
	want := [4]int64{24, 255, 2103, 16887}
	for i, exp := range want {
		if jumps[i] != exp {
			t.Errorf("jumps[%d] = %d, want %d", i, jumps[i], exp)
		}
	}
}

// TestLucene90DocValuesFormat_EmptyRoundTrip writes an empty doc-values
// pair via Consumer.Close and verifies the Producer accepts the
// resulting framing.
func TestLucene90DocValuesFormat_EmptyRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	si := index.NewSegmentInfo("_0", 0, dir)
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
		t.Fatalf("consumer.Close: %v", err)
	}
	if !dir.FileExists("_0.dvd") || !dir.FileExists("_0.dvm") {
		t.Fatal("expected _0.dvd / _0.dvm to exist after Close")
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
	if err := producer.Close(); err != nil {
		t.Fatalf("producer.Close: %v", err)
	}
}

// TestLucene90DocValuesFormat_InvalidSkipInterval verifies the
// constructor's lower-bound check (matches Java IAE).
func TestLucene90DocValuesFormat_InvalidSkipInterval(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on invalid skip interval")
		}
	}()
	codecs.NewLucene90DocValuesFormatWithSkipInterval(1)
}
