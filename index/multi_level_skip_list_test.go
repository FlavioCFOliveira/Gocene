// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestMultiLevelSkipList is a simplified unit test ported from the Apache
// Lucene 10.4.0 test org.apache.lucene.index.TestMultiLevelSkipList
// (core/src/test/org/apache/lucene/index/TestMultiLevelSkipList.java).
//
// The Java original exercises the full indexing pipeline with payloads and
// I/O-counting directories. This test directly verifies the
// MultiLevelSkipListWriter + MultiLevelSkipListReader round-trip:
//
//  1. Create a writer, buffer skip entries with a constant doc delta,
//     and flush them via WriteSkip.
//  2. Create a reader over the flushed data and verify that SkipTo
//     positions the cursor at the expected (doc, numSkipped) pair for
//     several targets.
//
// The skip entries all carry a doc delta of `skipInterval` (4), so the
// skip-doc sequence at level 0 is: 4, 8, 12, 16, ...

func TestMultiLevelSkipList(t *testing.T) {
	const (
		skipInterval   = 4
		skipMultiplier = 2
		maxSkipLevels  = 10
		df             = 200
	)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("skipdata", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}

	// Writer-side hook: write the doc delta as a single byte per skip entry.
	// The delta matches the skip interval of the level so that the skip-list
	// shape mirrors a real Lucene skip list.
	w := codecs.NewMultiLevelSkipListWriter(skipInterval, skipMultiplier, maxSkipLevels, df,
		func(level int, buf *store.ByteArrayDataOutput) error {
			delta := skipInterval
			for i := 0; i < level; i++ {
				delta *= skipMultiplier
			}
			return buf.WriteByte(byte(delta))
		},
	)
	w.Init()

	// Buffer skip entries at skipInterval boundaries for df 1..200.
	for i := 1; i <= df; i++ {
		if i%skipInterval != 0 {
			continue
		}
		if err := w.BufferSkip(i); err != nil {
			t.Fatalf("BufferSkip(%d): %v", i, err)
		}
	}

	skipPointer, err := w.WriteSkip(out)
	if err != nil {
		t.Fatalf("WriteSkip: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}

	// Reader-side hook: read a byte as the doc delta; return (0, nil) on
	// end-of-stream so the skip walk terminates cleanly.
	readSkipData := func(level int, skipStream store.IndexInput) (int, error) {
		b, err := skipStream.ReadByte()
		if err != nil {
			return 0, nil
		}
		return int(b), nil
	}

	in, err := dir.OpenInput("skipdata", store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	r := codecs.NewMultiLevelSkipListReader(in, maxSkipLevels, skipInterval, skipMultiplier, readSkipData)
	if err := r.Init(skipPointer, df); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Test cases: (target, expectedDoc, minExpectedSkipped)
	//   - target < skipInterval: no skip entry qualifies.
	//   - target just past a skip entry: that entry is the last match.
	//   - target far into the posting list: multiple levels are exercised.
	//
	// The return value of SkipTo follows Lucene's contract:
	// numSkipped[0] - skipInterval[0] - 1, which for this data equals
	// (last accepted doc) - 1.
	cases := []struct {
		target     int
		wantDoc    int
		minSkipped int
	}{
		{target: 1, wantDoc: 0, minSkipped: -1},        // before first skip entry
		{target: 4, wantDoc: 0, minSkipped: -1},        // at the first skip entry → lastDoc is 0 before reading it
		{target: 5, wantDoc: 4, minSkipped: 3},         // after first skip entry
		{target: 17, wantDoc: 16, minSkipped: 15},      // after 4th skip entry
		{target: 50, wantDoc: 48, minSkipped: 47},      // after 12th skip entry
		{target: 100, wantDoc: 96, minSkipped: 95},     // between level-0 entries 96 and 100
		{target: 150, wantDoc: 148, minSkipped: 147},   // deep in the list
	}
	for _, tc := range cases {
		// Reset the reader for each target.
		r2 := codecs.NewMultiLevelSkipListReader(in.Clone(), maxSkipLevels, skipInterval, skipMultiplier, readSkipData)
		if err := r2.Init(skipPointer, df); err != nil {
			t.Fatalf("Init: %v", err)
		}
		numSkipped, err := r2.SkipTo(tc.target)
		if err != nil {
			t.Fatalf("SkipTo(%d): %v", tc.target, err)
		}
		gotDoc := r2.GetDoc()

		if gotDoc != tc.wantDoc {
			t.Errorf("SkipTo(%d): GetDoc = %d, want %d", tc.target, gotDoc, tc.wantDoc)
		}
		if numSkipped < tc.minSkipped {
			t.Errorf("SkipTo(%d): numSkipped = %d, want >= %d", tc.target, numSkipped, tc.minSkipped)
		}
	}
}
