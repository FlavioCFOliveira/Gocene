// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"math"
	"testing"
)

// Port of
// lucene/grouping/src/test/org/apache/lucene/search/grouping/TestDoubleRangeFactory.java
// (Apache Lucene 10.5.0).

func TestDoubleRangeFactory(t *testing.T) {
	factory := NewDoubleRangeFactory(10, 10, 50)
	scratch := NewDoubleRange(0, 0)
	for _, c := range []struct {
		expected *DoubleRange
		value    float64
	}{
		{NewDoubleRange(math.SmallestNonzeroFloat64, 10), 4},
		{NewDoubleRange(10, 20), 10},
		{NewDoubleRange(20, 30), 20},
		{NewDoubleRange(10, 20), 15},
		{NewDoubleRange(30, 40), 35},
		{NewDoubleRange(50, math.MaxFloat64), 50},
		{NewDoubleRange(50, math.MaxFloat64), 500},
	} {
		if got := factory.GetRange(c.value, scratch); !c.expected.Equals(got) {
			t.Fatalf("getRange(%v): expected %v, got %v", c.value, c.expected, got)
		}
	}
}
