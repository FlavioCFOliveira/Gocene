// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"testing"
)

func TestBytesRefHash_AddFind(t *testing.T) {
	h := NewBytesRefHash()
	refs := []string{"apple", "banana", "cherry", "apple"}

	ids := make([]int, len(refs))
	for i, s := range refs {
		ref := &BytesRef{Bytes: []byte(s), Offset: 0, Length: len(s)}
		id, err := h.Add(ref)
		if err != nil {
			t.Fatalf("Add() error: %v", err)
		}
		ids[i] = id
	}

	// "apple" should be added twice, but only once in hash
	if ids[3] >= 0 {
		t.Errorf("Expected negative ID for duplicate 'apple', got %d", ids[3])
	}

	for _, s := range refs {
		ref := &BytesRef{Bytes: []byte(s), Offset: 0, Length: len(s)}
		id := h.Find(ref)
		if id != ids[0] && s == "apple" {
			t.Errorf("Find('apple') = %d, want %d", id, ids[0])
		}
	}
}

func TestBytesRefHash_Sort(t *testing.T) {
	h := NewBytesRefHash()
	refs := []string{"cherry", "banana", "apple", "date"}

	for _, s := range refs {
		ref := &BytesRef{Bytes: []byte(s), Offset: 0, Length: len(s)}
		h.Add(ref)
	}

	sortedIds := h.Sort()

	expected := []string{"apple", "banana", "cherry", "date"}
	for i := 0; i < len(expected); i++ {
		ref := NewBytesRefEmpty()
		h.Get(sortedIds[i], ref)
		if string(ref.ValidBytes()) != expected[i] {
			t.Errorf("Sort() at %d = %s, want %s", i, string(ref.ValidBytes()), expected[i])
		}
	}
}

func TestBytesRefHash_Clear(t *testing.T) {
	h := NewBytesRefHash()
	ref := &BytesRef{Bytes: []byte("test"), Offset: 0, Length: 4}
	h.Add(ref)
	if h.Size() != 1 {
		t.Errorf("Size() = %d, want 1", h.Size())
	}

	h.Clear(true)
	if h.Size() != 0 {
		t.Errorf("Size() after Clear = %d, want 0", h.Size())
	}
}
