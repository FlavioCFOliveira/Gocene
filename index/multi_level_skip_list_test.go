// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import "testing"

// TestMultiLevelSkipList is a faithful port of the Apache Lucene 10.4.0 test
// org.apache.lucene.index.TestMultiLevelSkipList
// (core/src/test/org/apache/lucene/index/TestMultiLevelSkipList.java).
//
// The Java testcase verifies that multi-level skipping is actually exercised
// to reduce I/O while advancing through posting lists. It does so by:
//
//  1. Wrapping a ByteBuffersDirectory in a CountingDirectory whose .frq input
//     stream (CountingStream) increments a byte counter on every readByte /
//     readBytes call.
//  2. Indexing 5000 single-token documents whose token "a" carries a 1-byte
//     payload equal to the document ordinal, using a PayloadAnalyzer built on
//     MockTokenizer plus a PayloadFilter, force-merged to a single segment.
//  3. Calling PostingsEnum.advance to four targets (14, 17, 287, 4800) and,
//     for each, asserting (a) the bytes read so far stays under a per-target
//     budget that only holds when >1 skip level is used, (b) docID equals the
//     target, (c) freq is 1, and (d) the payload byte equals (byte) target.
//
// Port status: SKIPPED. A behaviour-preserving Go port requires the following
// Gocene capabilities, none of which exist in the tree at the time of this
// task (GOC-4153, Sprint 58):
//
//   - A writable indexing pipeline: index.IndexWriter / IndexWriterConfig with
//     a configurable Codec and LogMergePolicy, plus forceMerge(1).
//   - A readable side: DirectoryReader.open producing a LeafReader exposing
//     postings(Term, PostingsEnum.ALL) with working advance/freq/nextPosition
//     and getPayload.
//   - The analysis fixtures the Java test relies on: a MockTokenizer
//     (WHITESPACE) and a TokenFilter able to attach a PayloadAttribute, wired
//     through an Analyzer.
//   - An instrumentable Directory equivalent to MockDirectoryWrapper that can
//     intercept openInput for ".frq" files and count bytes read, which is the
//     entire point of the test (the I/O-budget assertions are meaningless
//     without it).
//
// This file is intentionally a 1:1 placeholder for the single Java @Test
// method (testSimpleSkip). It is kept in the index package so it is picked up
// and unskipped automatically once the indexing/search stack lands, rather
// than being silently forgotten. The CountingDirectory / CountingStream /
// PayloadAnalyzer / PayloadFilter helpers from the Java source are documented
// above but not yet stubbed, since their shape depends on the final Gocene
// store and analysis APIs.
func TestMultiLevelSkipList(t *testing.T) {
	t.Skip("GOC-4153: faithful port deferred — needs index.IndexWriter, " +
		"DirectoryReader/LeafReader postings with payloads, MockTokenizer/" +
		"PayloadFilter analysis fixtures, and a byte-counting Directory " +
		"(MockDirectoryWrapper equivalent). See file-level comment.")
}
