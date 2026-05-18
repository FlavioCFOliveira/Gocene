package facetset

import "testing"

func TestIntDecoder(t *testing.T) {
	s := NewIntFacetSet(7, -1, 0)
	buf := make([]byte, s.SizeInBytes())
	s.PackValues(buf)
	out := make([]int64, 3)
	n := IntDecoder(buf, 0, 3, out)
	if n != 12 || out[0] != 7 || out[1] != -1 || out[2] != 0 {
		t.Errorf("got out=%v n=%d", out, n)
	}
}

func TestLongDecoder(t *testing.T) {
	s := NewLongFacetSet(7, -1)
	buf := make([]byte, s.SizeInBytes())
	s.PackValues(buf)
	out := make([]int64, 2)
	LongDecoder(buf, 0, 2, out)
	if out[0] != 7 || out[1] != -1 {
		t.Errorf("got %v", out)
	}
}

func TestFloatRoundTrip(t *testing.T) {
	cases := []float32{0, 0.5, -0.5, 1e6, -1e6}
	for _, v := range cases {
		got := SortableIntToFloat(floatToSortableInt(v))
		if got != v {
			t.Errorf("round trip %v -> %v", v, got)
		}
	}
}

func TestDoubleRoundTrip(t *testing.T) {
	cases := []float64{0, 0.5, -0.5, 1e18, -1e18}
	for _, v := range cases {
		got := SortableLongToDouble(doubleToSortableLong(v))
		if got != v {
			t.Errorf("round trip %v -> %v", v, got)
		}
	}
}
