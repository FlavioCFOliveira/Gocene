// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/TestAssertions.java
//
// Deviation: testTokenStreams validates Java-specific assertion enforcement.
// The Java test verifies that the JVM's -ea flag is active and that the
// TokenStream constructor uses an assert statement to detect non-final
// incrementToken() overrides (TestTokenStream3). Go has no equivalent to
// Java assert statements or method finality; the test is registered as a
// stub that documents this incompatibility.

package gocene

import "testing"

// TestAssertions_TokenStreams mirrors testTokenStreams (Lucene 10.4.0).
// The original test verifies that Java assertions are enabled (-ea) and
// that the TokenStream constructor enforces a finality contract on
// incrementToken() via an AssertionError. Go has no equivalent: there
// are no assert statements, no -ea flag, and no method finality concept.
func TestAssertions_TokenStreams(t *testing.T) {
	t.Fatal("Java-specific: tests JVM assertion (-ea) enforcement and final-method detection in TokenStream constructor — no Go equivalent")
}
