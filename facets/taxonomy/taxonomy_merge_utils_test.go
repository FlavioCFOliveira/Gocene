package taxonomy

import "testing"

func TestMergeOrdinalMap(t *testing.T) {
	// src taxonomy: 0 (root, parent=-1), 1 (child of 0), 2 (child of 0)
	src := []int{-1, 0, 0}
	var dst []int
	next := 5 // dst starts at ordinal 5 (simulate non-empty dst)
	ordMap := MergeOrdinalMap(src, &dst, func(parent int) int {
		out := next
		next++
		return out
	})
	if len(ordMap) != 3 {
		t.Fatalf("ordMap len = %d", len(ordMap))
	}
	if ordMap[0] != 5 || ordMap[1] != 6 || ordMap[2] != 7 {
		t.Errorf("ordMap = %v", ordMap)
	}
	if dst[0] != -1 || dst[1] != 5 || dst[2] != 5 {
		t.Errorf("dst parents = %v", dst)
	}
}
