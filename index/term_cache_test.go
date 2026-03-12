// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
)

func TestNewTermCache(t *testing.T) {
	cache := NewTermCache(100)
	if cache == nil {
		t.Fatal("NewTermCache returned nil")
	}
	if cache.MaxSize() != 100 {
		t.Errorf("Expected max size 100, got %d", cache.MaxSize())
	}
	if cache.Size() != 0 {
		t.Errorf("Expected initial size 0, got %d", cache.Size())
	}
}

func TestNewTermCache_DefaultSize(t *testing.T) {
	cache := NewTermCache(0)
	if cache.MaxSize() != 1000 {
		t.Errorf("Expected default max size 1000, got %d", cache.MaxSize())
	}
}

func TestTermCache_PutAndGet(t *testing.T) {
	cache := NewTermCache(10)
	term := NewTerm("field", "value")
	entry := &TermCacheEntry{
		Term:    term,
		DocFreq: 5,
	}

	// Put entry
	cache.Put(term, entry)

	// Get entry
	got, found := cache.Get(term)
	if !found {
		t.Fatal("Expected to find cached term")
	}
	if got == nil {
		t.Fatal("Get returned nil entry")
	}
	if got.DocFreq != 5 {
		t.Errorf("Expected DocFreq 5, got %d", got.DocFreq)
	}
}

func TestTermCache_Get_NotFound(t *testing.T) {
	cache := NewTermCache(10)
	term := NewTerm("field", "nonexistent")

	got, found := cache.Get(term)
	if found {
		t.Error("Expected not to find uncached term")
	}
	if got != nil {
		t.Error("Expected nil for uncached term")
	}
}

func TestTermCache_LRU_Eviction(t *testing.T) {
	cache := NewTermCache(3)

	// Add 3 items
	for i := 0; i < 3; i++ {
		term := NewTerm("field", string(rune('a'+i)))
		entry := &TermCacheEntry{Term: term, DocFreq: int64(i)}
		cache.Put(term, entry)
	}

	if cache.Size() != 3 {
		t.Errorf("Expected size 3, got %d", cache.Size())
	}

	// Access first item to make it recently used
	firstTerm := NewTerm("field", "a")
	cache.Get(firstTerm)

	// Add 4th item - should evict 'b' (least recently used)
	fourthTerm := NewTerm("field", "d")
	cache.Put(fourthTerm, &TermCacheEntry{Term: fourthTerm, DocFreq: 3})

	if cache.Size() != 3 {
		t.Errorf("Expected size 3 after eviction, got %d", cache.Size())
	}

	// 'a' should still be there (was accessed)
	_, foundA := cache.Get(firstTerm)
	if !foundA {
		t.Error("Expected 'a' to still be in cache (was recently used)")
	}

	// 'b' should be evicted
	secondTerm := NewTerm("field", "b")
	_, foundB := cache.Get(secondTerm)
	if foundB {
		t.Error("Expected 'b' to be evicted")
	}
}

func TestTermCache_Invalidate(t *testing.T) {
	cache := NewTermCache(10)
	term := NewTerm("field", "value")
	entry := &TermCacheEntry{Term: term, DocFreq: 5}

	cache.Put(term, entry)
	if cache.Size() != 1 {
		t.Errorf("Expected size 1, got %d", cache.Size())
	}

	cache.Invalidate(term)
	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after invalidation, got %d", cache.Size())
	}

	_, found := cache.Get(term)
	if found {
		t.Error("Expected term to be invalidated")
	}
}

func TestTermCache_InvalidateAll(t *testing.T) {
	cache := NewTermCache(10)

	// Add multiple items
	for i := 0; i < 5; i++ {
		term := NewTerm("field", string(rune('a'+i)))
		cache.Put(term, &TermCacheEntry{Term: term, DocFreq: int64(i)})
	}

	if cache.Size() != 5 {
		t.Errorf("Expected size 5, got %d", cache.Size())
	}

	cache.InvalidateAll()

	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after InvalidateAll, got %d", cache.Size())
	}
}

