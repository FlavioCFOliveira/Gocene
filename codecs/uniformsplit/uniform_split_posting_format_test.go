// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit_test

import "testing"

// TestUniformSplitPostingFormat mirrors the Java class
// org.apache.lucene.codecs.uniformsplit.TestUniformSplitPostingFormat
// (Lucene 10.4.0).
//
// The Java class extends BasePostingsFormatTestCase, which contributes its own
// large suite of randomized tests. No individual @Test methods are declared
// in the Java source; the only custom logic is:
//
//  1. Construct UniformSplitPostingsFormat (optionally wrapping it in the
//     ROT13 test encoder UniformSplitRot13PostingsFormat).
//  2. In an @After hook, assert that the encoder / decoder were called at
//     least once.
//
// Both UniformSplitTermsWriter (which defines DEFAULT_TARGET_NUM_BLOCK_LINES,
// DEFAULT_DELTA_NUM_LINES) and UniformSplitRot13PostingsFormat are not yet
// ported to the Gocene uniformsplit package.  The test is therefore skipped
// until those components land.
func TestUniformSplitPostingFormat(t *testing.T) {
	t.Skip(
		"UniformSplitTermsWriter and UniformSplitRot13PostingsFormat are not " +
			"yet ported to Gocene; test deferred until those components land",
	)
}
