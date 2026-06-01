// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/TestAssertions.java
//
// Deviation: testTokenStreams validates Java-specific assertion enforcement.
// The Java test verifies that the JVM's -ea flag is active and that the
// TokenStream constructor uses an assert statement to detect non-final
// incrementToken() overrides (TestTokenStream3). Go has no equivalent:
// there are no assert statements, no -ea flag, and no method finality
// concept. The test is retained as a documentation anchor and passes
// vacuously.

package gocene

import "testing"

// TestAssertions_TokenStreams mirrors testTokenStreams (Lucene 10.4.0).
//
// The Java original verifies two JVM-specific behaviours:
//  1. Java assertions (-ea) are enabled in the test JVM.
//  2. The TokenStream constructor detects non-final incrementToken()
//     overrides at construction time via an assert statement
//     (AssertionError is expected for TestTokenStream3).
//
// Neither behaviour exists in Go:
//   - Go has no runtime assert flag analogous to -ea.
//   - Go interfaces carry no finality restriction on implementing methods.
//
// The test passes vacuously; the Gocene equivalent of the finality contract
// is enforced by convention and static analysis (go vet / golangci-lint)
// rather than by a runtime check.
func TestAssertions_TokenStreams(t *testing.T) {
	// Nothing to verify in Go — see file-level deviation note.
}
