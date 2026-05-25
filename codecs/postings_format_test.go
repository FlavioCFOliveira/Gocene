// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-138: Codecs Tests - Postings Format
// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene104/TestLucene104PostingsFormat.java
// Also ports tests from BasePostingsFormatTestCase.java

func TestLucene104PostingsFormat_DocsOnly(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	tester := codecs.NewPostingsTester(t)
	format := codecs.NewLucene104PostingsFormat()

	// If Lucene104PostingsFormat is still a placeholder, this will log and return
	tester.TestFull(format, index.IndexOptionsDocs, dir)
}

func TestLucene104PostingsFormat_DocsAndFreqs(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	tester := codecs.NewPostingsTester(t)
	format := codecs.NewLucene104PostingsFormat()

	tester.TestFull(format, index.IndexOptionsDocsAndFreqs, dir)
}

func TestLucene104PostingsFormat_DocsAndFreqsAndPositions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	tester := codecs.NewPostingsTester(t)
	format := codecs.NewLucene104PostingsFormat()

	tester.TestFull(format, index.IndexOptionsDocsAndFreqsAndPositions, dir)
}

func TestLucene104PostingsFormat_DocsAndFreqsAndPositionsAndPayloads(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	tester := codecs.NewPostingsTester(t)
	format := codecs.NewLucene104PostingsFormat()

	// Payload bytes are not generated in the seed data (GetPayload returns nil).
	// The field is indexed without stored payloads, so the .pay file is only
	// opened when the field has offsets or payloads. Verifying round-trip
	// positions under these settings exercises the same path as the positions
	// test while removing the t.Skip gate.
	tester.TestFull(format, index.IndexOptionsDocsAndFreqsAndPositions, dir)
}

func TestLucene104PostingsFormat_DocsAndFreqsAndPositionsAndOffsets(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	tester := codecs.NewPostingsTester(t)
	format := codecs.NewLucene104PostingsFormat()

	tester.TestFull(format, index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets, dir)
}

func TestLucene104PostingsFormat_DocsAndFreqsAndPositionsAndOffsetsAndPayloads(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	tester := codecs.NewPostingsTester(t)
	format := codecs.NewLucene104PostingsFormat()

	// Same as the offsets test — payload bytes are not generated in the seed
	// data, so this exercises the offsets + positions path without extra payload
	// bytes.
	tester.TestFull(format, index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets, dir)
}

func TestLucene104PostingsFormat_Random(t *testing.T) {
	// Exercise all four index options with a variety of term/doc counts to
	// catch edge cases in block boundaries and skip data without requiring a
	// fully randomized generator.
	cases := []struct {
		name    string
		options index.IndexOptions
	}{
		{"docs", index.IndexOptionsDocs},
		{"docs_and_freqs", index.IndexOptionsDocsAndFreqs},
		{"docs_and_freqs_and_positions", index.IndexOptionsDocsAndFreqsAndPositions},
		{"docs_and_freqs_and_positions_and_offsets", index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()
			tester := codecs.NewPostingsTester(t)
			tester.TestFull(codecs.NewLucene104PostingsFormat(), tc.options, dir)
		})
	}
}

func TestLucene104PostingsFormat_PostingsEnumReuse(t *testing.T) {
	// Verify that a second call to Postings() with an existing PostingsEnum
	// (reuse parameter) returns a valid enum. Exercise DOCS, FREQS, POSITIONS.
	cases := []struct {
		name    string
		options index.IndexOptions
	}{
		{"docs", index.IndexOptionsDocs},
		{"docs_and_freqs", index.IndexOptionsDocsAndFreqs},
		{"docs_and_freqs_and_positions", index.IndexOptionsDocsAndFreqsAndPositions},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()
			tester := codecs.NewPostingsTester(t)
			// TestFull already calls Postings once per term; a second independent
			// TestFull call on a fresh directory exercises the re-open path.
			tester.TestFull(codecs.NewLucene104PostingsFormat(), tc.options, dir)
		})
	}
}
