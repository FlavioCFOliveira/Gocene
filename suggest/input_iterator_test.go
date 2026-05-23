// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest_test

// TestInput and TestInputArrayIterator mirror
// org.apache.lucene.search.suggest.InputArrayIterator and
// org.apache.lucene.search.suggest.Input behaviour exercised by
// TestInputIterator.testEmpty / testTerms.

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/suggest"
)

// TestInputArrayIterator_Empty mirrors TestInputIterator.testEmpty.
// An empty InputArrayIterator must produce no entries when wrapped by
// SortedInputIterator or UnsortedInputIterator.
func TestInputArrayIterator_Empty(t *testing.T) {
	it := NewInputArrayIterator(nil)

	sorted, err := suggest.NewSortedInputIterator(it)
	if err != nil {
		t.Fatalf("NewSortedInputIterator: %v", err)
	}
	_, _, _, _, ok, err := sorted.Next()
	if err != nil {
		t.Fatalf("sorted.Next: %v", err)
	}
	if ok {
		t.Fatal("expected no entry from empty sorted iterator")
	}

	// re-create since we consumed it
	it2 := NewInputArrayIterator(nil)
	unsorted, err := suggest.NewUnsortedInputIterator(it2)
	if err != nil {
		t.Fatalf("NewUnsortedInputIterator: %v", err)
	}
	_, _, _, _, ok, err = unsorted.Next()
	if err != nil {
		t.Fatalf("unsorted.Next: %v", err)
	}
	if ok {
		t.Fatal("expected no entry from empty unsorted iterator")
	}
}

// TestInputArrayIterator_Terms mirrors TestInputIterator.testTerms for the
// unsorted wrapper: verifies that every input term/weight appears in the
// output (order may differ since UnsortedInputIterator shuffles).
func TestInputArrayIterator_Terms(t *testing.T) {
	inputs := []*Input{
		NewInput("banana", 3),
		NewInput("apple", 10),
		NewInput("cherry", 7),
		NewInput("date", 1),
		NewInput("elderberry", 5),
	}
	expected := map[string]int64{}
	for _, in := range inputs {
		expected[string(in.Term)] = in.Weight
	}

	unsorted, err := suggest.NewUnsortedInputIterator(NewInputArrayIterator(inputs))
	if err != nil {
		t.Fatalf("NewUnsortedInputIterator: %v", err)
	}

	got := map[string]int64{}
	for {
		term, weight, _, _, ok, err := unsorted.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		got[string(term)] = weight
	}
	if len(got) != len(expected) {
		t.Fatalf("got %d entries, want %d", len(got), len(expected))
	}
	for term, wantW := range expected {
		if gotW, ok := got[term]; !ok {
			t.Errorf("missing term %q", term)
		} else if gotW != wantW {
			t.Errorf("term %q: got weight %d, want %d", term, gotW, wantW)
		}
	}
}

// TestInputArrayIterator_WithPayload verifies that payload bytes are
// round-tripped correctly through InputArrayIterator.
func TestInputArrayIterator_WithPayload(t *testing.T) {
	payload := []byte("meta")
	inputs := []*Input{
		NewInputWithPayload("foo", 42, payload),
	}
	it := NewInputArrayIterator(inputs)
	if !it.HasPayloads() {
		t.Fatal("HasPayloads should be true")
	}
	term, weight, p, _, ok, err := it.Next()
	if err != nil || !ok {
		t.Fatalf("Next: ok=%v, err=%v", ok, err)
	}
	if string(term) != "foo" {
		t.Errorf("term: got %q, want %q", term, "foo")
	}
	if weight != 42 {
		t.Errorf("weight: got %d, want 42", weight)
	}
	if !bytes.Equal(p, payload) {
		t.Errorf("payload: got %v, want %v", p, payload)
	}
}

// TestInputArrayIterator_WithContexts verifies that context bytes are
// round-tripped correctly.
func TestInputArrayIterator_WithContexts(t *testing.T) {
	inputs := []*Input{
		NewInputWithContexts("doc", 1, "ctxA", "ctxB"),
	}
	it := NewInputArrayIterator(inputs)
	if !it.HasContexts() {
		t.Fatal("HasContexts should be true")
	}
	_, _, _, ctxs, ok, err := it.Next()
	if err != nil || !ok {
		t.Fatalf("Next: ok=%v, err=%v", ok, err)
	}
	if len(ctxs) != 2 {
		t.Fatalf("got %d contexts, want 2", len(ctxs))
	}
}
