package facets

import "testing"

func TestTopOrdAndFloatQueueKeepsTopN(t *testing.T) {
	q := NewTopOrdAndFloatQueue(3)
	q.InsertFloat(1, 5)
	q.InsertFloat(2, 1)
	q.InsertFloat(3, 9)
	q.InsertFloat(4, 7)
	if q.Size() != 3 {
		t.Fatalf("size = %d", q.Size())
	}
	got := make(map[int]float32, 3)
	for q.Size() > 0 {
		ord, v, _ := q.PopFloat()
		got[ord] = v
	}
	if _, ok := got[2]; ok {
		t.Error("smallest (ord=2) should have been evicted")
	}
}

func TestTopOrdAndFloatQueueClear(t *testing.T) {
	q := NewTopOrdAndFloatQueue(2)
	q.InsertFloat(1, 1)
	q.Clear()
	if q.Size() != 0 {
		t.Error("Clear")
	}
}
