// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sharedterms_test

import "testing"

// TestSTUniformSplitPostingFormat mirrors the Java class
// org.apache.lucene.codecs.uniformsplit.sharedterms.TestSTUniformSplitPostingFormat
// (Lucene 10.4.0).
//
// The Java class extends TestUniformSplitPostingFormat (which itself extends
// BasePostingsFormatTestCase) and overrides only getPostingsFormat() to
// substitute STUniformSplitPostingsFormat (with optional ROT13 encoding).
// No individual @Test methods are declared; the test suite comes from the
// superclass framework.
//
// The test is skipped because neither STUniformSplitPostingsFormat,
// STUniformSplitRot13PostingsFormat, nor UniformSplitTermsWriter are yet
// ported to the Gocene sharedterms package.
func TestSTUniformSplitPostingFormat(t *testing.T) {
	t.Fatal(
		"STUniformSplitPostingsFormat, STUniformSplitRot13PostingsFormat, and " +
			"UniformSplitTermsWriter are not yet ported to Gocene; test deferred " +
			"until those components land",
	)
}
