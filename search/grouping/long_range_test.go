package grouping

import (
	"math"
	"testing"
)

func TestLongRangeFactory_GetRange(t *testing.T) {
	tests := []struct {
		name     string
		min      int64
		width    int64
		max      int64
		value    int64
		expected LongRange
	}{
		{
			name:     "below min",
			min:      100,
			width:    10,
			max:      200,
			value:    50,
			expected: LongRange{Min: math.MinInt64, Max: 100},
		},
		{
			name:     "above max",
			min:      100,
			width:    10,
			max:      200,
			value:    250,
			expected: LongRange{Min: 200, Max: math.MaxInt64},
		},
		{
			name:     "exactly min",
			min:      100,
			width:    10,
			max:      200,
			value:    100,
			expected: LongRange{Min: 100, Max: 110},
		},
		{
			name:     "exactly max",
			min:      100,
			width:    10,
			max:      200,
			value:    200,
			expected: LongRange{Min: 200, Max: math.MaxInt64},
		},
		{
			name:     "inside range start",
			min:      100,
			width:    10,
			max:      200,
			value:    105,
			expected: LongRange{Min: 100, Max: 110},
		},
		{
			name:     "inside range middle",
			min:      100,
			width:    10,
			max:      200,
			value:    155,
			expected: LongRange{Min: 150, Max: 160},
		},
		{
			name:     "inside range end",
			min:      100,
			width:    10,
			max:      200,
			value:    195,
			expected: LongRange{Min: 190, Max: 200},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewLongRangeFactory(tt.min, tt.width, tt.max)

			// Test without reuse
			res := factory.GetRange(tt.value, nil)
			if res.Min != tt.expected.Min || res.Max != tt.expected.Max {
				t.Errorf("GetRange(%d, nil) = %v, want %v", tt.value, res, tt.expected)
			}

			// Test with reuse
			reuse := &LongRange{Min: 0, Max: 0}
			res = factory.GetRange(tt.value, reuse)
			if res.Min != tt.expected.Min || res.Max != tt.expected.Max {
				t.Errorf("GetRange(%d, reuse) = %v, want %v", tt.value, res, tt.expected)
			}
			if res != reuse {
				t.Errorf("GetRange did not reuse the provided object")
			}
		})
	}
}
