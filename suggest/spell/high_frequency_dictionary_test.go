// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spell_test

import (
	"io"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/suggest/spell"
)

// TestHighFrequencyDictionary_Empty mirrors
// org.apache.lucene.search.suggest.TestHighFrequencyDictionary.testEmpty.
//
// An empty dictionary (no terms) wrapped in a HighFrequencyDictionary must
// produce an iterator whose first call to Next returns ok=false (no entries).
func TestHighFrequencyDictionary_Empty(t *testing.T) {
	// Use a PlainTextDictionary over an empty string as the empty source.
	inner := spell.NewPlainTextDictionary(emptyReader{})
	dict := spell.NewHighFrequencyDictionary(inner, 0)

	it := dict.GetEntryIterator()
	_, _, ok, err := it.Next()
	if err != nil {
		t.Fatalf("Next returned unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected no entry from empty dictionary, got one")
	}
}

// emptyReader is an io.Reader that immediately returns io.EOF.
type emptyReader struct{}

func (emptyReader) Read(_ []byte) (int, error) {
	return 0, io.EOF
}
