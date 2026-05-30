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

// GOC-4134: Port of perfield/TestPerFieldPostingsFormat2.
//
// Source: lucene/core/src/test/org/apache/lucene/codecs/perfield/
//         TestPerFieldPostingsFormat2.java (Lucene 10.4.0, tag
//         releases/lucene/10.4.0, commit 9983b7c).
//
// Scope. The Java parent exercises the *end-to-end* per-field
// postings pipeline: a MockCodec extends AssertingCodec and routes
// the "id" field to DirectPostingsFormat while leaving every other
// field on the default PostingsFormat. Documents are pushed through
// IndexWriter (with LogDocMergePolicy + setNoCFSRatio(0.0)) using
// MockAnalyzer / RandomIndexWriter; readers are opened with
// DirectoryReader; queries flow through IndexSearcher + TermQuery;
// merges are forced via writer.forceMerge(1) and the test asserts
// merge call counts captured by a wrapping FieldsConsumer that
// intercepts FieldsConsumer.merge(MergeState, NormsProducer).
//
// Sprint 55 gap audit (option c, skip-with-symbols).
// Each of the following hard dependencies is missing from the
// Gocene tree as of this sprint, so the port is emitted as a
// documented Skip stub:
//
//   - tests/asserting: AssertingCodec is not ported. There is no
//     Go equivalent of org.apache.lucene.tests.codecs.asserting.
//     AssertingCodec, so MockCodec cannot be expressed as a thin
//     override of getPostingsFormatForField(String).
//   - codecs/memory: DirectPostingsFormat is not ported. The
//     in-memory postings format that the Java MockCodec routes
//     "id" / "date" through has no Gocene counterpart.
//   - tests/blockterms: LuceneVarGapFixedInterval is not ported
//     (testSameCodecDifferentParams asserts that two *instances*
//     of the same PostingsFormat with different fixed-interval
//     parameters can co-exist on different fields).
//   - tests/analysis: MockAnalyzer is not ported. The MockTokenizer
//     pipeline that Lucene's tests rely on (whitespace splitter
//     + lowercasing + simulate-payloads switches) is not yet
//     available under analysis_test/.
//   - tests/index: RandomIndexWriter is not ported. The Java
//     testSameCodec*Postings tests drive index writes through
//     RandomIndexWriter (which randomises flush/merge cadence
//     and adds checkindex side-effects on close).
//   - document.IntPoint: IntPoint exists in Gocene (document/
//     int_field.go ships an integer Point variant) but the Java
//     testMergeCalledOnTwoFormats relies on the *exact* IntPoint
//     wire format that Lucene 10.4.0 emits, which is asserted by
//     ports under document/ rather than re-asserted here.
//   - PerFieldPostingsFormat <-> IndexWriter wiring. The Gocene
//     PerFieldPostingsFormat (codecs/per_field_postings_format.go)
//     does not yet plug into the IndexWriter codec dispatch path
//     used by AddDocument / Commit / ForceMerge; see backlog
//     GOC-2691 (PostingsFormat wiring for blocktree) for the
//     adjacent gap.
//   - FieldsConsumer.merge(MergeState, NormsProducer). The
//     MergeRecordingPostingsFormatWrapper in the Java source
//     intercepts the merge entry-point on FieldsConsumer to count
//     calls and record FieldInfo.name values; the Gocene
//     FieldsConsumer surface does not yet expose a merge hook
//     suitable for that interception in this sprint.
//
// All five Java @Test methods are surfaced below as Go Test*
// stubs so `go test -v ./codecs/...` enumerates the activation
// budget. Symbols referenced from the future activated bodies are
// kept live via blank assignments so a follow-up patch surfaces as
// body fills rather than import churn.

