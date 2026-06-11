// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest_test

// TestPersistence mirrors org.apache.lucene.search.suggest.TestPersistence.
//
// The Java original tests that TSTLookup and FSTCompletionLookup can be
// serialised to disk and reloaded.  These tests use in-memory byte arrays
// for the Store→Load→Lookup round-trip.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/suggest/fst"
	"github.com/FlavioCFOliveira/Gocene/suggest/tst"
)

var persistenceKeys = []string{
	"one", "two", "three", "four", "oneness", "onerous", "onesimus",
	"twofold", "twonk", "thrive", "through", "threat",
	"foundation", "fourier", "fourty",
}

// TestPersistence_TSTRoundTrip builds a TSTLookup, stores it to a byte
// array, loads a fresh lookup from those bytes, and verifies that lookup
// results match before and after the round-trip.
func TestPersistence_TSTRoundTrip(t *testing.T) {
	inputs := make([]*Input, len(persistenceKeys))
	for i, k := range persistenceKeys {
		inputs[i] = NewInput(k, int64(i))
	}

	// Build original.
	orig := tst.NewTSTLookup()
	if err := orig.Build(NewInputArrayIterator(inputs)); err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Store to byte array.
	buf := store.NewByteArrayDataOutput(4096)
	stored, err := orig.Store(buf)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if !stored {
		t.Fatal("Store returned false (no data written)")
	}

	// Load from byte array.
	reloaded := tst.NewTSTLookup()
	data := buf.GetBytes()
	loaded, err := reloaded.Load(store.NewByteArrayDataInput(data))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded {
		t.Fatal("Load returned false (no data read)")
	}

	// Verify counts match.
	if orig.GetCount() != reloaded.GetCount() {
		t.Errorf("GetCount: orig=%d, reloaded=%d", orig.GetCount(), reloaded.GetCount())
	}

	// Verify lookup results match for several prefixes.
	for _, prefix := range []string{"o", "t", "f", "one", "two", "th", "fo"} {
		origResults, err := orig.LookupResults(prefix, nil, false, 10)
		if err != nil {
			t.Fatalf("orig LookupResults(%q): %v", prefix, err)
		}
		reloadedResults, err := reloaded.LookupResults(prefix, nil, false, 10)
		if err != nil {
			t.Fatalf("reloaded LookupResults(%q): %v", prefix, err)
		}
		if len(origResults) != len(reloadedResults) {
			t.Errorf("LookupResults(%q): orig len=%d, reloaded len=%d",
				prefix, len(origResults), len(reloadedResults))
			continue
		}
		for i := range origResults {
			if origResults[i].Key != reloadedResults[i].Key ||
				origResults[i].Value != reloadedResults[i].Value {
				t.Errorf("LookupResults(%q)[%d]: orig=(%q,%d), reloaded=(%q,%d)",
					prefix, i,
					origResults[i].Key, origResults[i].Value,
					reloadedResults[i].Key, reloadedResults[i].Value)
			}
		}
	}
}

// TestPersistence_TSTEmptyRoundTrip verifies that storing and loading an
// empty TSTLookup works.
func TestPersistence_TSTEmptyRoundTrip(t *testing.T) {
	orig := tst.NewTSTLookup()

	buf := store.NewByteArrayDataOutput(64)
	stored, err := orig.Store(buf)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if stored {
		t.Log("Store returned true for empty lookup (expected false)")
	}

	reloaded := tst.NewTSTLookup()
	data := buf.GetBytes()
	_, err = reloaded.Load(store.NewByteArrayDataInput(data))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.GetCount() != 0 {
		t.Errorf("expected count 0 after load, got %d", reloaded.GetCount())
	}
}

// TestPersistence_FSTRoundTrip builds an FSTCompletionLookup, stores it,
// reloads, and verifies results match.
func TestPersistence_FSTRoundTrip(t *testing.T) {
	inputs := make([]*Input, len(persistenceKeys))
	for i, k := range persistenceKeys {
		inputs[i] = NewInput(k, int64(i))
	}

	orig := fst.NewFSTCompletionLookup(10, false)
	if err := orig.Build(NewInputArrayIterator(inputs)); err != nil {
		t.Fatalf("Build: %v", err)
	}

	buf := store.NewByteArrayDataOutput(4096)
	stored, err := orig.Store(buf)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if !stored {
		t.Fatal("Store returned false (no data written)")
	}

	reloaded := fst.NewFSTCompletionLookup(10, false)
	data := buf.GetBytes()
	loaded, err := reloaded.Load(store.NewByteArrayDataInput(data))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded {
		t.Fatal("Load returned false (no data read)")
	}

	if orig.GetCount() != reloaded.GetCount() {
		t.Errorf("GetCount: orig=%d, reloaded=%d", orig.GetCount(), reloaded.GetCount())
	}

	for _, prefix := range []string{"o", "t", "f", "one", "two", "th", "fo"} {
		origResults, err := orig.LookupResults(prefix, nil, false, 10)
		if err != nil {
			t.Fatalf("orig LookupResults(%q): %v", prefix, err)
		}
		reloadedResults, err := reloaded.LookupResults(prefix, nil, false, 10)
		if err != nil {
			t.Fatalf("reloaded LookupResults(%q): %v", prefix, err)
		}
		if len(origResults) != len(reloadedResults) {
			t.Errorf("LookupResults(%q): orig len=%d, reloaded len=%d",
				prefix, len(origResults), len(reloadedResults))
			continue
		}
		for i := range origResults {
			if origResults[i].Key != reloadedResults[i].Key ||
				origResults[i].Value != reloadedResults[i].Value {
				t.Errorf("LookupResults(%q)[%d]: orig=(%q,%d), reloaded=(%q,%d)",
					prefix, i,
					origResults[i].Key, origResults[i].Value,
					reloadedResults[i].Key, reloadedResults[i].Value)
			}
		}
	}
}

// TestPersistence_FSTEmptyRoundTrip verifies that storing and loading an
// empty FSTCompletionLookup works.
func TestPersistence_FSTEmptyRoundTrip(t *testing.T) {
	orig := fst.NewFSTCompletionLookup(10, false)

	buf := store.NewByteArrayDataOutput(64)
	stored, err := orig.Store(buf)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if stored {
		t.Log("Store returned true for empty lookup (expected false)")
	}

	reloaded := fst.NewFSTCompletionLookup(10, false)
	data := buf.GetBytes()
	_, err = reloaded.Load(store.NewByteArrayDataInput(data))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.GetCount() != 0 {
		t.Errorf("expected count 0 after load, got %d", reloaded.GetCount())
	}
}
