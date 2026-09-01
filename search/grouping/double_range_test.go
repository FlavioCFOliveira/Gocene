package grouping

import (
	"math"
	"testing"
)

func TestDoubleRangeFactory_GetRange(t *testing.T) {
	factory := NewDoubleRangeFactory(0.0, 10.0, 100.0)

	tests := []struct {
		name     string
		value    float64
		wantMin  float64
		wantMax  float64
	}{
		{
			name:    "below min",
			value:   -5.0,
			wantMin: math.SmallestNonzeroFloat64,
			wantMax: 0.0,
		},
		{
			name:    "at min",
			value:   0.0,
			wantMin: 0.0,
			wantMax: 10.0,
		},
		{
			name:    "mid range",
			value:   15.0,
			wantMin: 10.0,
			wantMax: 20.0,
		},
		{
			name:    "mid range boundary",
			value:   19.99,
			wantMin: 10.0,
			wantMax: 20.0,
		},
		{
			name:    "above max",
			value:   105.0,
			wantMin: 100.0,
			wantMax: math.MaxFloat64,
		},
		{
			name:    "at max",
			value:   100.0,
			wantMin: 100.0,
			wantMax: math.MaxFloat64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := factory.GetRange(tt.value, nil)
			if got.Min != tt.wantMin || got.Max != tt.wantMax {
				t.Errorf("GetRange(%g) = DoubleRange(%g, %g), want DoubleRange(%g, %g)", tt.value, got.Min, got.Max, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestDoubleRangeFactory_Reuse(t *testing.T) {
	factory := NewDoubleRangeFactory(0.0, 10.0, 100.0)
	reuse := &DoubleRange{Min: 1.23, Max: 4.56}

	res := factory.GetRange(15.0, reuse)

	if res != reuse {
		t.Error("GetRange did not reuse the provided object")
	}
	if res.Min != 10.0 || res.Max != 20.0 {
		t.Errorf("GetRange(15.0, reuse) = DoubleRange(%g, %g), want DoubleRange(10.0, 20.0)", res.Min, res.Max)
	}
}
