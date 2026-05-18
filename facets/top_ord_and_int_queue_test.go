package facets

import "testing"

func TestTopOrdAndIntQueueKeepsTopN(t *testing.T) {
	q := NewTopOrdAndIntQueue(3)
	q.InsertInt(1, 5)
	q.InsertInt(2, 1)
	q.InsertInt(3, 9)
	q.InsertInt(4, 7)
	if q.Size() != 3 {
		t.Fatalf("size = %d", q.Size())
	}
	got := make(map[int]int32, 3)
	for q.Size() > 0 {
		ord, v, _ := q.PopInt()
		got[ord] = v
	}
	if _, ok := got[2]; ok {
		t.Error("smallest should be evicted")
	}
}
