package taxonomywritercache

import "testing"

func TestLruTaxonomyWriterCacheBasic(t *testing.T) {
	c := NewLruTaxonomyWriterCache(2)
	c.Put("a", 1)
	c.Put("b", 2)
	if c.Get("a") != 1 || c.Get("b") != 2 {
		t.Error("basic Get")
	}
	if c.Size() != 2 || !c.IsFull() {
		t.Errorf("size=%d full=%v", c.Size(), c.IsFull())
	}
	// inserting a third evicts the LRU ("a" was promoted, so "b" is LRU)
	c.Put("c", 3)
	if c.Get("b") != -1 {
		t.Error("b should have been evicted")
	}
	if c.Get("a") != 1 || c.Get("c") != 3 {
		t.Error("a and c should survive")
	}
}

func TestLruTaxonomyWriterCacheClear(t *testing.T) {
	c := NewLruTaxonomyWriterCache(2)
	c.Put("a", 1)
	c.Clear()
	if c.Size() != 0 || c.Get("a") != -1 {
		t.Error("Clear")
	}
}
