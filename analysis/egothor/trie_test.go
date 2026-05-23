// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package egothor

import (
	"testing"
)

// assertTrieContents verifies that trie.GetFully and trie.GetLastOnPath return
// the expected values for every key, and repeats the assertion after reducing
// with all standard reducers. This mirrors org.egothor.stemmer.TestStemmer
// (Lucene 10.4.0).
func assertTrieContents(t *testing.T, trie *Trie, keys, vals []string) {
	t.Helper()
	tries := []*Trie{
		trie,
		trie.Reduce(new(Optimizer)),
		trie.Reduce(new(Optimizer2)),
		trie.Reduce(new(Gener)),
		trie.Reduce(NewLift(true)),
		trie.Reduce(NewLift(false)),
	}
	for _, tr := range tries {
		for i, key := range keys {
			kr := []rune(key)
			got := tr.GetFully(kr)
			if got != vals[i] {
				t.Errorf("GetFully(%q) = %q, want %q", key, got, vals[i])
			}
			got = tr.GetLastOnPath(kr)
			if got != vals[i] {
				t.Errorf("GetLastOnPath(%q) = %q, want %q", key, got, vals[i])
			}
		}
	}
}

// TestTrie mirrors org.egothor.stemmer.TestStemmer.testTrie.
func TestTrie(t *testing.T) {
	tr := NewTrie(true)

	keys := []string{"a", "ba", "bb", "c"}
	vals := []string{"1", "2", "2", "4"}

	for i, key := range keys {
		tr.Add([]rune(key), vals[i])
	}

	if tr.root != 0 {
		t.Errorf("root = %d, want 0", tr.root)
	}
	if len(tr.rows) != 2 {
		t.Errorf("rows = %d, want 2", len(tr.rows))
	}
	if len(tr.cmds) != 3 {
		t.Errorf("cmds = %d, want 3", len(tr.cmds))
	}
	assertTrieContents(t, tr, keys, vals)
}

// TestTrieBackwards mirrors org.egothor.stemmer.TestStemmer.testTrieBackwards.
func TestTrieBackwards(t *testing.T) {
	tr := NewTrie(false)

	keys := []string{"a", "ba", "bb", "c"}
	vals := []string{"1", "2", "2", "4"}

	for i, key := range keys {
		tr.Add([]rune(key), vals[i])
	}
	assertTrieContents(t, tr, keys, vals)
}

// TestMultiTrie mirrors org.egothor.stemmer.TestStemmer.testMultiTrie.
func TestMultiTrie(t *testing.T) {
	mt := NewMultiTrie(true)

	keys := []string{"a", "ba", "bb", "c"}
	vals := []string{"1", "2", "2", "4"}

	for i, key := range keys {
		mt.Add([]rune(key), vals[i])
	}
	// Wrap as *Trie for assertTrieContents via a thin adapter.
	assertMultiTrieContents(t, mt, keys, vals)
}

// TestMultiTrieBackwards mirrors org.egothor.stemmer.TestStemmer.testMultiTrieBackwards.
func TestMultiTrieBackwards(t *testing.T) {
	mt := NewMultiTrie(false)

	keys := []string{"a", "ba", "bb", "c"}
	vals := []string{"1", "2", "2", "4"}

	for i, key := range keys {
		mt.Add([]rune(key), vals[i])
	}
	assertMultiTrieContents(t, mt, keys, vals)
}

// TestMultiTrie2 mirrors org.egothor.stemmer.TestStemmer.testMultiTrie2.
func TestMultiTrie2(t *testing.T) {
	mt := NewMultiTrie2(true)

	keys := []string{"a", "ba", "bb", "c"}
	// Longer values are required for MultiTrie2 (short vals cause IOOBE in Java too).
	vals := []string{"1111", "2222", "2223", "4444"}

	for i, key := range keys {
		mt.Add([]rune(key), vals[i])
	}
	assertMultiTrie2Contents(t, mt, keys, vals)
}

// TestMultiTrie2Backwards mirrors org.egothor.stemmer.TestStemmer.testMultiTrie2Backwards.
func TestMultiTrie2Backwards(t *testing.T) {
	mt := NewMultiTrie2(false)

	keys := []string{"a", "ba", "bb", "c"}
	vals := []string{"1111", "2222", "2223", "4444"}

	for i, key := range keys {
		mt.Add([]rune(key), vals[i])
	}
	assertMultiTrie2Contents(t, mt, keys, vals)
}

type multiTrieLike interface {
	GetFully(key []rune) string
	GetLastOnPath(key []rune) string
	Reduce(by Reducer) *MultiTrie
}

func assertMultiTrieContents(t *testing.T, mt *MultiTrie, keys, vals []string) {
	t.Helper()
	reducers := []Reducer{
		new(Optimizer),
		new(Optimizer2),
		new(Gener),
		NewLift(true),
		NewLift(false),
	}
	sources := []*MultiTrie{mt}
	for _, r := range reducers {
		sources = append(sources, mt.Reduce(r))
	}
	for _, tr := range sources {
		for i, key := range keys {
			kr := []rune(key)
			got := tr.GetFully(kr)
			if got != vals[i] {
				t.Errorf("MultiTrie.GetFully(%q) = %q, want %q", key, got, vals[i])
			}
			got = tr.GetLastOnPath(kr)
			if got != vals[i] {
				t.Errorf("MultiTrie.GetLastOnPath(%q) = %q, want %q", key, got, vals[i])
			}
		}
	}
}

func assertMultiTrie2Contents(t *testing.T, mt *MultiTrie2, keys, vals []string) {
	t.Helper()
	reducers := []Reducer{
		new(Optimizer),
		new(Optimizer2),
		new(Gener),
		NewLift(true),
		NewLift(false),
	}
	sources := []*MultiTrie2{mt}
	for _, r := range reducers {
		sources = append(sources, mt.Reduce(r))
	}
	for _, tr := range sources {
		for i, key := range keys {
			kr := []rune(key)
			got := tr.GetFully(kr)
			if got != vals[i] {
				t.Errorf("MultiTrie2.GetFully(%q) = %q, want %q", key, got, vals[i])
			}
			got = tr.GetLastOnPath(kr)
			if got != vals[i] {
				t.Errorf("MultiTrie2.GetLastOnPath(%q) = %q, want %q", key, got, vals[i])
			}
		}
	}
}
