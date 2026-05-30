// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// TestDuelingCodecs ports org.apache.lucene.index.TestDuelingCodecs.
//
// It builds two random indexes from LineFileDocs (atLeast(20) documents each)
// with different codecs (SimpleText vs a RandomCodec) sharing one seed, then
// asserts the readers are equivalent via assertReaderEquals. testCrazyReaderEquals
// repeats the check over wrapped readers.
//
// Sprint 55 option c stub: depends on RandomIndexWriter, RandomCodec, LineFileDocs,
// MockAnalyzer and assertReaderEquals, none of which are ported yet.
func TestDuelingCodecsEquals(t *testing.T) {
	t.Fatal("pending test infra: RandomIndexWriter, RandomCodec, LineFileDocs, MockAnalyzer, assertReaderEquals")
}

// TestDuelingCodecsCrazyReaderEquals ports TestDuelingCodecs.testCrazyReaderEquals.
//
// Sprint 55 option c stub: see TestDuelingCodecsEquals.
func TestDuelingCodecsCrazyReaderEquals(t *testing.T) {
	t.Fatal("pending test infra: RandomIndexWriter, RandomCodec, LineFileDocs, MockAnalyzer, wrapReader, assertReaderEquals")
}
