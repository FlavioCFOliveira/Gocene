package taxonomy

import "testing"

func TestLRUHashMap(t *testing.T) {
	m := NewLRUHashMap[string, int](2)
	m.Put("a", 1)
	m.Put("b", 2)
	if v, ok := m.Get("a"); !ok || v != 1 {
		t.Error("get a")
	}
	// "a" was just accessed, "b" is now LRU
	if k, _, evicted := m.Put("c", 3); !evicted || k != "b" {
		t.Errorf("expected eviction of b, got %v (evicted=%v)", k, evicted)
	}
	if _, ok := m.Get("b"); ok {
		t.Error("b should be evicted")
	}
}
