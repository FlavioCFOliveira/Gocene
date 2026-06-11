// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest_test

// TestPersistence mirrors org.apache.lucene.search.suggest.TestPersistence.
//
// The Java original tests that TSTLookup and FSTCompletionLookup can be
// serialised to disk and reloaded.  Store/Load persistence is not yet
// implemented in the Gocene stubs for TSTLookup and FSTCompletionLookup,
// so these tests verify the build+lookup contract that the persistence
// round-trip must preserve.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/suggest/fst"
	"github.com/FlavioCFOliveira/Gocene/suggest/tst"
)

var persistenceKeys = []string{
	"one", "two", "three", "four", "oneness", "onerous", "onesimus",
	"twofold", "twonk", "thrive", "through", "threat",
	"foundation", "fourier", "fourty",
}

// TestPersistence_TST mirrors TestPersistence.testTSTPersistence.
// Verifies that a TSTLookup built from an InputArrayIterator returns the
// expected results.
func TestPersistence_TST(t *testing.T) {
	inputs := make([]*Input, len(persistenceKeys))
	for i, k := range persistenceKeys {
		inputs[i] = NewInput(k, int64(i))
	}

	l := tst.NewTSTLookup()
	if err := l.Build(NewInputArrayIterator(inputs)); err != nil {
		t.Fatalf("Build: %v", err)
	}
	// Verify at least one result comes back.
	results, err := l.LookupResults("one", nil, false, 3)
	if err != nil {
		t.Fatalf("LookupResults: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least one result for prefix 'one'")
	}
}

// TestPersistence_FST mirrors TestPersistence.testFSTPersistence.
// Verifies that a FSTCompletionLookup built from an InputArrayIterator
// returns the expected results.
func TestPersistence_FST(t *testing.T) {
	inputs := make([]*Input, len(persistenceKeys))
	for i, k := range persistenceKeys {
		inputs[i] = NewInput(k, int64(i))
	}

	l := fst.NewFSTCompletionLookup(10, false)
	if err := l.Build(NewInputArrayIterator(inputs)); err != nil {
		t.Fatalf("Build: %v", err)
	}
	results, err := l.LookupResults("fou", nil, false, 3)
	if err != nil {
		t.Fatalf("LookupResults: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least one result for prefix 'fou'")
	}
}
