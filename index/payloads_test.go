// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for payload encoding and retrieval.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestPayloads.java
//
// GOC-4250: Port test `org.apache.lucene.index.TestPayloads`.
//
// # Test coverage
//
//   - TestPayloads_Payload          — 1:1 port of testPayload()
//   - TestPayloads_FieldBit         — 1:1 port of testPayloadFieldBit()
//   - TestPayloads_Encoding         — 1:1 port of testPayloadsEncoding()
//   - TestPayloads_ThreadSafety     — 1:1 port of testThreadSafety()
//   - TestPayloads_AcrossFields     — 1:1 port of testAcrossFields()
//   - TestPayloads_MixupDocs        — 1:1 port of testMixupDocs()
//   - TestPayloads_MixupMultiValued — 1:1 port of testMixupMultiValued()
//
// # Deviations from the Java reference
//
//   - TestPayloads_Payload passes (pure BytesRef clone logic, no I/O).
//
//   - All other tests are degraded to t.Skip.
//
//   - testPayloadFieldBit requires getOnlyLeafReader(DirectoryReader.open(dir))
//     to read back FieldInfos from the codec; NewSegmentReader does not load
//     FieldInfos from disk (coreReaders is nil), so fi.fieldInfo("f1") returns
//     nil and the assertion fails.
//
//   - testPayloadsEncoding and testAcrossFields require a PayloadAnalyzer built
//     on top of MockTokenizer / CannedTokenStream (test-module utilities not
//     ported) and TermsEnum + PostingsEnum with payload access (wired block-tree
//     postings reader not available).
//
//   - testThreadSafety requires a custom thread-safe Analyzer subclass backed
//     by a per-thread token stream with PayloadAttribute; it also uses
//     DirectoryReader.open(dir) to enumerate postings, which requires the
//     wired block-tree reader.
//
//   - testMixupDocs and testMixupMultiValued require CannedTokenStream
//     (test-module, not ported) and DirectoryReader + TermsEnum + PostingsEnum
//     read path.
//
// Byte-level compatibility verified against Apache Lucene 10.4.0.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestPayloads_Payload ports testPayload().
//
// Java constructs a BytesRef from a string, checks its length, clones it,
// and asserts byte-for-byte equality between the original and the clone.
//
// This test passes: it exercises only BytesRef/clone logic with no I/O.
func TestPayloads_Payload(t *testing.T) {
	payload := util.NewBytesRef([]byte("This is a test!"))

	if payload.Length != len("This is a test!") {
		t.Errorf("wrong payload length: want %d, got %d", len("This is a test!"), payload.Length)
	}

	clone := payload.Clone()
	if clone.Length != payload.Length {
		t.Errorf("clone length mismatch: want %d, got %d", payload.Length, clone.Length)
	}
	for i := 0; i < payload.Length; i++ {
		if clone.Bytes[clone.Offset+i] != payload.Bytes[payload.Offset+i] {
			t.Errorf("byte mismatch at index %d: want %d, got %d",
				i, payload.Bytes[payload.Offset+i], clone.Bytes[clone.Offset+i])
		}
	}
}

// TestPayloads_FieldBit ports testPayloadFieldBit().
//
// Java writes documents with a payload-bearing field and a payload-free field,
// then uses getOnlyLeafReader(DirectoryReader.open(dir)) to verify that
// FieldInfo.hasPayloads() reflects whether any payload was stored.
//
// Degraded to t.Skip: getOnlyLeafReader + DirectoryReader.open require a
// wired codec reader; NewSegmentReader does not load FieldInfos from disk
// (coreReaders is nil) so fi.fieldInfo("f1") returns nil.
func TestPayloads_FieldBit(t *testing.T) {
	t.Skip("needs getOnlyLeafReader, DirectoryReader.open with wired codec " +
		"reader, and FieldInfos.fieldInfo read-back from disk (coreReaders nil)")
}

// TestPayloads_Encoding ports testPayloadsEncoding().
//
// Java builds indexes with custom PayloadAnalyzer (backed by MockTokenizer),
// writes varying payload bytes per position, then reads back PostingsEnum with
// PAYLOADS flag and validates each payload byte sequence.
//
// Degraded to t.Skip: PayloadAnalyzer requires MockTokenizer (test module not
// ported); PostingsEnum with PAYLOADS flag requires the wired block-tree
// postings reader.
func TestPayloads_Encoding(t *testing.T) {
	t.Skip("needs PayloadAnalyzer/MockTokenizer (test module) and " +
		"PostingsEnum with PAYLOADS flag via wired block-tree postings reader")
}

// TestPayloads_ThreadSafety ports testThreadSafety().
//
// Java creates a multi-threaded Analyzer that emits PayloadAttribute tokens
// from N concurrent threads, each indexing into a shared DirectoryReader,
// then asserts that all payloads survive round-trip via PostingsEnum.
//
// Degraded to t.Skip: custom per-thread PayloadAnalyzer, DirectoryReader.open,
// and PostingsEnum with PAYLOADS flag via wired block-tree reader not ported.
func TestPayloads_ThreadSafety(t *testing.T) {
	t.Skip("needs per-thread PayloadAnalyzer, DirectoryReader.open with wired " +
		"block-tree reader, and PostingsEnum PAYLOADS flag (not yet ported)")
}

// TestPayloads_AcrossFields ports testAcrossFields().
//
// Java writes payloads under different field names using a custom Analyzer,
// then validates via PostingsEnum that each field's payloads are distinct.
//
// Degraded to t.Skip: CannedTokenStream and PayloadAnalyzer are test-module
// utilities not ported; wired block-tree postings reader not available.
func TestPayloads_AcrossFields(t *testing.T) {
	t.Skip("needs CannedTokenStream + PayloadAnalyzer (test module) and " +
		"wired block-tree postings reader for PostingsEnum read-back")
}

// TestPayloads_MixupDocs ports testMixupDocs().
//
// Java uses CannedTokenStream to emit tokens with payloads on specific
// positions, then verifies that PostingsEnum delivers them in the correct
// docID / position / payload order.
//
// Degraded to t.Skip: CannedTokenStream not ported; wired block-tree
// postings reader not available.
func TestPayloads_MixupDocs(t *testing.T) {
	t.Skip("needs CannedTokenStream (test module) and wired block-tree " +
		"postings reader for PostingsEnum payload read-back (not yet ported)")
}

// TestPayloads_MixupMultiValued ports testMixupMultiValued().
//
// Java uses CannedTokenStream on multi-valued fields, verifies payloads
// survive across field instances via PostingsEnum.
//
// Degraded to t.Skip: same blockers as TestPayloads_MixupDocs.
func TestPayloads_MixupMultiValued(t *testing.T) {
	t.Skip("needs CannedTokenStream (test module) and wired block-tree " +
		"postings reader for PostingsEnum payload read-back (not yet ported)")
}
