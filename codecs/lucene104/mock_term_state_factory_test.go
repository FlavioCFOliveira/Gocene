// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene104_test

import "testing"

// TestMockTermStateFactory_Placeholder mirrors the Java class
// org.apache.lucene.codecs.lucene90.tests.MockTermStateFactory
// (Lucene 10.4.0).
//
// The Java class is a test utility, not a runnable test. It exposes a single
// factory method:
//
//	public static IntBlockTermState create() { return new IntBlockTermState(); }
//
// IntBlockTermState is an inner class of Lucene104PostingsFormat. It is not
// yet ported to the Gocene codecs/lucene104 package; the full postings reader
// and writer byte-format ports (including IntBlockTermState) are deferred to
// a follow-up deep-port sprint.
//
// Once IntBlockTermState lands, the factory below should be expressed as a
// package-level test helper function:
//
//	func createMockTermState() *lucene104.IntBlockTermState {
//	    return lucene104.NewIntBlockTermState()
//	}
func TestMockTermStateFactory_Placeholder(t *testing.T) {
	t.Fatal(
		"IntBlockTermState is not yet ported to codecs/lucene104; " +
			"MockTermStateFactory will be implemented when that type lands",
	)
}
