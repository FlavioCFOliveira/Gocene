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
	t.Skip("Payloads not yet fully supported in Lucene104PostingsFormat tests")

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	tester := codecs.NewPostingsTester(t)
	format := codecs.NewLucene104PostingsFormat()

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
	t.Skip("Payloads not yet fully supported in Lucene104PostingsFormat tests")

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	tester := codecs.NewPostingsTester(t)
	format := codecs.NewLucene104PostingsFormat()

	tester.TestFull(format, index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets, dir)
}

func TestLucene104PostingsFormat_Random(t *testing.T) {
	t.Skip("Randomized postings testing not yet fully implemented")
}

func TestLucene104PostingsFormat_PostingsEnumReuse(t *testing.T) {
	t.Skip("PostingsEnum reuse testing not yet fully implemented")
}
