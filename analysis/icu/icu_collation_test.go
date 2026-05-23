// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package icu_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis/icu"
	"github.com/FlavioCFOliveira/Gocene/analysis/icu/tokenattributes"
)

// ---- ICUCollationDocValuesField tests (task 2966) ----

// TestICUCollationDocValuesField_SetStringValue verifies that the collation
// key is stored as the doc-values bytes.
func TestICUCollationDocValuesField_SetStringValue(t *testing.T) {
	collator := &lexicographicCollator{}
	f, err := icu.NewICUCollationDocValuesField("sortField", collator)
	if err != nil {
		t.Fatalf("NewICUCollationDocValuesField: %v", err)
	}
	if err := f.SetStringValue("hello"); err != nil {
		t.Fatalf("SetStringValue: %v", err)
	}
	got := f.GetValue()
	if string(got) != "hello" {
		t.Errorf("got %q, want %q", string(got), "hello")
	}
}

// TestICUCollationDocValuesField_Name verifies that the field name is
// preserved.
func TestICUCollationDocValuesField_Name(t *testing.T) {
	collator := &lexicographicCollator{}
	f, err := icu.NewICUCollationDocValuesField("myField", collator)
	if err != nil {
		t.Fatalf("NewICUCollationDocValuesField: %v", err)
	}
	if f.Name() != "myField" {
		t.Errorf("got %q, want %q", f.Name(), "myField")
	}
}

// TestICUCollationDocValuesField_SortOrder verifies that lexicographic
// sort order is preserved by the collation key.
func TestICUCollationDocValuesField_SortOrder(t *testing.T) {
	collator := &lexicographicCollator{}
	words := []string{"banana", "apple", "cherry"}
	var keys [][]byte
	for _, w := range words {
		f, err := icu.NewICUCollationDocValuesField("f", collator)
		if err != nil {
			t.Fatal(err)
		}
		_ = f.SetStringValue(w)
		keys = append(keys, f.GetValue())
	}
	// With a lexicographic collator the keys should sort the same as strings.
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			cmp := strings.Compare(string(keys[i]), string(keys[j]))
			want := strings.Compare(words[i], words[j])
			if (cmp < 0) != (want < 0) || (cmp > 0) != (want > 0) {
				t.Errorf("key order mismatch: words[%d]=%q words[%d]=%q", i, words[i], j, words[j])
			}
		}
	}
}

// ---- ICUCollationKeyAnalyzer tests (task 2967) ----

// TestICUCollationKeyAnalyzer_TokenStream verifies that the analyser emits
// a single token whose bytes are the collation key.
func TestICUCollationKeyAnalyzer_TokenStream(t *testing.T) {
	collator := &lexicographicCollator{}
	a := icu.NewICUCollationKeyAnalyzer(collator)
	defer a.Close()

	ts, err := a.TokenStream("field", strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("TokenStream: %v", err)
	}

	ok, err := ts.IncrementToken()
	if err != nil {
		t.Fatalf("IncrementToken: %v", err)
	}
	if !ok {
		t.Fatal("expected a token")
	}

	// Retrieve the collation key via the concrete type.
	cts := ts.(*icu.CollationKeyTokenStream)
	key := cts.GetCollationKey()
	if key == nil {
		t.Fatal("GetCollationKey returned nil")
	}
	// With the lexicographic collator the key equals the input bytes.
	if string(key.Bytes[:key.Length]) != "hello" {
		t.Errorf("got %q, want %q", key.Bytes[:key.Length], "hello")
	}
}

// TestICUCollationKeyAnalyzer_Close verifies that Close is a no-op.
func TestICUCollationKeyAnalyzer_Close(t *testing.T) {
	a := icu.NewICUCollationKeyAnalyzer(&lexicographicCollator{})
	if err := a.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// TestCollationKeyBytes_Direct verifies the helper function.
func TestCollationKeyBytes_Direct(t *testing.T) {
	collator := &lexicographicCollator{}
	key := icu.CollationKeyBytes(collator, "world")
	if string(key) != "world" {
		t.Errorf("got %q, want %q", string(key), "world")
	}
}

// Ensure the package-level Collator alias is satisfied.
var _ tokenattributes.Collator = (*lexicographicCollator)(nil)
