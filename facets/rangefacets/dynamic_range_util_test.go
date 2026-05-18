package rangefacets

import "testing"

func TestComputeDynamicLongRanges(t *testing.T) {
	values := []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	ranges := ComputeDynamicLongRanges(values, 2)
	if len(ranges) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(ranges))
	}
	if ranges[0].Min != 1 || ranges[0].Max != 5 {
		t.Errorf("bucket0 = [%d,%d]", ranges[0].Min, ranges[0].Max)
	}
	if ranges[1].Min != 6 || ranges[1].Max != 10 {
		t.Errorf("bucket1 = [%d,%d]", ranges[1].Min, ranges[1].Max)
	}
}

func TestComputeDynamicLongRangesEmpty(t *testing.T) {
	if r := ComputeDynamicLongRanges(nil, 5); r != nil {
		t.Error("empty input should produce nil")
	}
	if r := ComputeDynamicLongRanges([]int64{1, 2}, 0); r != nil {
		t.Error("zero buckets should produce nil")
	}
}
