// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blockterms_test

import "testing"

// TestVarGapDocFreqIntervalPostingsFormat mirrors the Java class
// org.apache.lucene.codecs.blockterms.TestVarGapDocFreqIntervalPostingsFormat
// (Lucene 10.4.0).
//
// The Java class extends BasePostingsFormatTestCase and registers
// LuceneVarGapFixedInterval (a BlockTerms postings format with
// variable-gap/doc-freq-interval terms index) as the codec under test.
// No @Test methods are declared; the suite comes from the superclass.
//
// The test is skipped because LuceneVarGapFixedInterval and the
// underlying VariableGapTermsIndexWriter / BlockTermsWriter are not
// yet ported to the Gocene blockterms package.
func TestVarGapDocFreqIntervalPostingsFormat(t *testing.T) {
	t.Fatal(
		"LuceneVarGapFixedInterval and the underlying " +
			"VariableGapTermsIndexWriter / BlockTermsWriter write path are " +
			"not yet ported to Gocene; test deferred until those components land",
	)
}
