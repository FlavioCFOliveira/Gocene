package join

import "testing"

func TestReduceLongs(t *testing.T) {
	if v := ReduceLongs(BlockJoinMin, []int64{3, 1, 2}); v != 1 {
		t.Errorf("min = %d", v)
	}
	if v := ReduceLongs(BlockJoinMax, []int64{3, 1, 2}); v != 3 {
		t.Errorf("max = %d", v)
	}
	if v := ReduceLongs(BlockJoinSum, []int64{3, 1, 2}); v != 6 {
		t.Errorf("sum = %d", v)
	}
	if v := ReduceLongs(BlockJoinAvg, []int64{3, 1, 2}); v != 2 {
		t.Errorf("avg = %d", v)
	}
}

func TestReduceDoubles(t *testing.T) {
	if v := ReduceDoubles(BlockJoinSum, []float64{1.5, 2.5}); v != 4.0 {
		t.Errorf("sum = %v", v)
	}
}