func TestTermCache_UpdateExisting(t *testing.T) {
	cache := NewTermCache(10)
	term := NewTerm("field", "value")

	// First put
	entry1 := &TermCacheEntry{Term: term, DocFreq: 5}
	cache.Put(term, entry1)

	// Update same term
	entry2 := &TermCacheEntry{Term: term, DocFreq: 10}
	cache.Put(term, entry2)

	if cache.Size() != 1 {
		t.Errorf("Expected size 1 after update, got %d", cache.Size())
	}

	got, found := cache.Get(term)
	if !found {
		t.Fatal("Expected to find term")
	}
	if got.DocFreq != 10 {
		t.Errorf("Expected updated DocFreq 10, got %d", got.DocFreq)
	}
}

func TestTermCache_Resize(t *testing.T) {
	cache := NewTermCache(5)

	// Add 5 items
	for i := 0; i < 5; i++ {
		term := NewTerm("field", string(rune('a'+i)))
		cache.Put(term, &TermCacheEntry{Term: term})
	}

	if cache.Size() != 5 {
		t.Errorf("Expected size 5, got %d", cache.Size())
	}

	// Resize to 3 - should evict 2 oldest
	cache.Resize(3)

	if cache.Size() != 3 {
		t.Errorf("Expected size 3 after resize, got %d", cache.Size())
	}
	if cache.MaxSize() != 3 {
		t.Errorf("Expected max size 3, got %d", cache.MaxSize())
	}
}

func TestTermCache_Resize_Larger(t *testing.T) {
	cache := NewTermCache(3)

	// Add 3 items
	for i := 0; i < 3; i++ {
		term := NewTerm("field", string(rune('a'+i)))
		cache.Put(term, &TermCacheEntry{Term: term})
	}

	// Resize to larger - should keep all items
	cache.Resize(10)

	if cache.Size() != 3 {
		t.Errorf("Expected size 3 after resize to larger, got %d", cache.Size())
	}
}

func TestTermCache_GetStats(t *testing.T) {
	cache := NewTermCache(100)

	// Add some items
	for i := 0; i < 5; i++ {
		term := NewTerm("field", string(rune('a'+i)))
		cache.Put(term, &TermCacheEntry{Term: term})
	}

	stats := cache.GetStats()
	if stats.Size != 5 {
		t.Errorf("Expected stats size 5, got %d", stats.Size)
	}
	if stats.MaxSize != 100 {
		t.Errorf("Expected stats max size 100, got %d", stats.MaxSize)
	}
}

func TestTermCache_NilSafety(t *testing.T) {
	var cache *TermCache

	// Should not panic
	term := NewTerm("field", "value")

	cache.Get(term)
	cache.Put(term, &TermCacheEntry{Term: term})
	cache.Invalidate(term)
	cache.InvalidateAll()
	cache.Size()
	cache.MaxSize()
	cache.Resize(10)
	cache.GetStats()
}

func TestTermCache_NilTerm(t *testing.T) {
	cache := NewTermCache(10)

	// Should not panic with nil term
	cache.Get(nil)
	cache.Put(nil, &TermCacheEntry{})
	cache.Invalidate(nil)
}

func TestTermCache_NilEntry(t *testing.T) {
	cache := NewTermCache(10)
	term := NewTerm("field", "value")

	// Should not panic with nil entry
	cache.Put(term, nil)
}

func TestTermCacheEntry_String(t *testing.T) {
	term := NewTerm("field", "value")
	entry := &TermCacheEntry{Term: term}

	result := entry.String()
	if result == "" || result == "nil" {
		t.Error("Expected non-empty string representation")
	}
	if result != term.String() {
		t.Errorf("Expected String() to match term.String(), got %s", result)
	}
}

func TestTermCacheEntry_String_Nil(t *testing.T) {
	var entry *TermCacheEntry
	if entry.String() != "nil" {
		t.Errorf("Expected 'nil' for nil entry, got %s", entry.String())
	}

	entry = &TermCacheEntry{Term: nil}
	if entry.String() != "nil" {
		t.Errorf("Expected 'nil' for entry with nil term, got %s", entry.String())
	}
}

func TestTermCache_ConcurrentAccess(t *testing.T) {
	cache := NewTermCache(100)

	// Concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				term := NewTerm("field", string(rune('a'+j%26)))
				cache.Put(term, &TermCacheEntry{Term: term, DocFreq: int64(id)})
			}
			done <- true
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				term := NewTerm("field", string(rune('a'+j%26)))
				cache.Get(term)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Cache should be consistent
	if cache.Size() > cache.MaxSize() {
		t.Errorf("Cache size %d exceeds max size %d", cache.Size(), cache.MaxSize())
	}
}
