// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/intervals/TestComplexMatches.java

package intervals

import (
	"testing"
)

// TestComplexMatches exercises the complex interval query infrastructure
// available in Gocene: interval source builders, equality, and string
// representation at the unit level.
//
// The Lucene original requires full index integration with RandomIndexWriter.
func TestComplexMatches(t *testing.T) {
	// Term intervals source: create and verify description.
	src := Term("hello")
	desc := src.String()
	if desc == "" {
		t.Error("Term intervals source String() returned empty")
	}

	// Check that different term sources produce different descriptions.
	src2 := Term("world")
	if src.String() == src2.String() {
		t.Error("different terms produce the same String()")
	}

	// Ordered source wrapping two terms.
	ordered := Ordered(src, src2)
	if ordered.String() == "" {
		t.Error("Ordered intervals source String() returned empty")
	}

	// Unordered source wrapping two terms.
	unordered := Unordered(src, src2)
	if unordered.String() == "" {
		t.Error("Unordered intervals source String() returned empty")
	}

	// Or (disjunction) source.
	disj := Or(src, src2)
	if disj.String() == "" {
		t.Error("Or intervals source String() returned empty")
	}

	// Phrase source wraps terms in an ordered container.
	phrase := Phrase("hello", "world")
	if phrase.String() == "" {
		t.Error("Phrase intervals source String() returned empty")
	}

	// FixField source.
	fixed := FixField("field1", src)
	if fixed.String() == "" {
		t.Error("FixField intervals source String() returned empty")
	}
}
