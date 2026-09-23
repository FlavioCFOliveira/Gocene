// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"math"
	"testing"
)

// Port of
// lucene/grouping/src/test/org/apache/lucene/search/grouping/TestLongRangeFactory.java
// (Apache Lucene 10.5.0).

func TestLongRangeFactory(t *testing.T) {
	factory := NewLongRangeFactory(10, 10, 50)
	scratch := NewLongRange(0, 0)
	for _, c := range []struct {
		expected *LongRange
		value    int64
	}{
		{NewLongRange(math.MinInt64, 10), 4},
		{NewLongRange(10, 20), 10},
		{NewLongRange(20, 30), 20},
		{NewLongRange(10, 20), 15},
		{NewLongRange(30, 40), 35},
		{NewLongRange(50, math.MaxInt64), 50},
		{NewLongRange(50, math.MaxInt64), 500},
	} {
		if got := factory.GetRange(c.value, scratch); !c.expected.Equals(got) {
			t.Fatalf("getRange(%v): expected %v, got %v", c.value, c.expected, got)
		}
	}
}
