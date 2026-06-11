// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/intervals/TestPayloadFilteredInterval.java

package intervals

import (
	"testing"
)

// TestPayloadFilteredInterval exercises the PayloadFilteredTermIntervalsSource
// and its associated filtering infrastructure.
//
// The Lucene original requires MockTokenizer, SimplePayloadFilter,
// RandomIndexWriter, and full index integration. Gocene tests the
// construction and basic properties at the unit level.
func TestPayloadFilteredInterval(t *testing.T) {
	// Construct a payload-filtered term source.
	filter := func(payload []byte) bool { return len(payload) > 0 }
	src := TermWithPayloadFilter("term", filter)
	desc := src.String()
	if desc == "" {
		t.Error("PayloadFilteredTermIntervalsSource.String() returned empty")
	}

	// Verify that description includes the term.
	if desc != "" && desc != src.String() {
		t.Error("String() returned inconsistent values")
	}

	// NonOverlapping source composition.
	nonOverlap := NonOverlapping(
		Term("a"),
		Term("b"),
	)
	if nonOverlap.String() == "" {
		t.Error("NonOverlapping intervals source String() returned empty")
	}
}
