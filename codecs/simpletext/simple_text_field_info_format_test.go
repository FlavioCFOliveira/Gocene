// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext_test

import "testing"

// TestSimpleTextFieldInfoFormat mirrors the Java class
// org.apache.lucene.codecs.simpletext.TestSimpleTextFieldInfoFormat
// (Lucene 10.4.0).
//
// The Java class extends BaseFieldInfoFormatTestCase and registers
// SimpleTextCodec as the codec under test. No @Test methods are declared;
// the test suite is inherited from the superclass framework.
//
// The test is skipped because SimpleTextFieldInfoFormat is not yet ported to
// the Gocene simpletext package and BaseFieldInfoFormatTestCase has no Go
// equivalent.
func TestSimpleTextFieldInfoFormat(t *testing.T) {
	t.Skip(
		"SimpleTextFieldInfoFormat is not yet ported to Gocene and " +
			"BaseFieldInfoFormatTestCase has no Go equivalent; " +
			"test deferred until those components land",
	)
}
