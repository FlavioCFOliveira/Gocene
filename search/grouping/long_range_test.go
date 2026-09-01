package grouping

import (
	"math"
	"testing"
)

func TestLongRange(t *testing.T) {
	lr := NewLongRange(10, 20)
	if lr.Min != 10 || lr.Max != 20 {
		t.Errorf("expected LongRange(10, 20), got %s", lr.String())
	}
}

func TestLongRangeFactory(t *testing.T) {
	factory := NewLongRangeFactory(100, 10, 200)

	tests := []struct {
		value    int64
		expected [2]int64
	}{
		{50, [2]int64{math.MinInt64, 100}},
		{100, [2]int64{100, 110}},
		{105, [2]int64{100, 110}},
		{109, [2]int64{100, 110}},
		{110, [2]int64{110, 120}},
		{155, [2]int64{150, 160}},
		{199, [2]int64{190, 200}},
		{200, [2]int64{200, math.MaxInt64}},
		{250, [2]int64{200, math.MaxInt64}},
	}

	for _, tc := range tests {
		rangeRes := factory.GetRange(tc.value, nil)
		if rangeRes.Min != tc.expected[0] || rangeRes.Max != tc.expected[1] {
			t.Errorf("for value %d, expected range [%d, %d), got [%d, %d)",
				tc.value, tc.expected[0], tc.expected[1], rangeRes.Min, rangeRes.Max)
		}
	}
}

func TestLongRangeFactoryReuse(t *testing.T) {
	factory := NewLongRangeFactory(100, 10, 200)
	reuse := &LongRange{}

	val1 := int64(105)
	res1 := factory.GetRange(val1, reuse)
	if res1 != reuse {
		t.Error("GetRange did not reuse the provided LongRange object")
	}
	if res1.Min != 100 || res1.Max != 110 {
		t.Errorf("expected [100, 110), got [%d, %d)", res1.Min, res1.Max)
	}

	val2 := int64(155)
	res2 := factory.GetRange(val2, reuse)
	if res2 != reuse {
		t.Error("GetRange did not reuse the provided LongRange object")
	}
	if res2.Min != 150 || res2.Max != 160 {
		t.Errorf("expected [150, 160), got [%d, %d)", res2.Min, res2.Max)
	}
}
