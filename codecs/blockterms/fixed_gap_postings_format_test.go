// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blockterms_test

import "testing"

// TestFixedGapPostingsFormat mirrors the Java class
// org.apache.lucene.codecs.blockterms.TestFixedGapPostingsFormat
// (Lucene 10.4.0).
//
// The Java class extends BasePostingsFormatTestCase and registers
// LuceneFixedGap (a BlockTerms postings format with fixed-gap terms index)
// as the codec under test. No @Test methods are declared; the test suite
// comes entirely from the superclass framework.
//
// The test is skipped because LuceneFixedGap (and the underlying
// FixedGapTermsIndexWriter / BlockTermsWriter) are not yet ported to the
// Gocene blockterms package.
func TestFixedGapPostingsFormat(t *testing.T) {
	t.Fatal(
		"LuceneFixedGap and the underlying FixedGapTermsIndexWriter / " +
			"BlockTermsWriter write path are not yet ported to Gocene; " +
			"test deferred until those components land",
	)
}