// TestPerFieldPostingsFormat2_MergeUnusedPerFieldCodec ports
// `testMergeUnusedPerFieldCodec` (Java lines 106-126): writes
// three commits ("aaa", "ccc"+id, "bbb") through a MockCodec that
// only routes "id" to DirectPostingsFormat, then forceMerges to a
// single segment and asserts maxDoc == 30.
func TestPerFieldPostingsFormat2_MergeUnusedPerFieldCodec(t *testing.T) {
	t.Fatal("blocked by AssertingCodec/DirectPostingsFormat/MockAnalyzer/" +
		"PerFieldPostingsFormat<->IndexWriter wiring; remove this Skip when fixed")

	// Reserved symbols: keep types live so the activation patch
	// surfaces as body fills rather than new imports.
	_ = codecs.NewLucene104PostingsFormat
	_ = index.IndexOptionsDocsAndFreqsAndPositions
	_ = store.NewByteBuffersDirectory
}

// TestPerFieldPostingsFormat2_ChangeCodecAndMerge ports
// `testChangeCodecAndMerge` (Java lines 134-204): opens and
// re-opens the writer in APPEND mode swapping in a fresh MockCodec
// instance, asserts term-query counts against three round-trips of
// commits, then forceMerges and re-asserts.
func TestPerFieldPostingsFormat2_ChangeCodecAndMerge(t *testing.T) {
	t.Fatal("blocked by AssertingCodec/DirectPostingsFormat/MockAnalyzer/" +
		"PerFieldPostingsFormat<->IndexWriter wiring + DirectoryReader/" +
		"IndexSearcher/TermQuery round-trip; remove this Skip when fixed")

	_ = codecs.NewLucene104PostingsFormat
}

// TestPerFieldPostingsFormat2_StressPerFieldCodec ports
// `testStressPerFieldCodec` (Java lines 234-264): drives 1+ rounds
// of 97 documents with 30-60 randomly-named fields each, asserting
// the cumulative maxDoc after each round survives the optional
// forceMerge(1).
func TestPerFieldPostingsFormat2_StressPerFieldCodec(t *testing.T) {
	t.Fatal("blocked by MockAnalyzer/RandomCodec/random()/TestUtil.nextInt + " +
		"PerFieldPostingsFormat<->IndexWriter wiring; remove this Skip when fixed")
}

// TestPerFieldPostingsFormat2_SameCodecDifferentInstance ports
// `testSameCodecDifferentInstance` (Java lines 266-281): two
// *separate* DirectPostingsFormat instances routed to "id" and
// "date" through AssertingCodec, driven by RandomIndexWriter +
// term-vectors checkindex cross-check on close.
func TestPerFieldPostingsFormat2_SameCodecDifferentInstance(t *testing.T) {
	t.Fatal("blocked by AssertingCodec/DirectPostingsFormat/RandomIndexWriter/" +
		"MockAnalyzer + checkindex cross-check; remove this Skip when fixed")
}

// TestPerFieldPostingsFormat2_SameCodecDifferentParams ports
// `testSameCodecDifferentParams` (Java lines 283-298): two
// LuceneVarGapFixedInterval instances with intervals 1 and 2 on
// "id" and "date" respectively, again driven by
// RandomIndexWriter + checkindex on close.
func TestPerFieldPostingsFormat2_SameCodecDifferentParams(t *testing.T) {
	t.Fatal("blocked by AssertingCodec/LuceneVarGapFixedInterval/" +
		"RandomIndexWriter/MockAnalyzer + checkindex cross-check; " +
		"remove this Skip when fixed")
}

// TestPerFieldPostingsFormat2_MergeCalledOnTwoFormats ports
// `testMergeCalledOnTwoFormats` (Java lines 325-383): two
// MergeRecordingPostingsFormatWrapper instances intercept
// FieldsConsumer.merge(MergeState, NormsProducer) and assert
// (a) the merge call count per format and (b) the FieldInfo.name
// list observed, including the negative assertion that IntPoint
// fields do not surface as posted fields.
func TestPerFieldPostingsFormat2_MergeCalledOnTwoFormats(t *testing.T) {
	t.Fatal("blocked by AssertingCodec/PostingsFormat(String)-named-ctor/" +
		"FieldsConsumer.merge(MergeState, NormsProducer) hook/" +
		"IntPoint exact wire format + PerFieldPostingsFormat<->IndexWriter " +
		"wiring; remove this Skip when fixed")
}
