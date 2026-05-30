// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene's org.apache.lucene.index.TestExceedMaxTermLength
// (releases/lucene/10.4.0, commit 9983b7c).
//
// GOC-4155, Sprint 55 (option c: full roundtrip where it compiles).
//
// The upstream test verifies that IndexWriter throws an IllegalArgumentException
// mentioning "immense term", the field name, IndexWriter.MAX_TERM_LENGTH, and the
// original "bytes can be at most ... in length; got" message when a term longer
// than MAX_TERM_LENGTH is indexed, via both the token-stream and binary-value
// paths.
//
// GAP: that roundtrip does not compile/run in Gocene yet.
//   - index has no IndexWriter.MAX_TERM_LENGTH constant.
//   - indexing_chain.invertTokenStream is a hard-failing stub (the analysis
//     TokenStream bridge is unported), so the token-stream path cannot reach
//     term-length enforcement.
//   - neither indexing_chain.invertTerm nor DocumentsWriterPerThread.ProcessDocument
//     enforce a maximum term length, so the binary path never raises the
//     "immense term" error.
//   - there is no MockAnalyzer nor TestUtil.randomSimpleString/randomBinaryTerm.
//
// The two test functions below preserve the upstream structure and skip with an
// explicit reason. They must be filled in once MAX_TERM_LENGTH enforcement and
// the analysis bridge land.

package index_test

import "testing"

// maxTermLengthGap is the shared skip reason for the immense-term roundtrip.
const maxTermLengthGap = "GOC-4155 GAP: IndexWriter.MAX_TERM_LENGTH and immense-term " +
	"enforcement unported; token-stream inversion stubbed. See file header."

// TestExceedMaxTermLength_TokenStream ports TestExceedMaxTermLength#testTokenStream.
//
// Upstream indexes a tokenized field whose value is a single token longer than
// MAX_TERM_LENGTH and asserts the IllegalArgumentException message mentions
// "immense term", the max length, the field name, and "bytes can be at most
// ... in length; got".
func TestExceedMaxTermLength_TokenStream(t *testing.T) {
	t.Fatal(maxTermLengthGap)
}

// TestExceedMaxTermLength_BinaryValue ports TestExceedMaxTermLength#testBinaryValue.
//
// Upstream indexes a non-tokenized binary field whose value exceeds
// MAX_TERM_LENGTH and asserts the same IllegalArgumentException message.
func TestExceedMaxTermLength_BinaryValue(t *testing.T) {
	t.Fatal(maxTermLengthGap)
}
