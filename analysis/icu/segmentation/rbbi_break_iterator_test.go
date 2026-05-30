// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

import (
	"reflect"
	"testing"
)

// segmentRBBI drives an RBBIBreakIterator over s and returns the boundary
// spans together with the rule-status of each span.
func segmentRBBI(t *testing.T, name, s string) ([]string, []int) {
	t.Helper()
	dict, err := LoadEmbeddedBRK(name)
	if err != nil {
		t.Fatalf("LoadEmbeddedBRK(%s): %v", name, err)
	}
	data, err := dict.RBBIData()
	if err != nil {
		t.Fatalf("RBBIData(%s): %v", name, err)
	}
	bi := newRBBIBreakIterator(data)
	r := []rune(s)
	bi.SetText(r, 0, len(r))

	var spans []string
	var status []int
	prev := bi.Current()
	if prev != 0 {
		t.Fatalf("Current() before first Next() = %d, want 0", prev)
	}
	for guard := 0; ; guard++ {
		if guard > len(r)+2 {
			t.Fatal("iterator did not terminate")
		}
		n := bi.Next()
		if n == Done {
			break
		}
		spans = append(spans, string(r[prev:n]))
		status = append(status, bi.GetRuleStatus())
		prev = n
	}
	return spans, status
}

// TestRBBIBreakIterator_MyanmarBoundaries asserts the forward executor returns
// the exact syllable boundaries (and that the dictionary-free MyanmarSyllable
// rules require no second pass).
func TestRBBIBreakIterator_MyanmarBoundaries(t *testing.T) {
	t.Parallel()
	spans, _ := segmentRBBI(t, EmbeddedMyanmarSyllableBRKName, "သက်ဝင်လှုပ်ရှားစေပြီး")
	want := []string{"သက်", "ဝင်", "လှုပ်", "ရှား", "စေ", "ပြီး"}
	if !reflect.DeepEqual(spans, want) {
		t.Errorf("Myanmar spans = %q, want %q", spans, want)
	}
}

// TestRBBIBreakIterator_DefaultBoundaries asserts the Default.brk forward
// executor splits CJK per ideograph and keeps Latin words/numbers intact, with
// the correct rule-status (tag) values.
func TestRBBIBreakIterator_DefaultBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("cjk per ideograph", func(t *testing.T) {
		spans, status := segmentRBBI(t, EmbeddedDefaultBRKName, "中文")
		wantSpans := []string{"中", "文"}
		wantStatus := []int{RuleStatusWordIdeo, RuleStatusWordIdeo}
		if !reflect.DeepEqual(spans, wantSpans) {
			t.Errorf("spans = %q, want %q", spans, wantSpans)
		}
		if !reflect.DeepEqual(status, wantStatus) {
			t.Errorf("status = %v, want %v", status, wantStatus)
		}
	})

	t.Run("latin words and spaces", func(t *testing.T) {
		spans, status := segmentRBBI(t, EmbeddedDefaultBRKName, "hello world")
		wantSpans := []string{"hello", " ", "world"}
		wantStatus := []int{RuleStatusWordLetter, RuleStatusWordNone, RuleStatusWordLetter}
		if !reflect.DeepEqual(spans, wantSpans) {
			t.Errorf("spans = %q, want %q", spans, wantSpans)
		}
		if !reflect.DeepEqual(status, wantStatus) {
			t.Errorf("status = %v, want %v", status, wantStatus)
		}
	})

	t.Run("number with internal comma", func(t *testing.T) {
		spans, status := segmentRBBI(t, EmbeddedDefaultBRKName, "4,600")
		wantSpans := []string{"4,600"}
		wantStatus := []int{RuleStatusWordNumber}
		if !reflect.DeepEqual(spans, wantSpans) {
			t.Errorf("spans = %q, want %q", spans, wantSpans)
		}
		if !reflect.DeepEqual(status, wantStatus) {
			t.Errorf("status = %v, want %v", status, wantStatus)
		}
	})
}

// TestRBBIBreakIterator_EmptyAndExhaustion checks edge cases: empty text, a
// single Next past the end, and idempotent Done.
func TestRBBIBreakIterator_EmptyAndExhaustion(t *testing.T) {
	t.Parallel()
	dict, err := LoadEmbeddedBRK(EmbeddedDefaultBRKName)
	if err != nil {
		t.Fatalf("LoadEmbeddedBRK: %v", err)
	}
	data, err := dict.RBBIData()
	if err != nil {
		t.Fatalf("RBBIData: %v", err)
	}
	bi := newRBBIBreakIterator(data)

	// Empty region: first Next is Done.
	bi.SetText([]rune("abc"), 0, 0)
	if got := bi.Next(); got != Done {
		t.Errorf("empty region Next() = %d, want Done", got)
	}
	if got := bi.Next(); got != Done {
		t.Errorf("empty region second Next() = %d, want Done", got)
	}

	// Single token then Done, repeatedly Done afterwards.
	r := []rune("hi")
	bi.SetText(r, 0, len(r))
	if got := bi.Next(); got != len(r) {
		t.Errorf("Next() = %d, want %d", got, len(r))
	}
	if got := bi.Next(); got != Done {
		t.Errorf("Next() after end = %d, want Done", got)
	}
	if got := bi.Current(); got != Done {
		t.Errorf("Current() after end = %d, want Done", got)
	}
}

// TestRBBIBreakIterator_Clone verifies clone independence.
func TestRBBIBreakIterator_Clone(t *testing.T) {
	t.Parallel()
	dict, _ := LoadEmbeddedBRK(EmbeddedDefaultBRKName)
	data, err := dict.RBBIData()
	if err != nil {
		t.Fatalf("RBBIData: %v", err)
	}
	bi := newRBBIBreakIterator(data)
	r := []rune("中文 abc")
	bi.SetText(r, 0, len(r))
	bi.Next() // advance original

	clone := bi.Clone()
	if clone == RuleBasedBreakIterator(bi) {
		t.Fatal("Clone returned the same instance")
	}
	// Advancing the clone must not affect the original's position.
	orig := bi.Current()
	clone.Next()
	if bi.Current() != orig {
		t.Errorf("original position changed after clone advanced: got %d, want %d", bi.Current(), orig)
	}
}
