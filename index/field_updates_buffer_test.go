// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"math"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Tests in this file mirror the scenarios in Lucene 10.4.0's
// TestFieldUpdatesBuffer.java, ported to a table-driven Go style.

func newTermFromString(field, text string) *Term {
	return &Term{Field: field, Bytes: util.NewBytesRef([]byte(text))}
}

func mustFinish(t *testing.T, b *FieldUpdatesBuffer) {
	t.Helper()
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
}

func mustIterator(t *testing.T, b *FieldUpdatesBuffer) *BufferedUpdateIterator {
	t.Helper()
	it, err := b.Iterator()
	if err != nil {
		t.Fatalf("Iterator: %v", err)
	}
	return it
}

// mustNext drains one update or fails the test.
func mustNext(t *testing.T, it *BufferedUpdateIterator) *BufferedUpdate {
	t.Helper()
	up, err := it.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if up == nil {
		t.Fatalf("Next: unexpected end of iterator")
	}
	return up
}

// TestFieldUpdatesBuffer_Basics ports Lucene's testBasics(). It exercises
// insertion-order iteration, the single-value flag flipping, max/min
// tracking and the divergent-field case.
func TestFieldUpdatesBuffer_Basics(t *testing.T) {
	counter := util.NewCounter()
	buf, err := NewFieldUpdatesBufferNumeric(counter, newTermFromString("id", "1"), 15, 6, true)
	if err != nil {
		t.Fatalf("NewFieldUpdatesBufferNumeric: %v", err)
	}

	if err := buf.AddUpdate(newTermFromString("id", "10"), 6, 15); err != nil {
		t.Fatalf("AddUpdate: %v", err)
	}
	if !buf.HasSingleValue() {
		t.Fatalf("HasSingleValue: want true after two equal-value updates")
	}

	if err := buf.AddUpdate(newTermFromString("id", "8"), 12, 15); err != nil {
		t.Fatalf("AddUpdate: %v", err)
	}
	if buf.HasSingleValue() {
		t.Fatalf("HasSingleValue: want false after divergent value")
	}

	if err := buf.AddUpdate(newTermFromString("some_other_field", "8"), 13, 17); err != nil {
		t.Fatalf("AddUpdate: %v", err)
	}
	if err := buf.AddUpdate(newTermFromString("id", "8"), 12, 16); err != nil {
		t.Fatalf("AddUpdate: %v", err)
	}

	if !buf.IsNumeric() {
		t.Fatalf("IsNumeric: want true")
	}
	if got, want := buf.GetMaxNumeric(), int64(13); got != want {
		t.Fatalf("GetMaxNumeric: got %d want %d", got, want)
	}
	if got, want := buf.GetMinNumeric(), int64(6); got != want {
		t.Fatalf("GetMinNumeric: got %d want %d", got, want)
	}

	mustFinish(t, buf)
	it := mustIterator(t, buf)

	want := []BufferedUpdate{
		{TermField: "id", NumericValue: 6, DocUpTo: 15, HasValue: true},
		{TermField: "id", NumericValue: 6, DocUpTo: 15, HasValue: true},
		{TermField: "id", NumericValue: 12, DocUpTo: 15, HasValue: true},
		{TermField: "some_other_field", NumericValue: 13, DocUpTo: 17, HasValue: true},
		{TermField: "id", NumericValue: 12, DocUpTo: 16, HasValue: true},
	}
	wantTerm := []string{"1", "10", "8", "8", "8"}

	for i := range want {
		up := mustNext(t, it)
		if up.TermField != want[i].TermField {
			t.Fatalf("update %d: TermField got %q want %q", i, up.TermField, want[i].TermField)
		}
		if got := string(up.TermValue.ValidBytes()); got != wantTerm[i] {
			t.Fatalf("update %d: TermValue got %q want %q", i, got, wantTerm[i])
		}
		if up.NumericValue != want[i].NumericValue {
			t.Fatalf("update %d: NumericValue got %d want %d", i, up.NumericValue, want[i].NumericValue)
		}
		if up.DocUpTo != want[i].DocUpTo {
			t.Fatalf("update %d: DocUpTo got %d want %d", i, up.DocUpTo, want[i].DocUpTo)
		}
		if up.HasValue != want[i].HasValue {
			t.Fatalf("update %d: HasValue got %v want %v", i, up.HasValue, want[i].HasValue)
		}
	}

	last, err := it.Next()
	if err != nil {
		t.Fatalf("Next (trailing): %v", err)
	}
	if last != nil {
		t.Fatalf("Next (trailing): want nil, got %+v", last)
	}
}

// TestFieldUpdatesBuffer_ShareValuesNumeric mirrors testUpdateShareValues.
// The numeric value is shared across all updates, so the buffer should
// collapse to one stored value and use sorted iteration.
func TestFieldUpdatesBuffer_ShareValuesNumeric(t *testing.T) {
	for _, valueForThree := range []bool{true, false} {
		valueForThree := valueForThree
		t.Run("valueForThree="+boolStr(valueForThree), func(t *testing.T) {
			counter := util.NewCounter()
			const value = int64(42)

			buf, err := NewFieldUpdatesBufferNumeric(counter, newTermFromString("id", "0"), math.MaxInt32, value, true)
			if err != nil {
				t.Fatalf("constructor: %v", err)
			}
			if err := buf.AddUpdate(newTermFromString("id", "1"), value, math.MaxInt32); err != nil {
				t.Fatalf("AddUpdate: %v", err)
			}
			if err := buf.AddUpdate(newTermFromString("id", "2"), value, math.MaxInt32); err != nil {
				t.Fatalf("AddUpdate: %v", err)
			}
			if valueForThree {
				if err := buf.AddUpdate(newTermFromString("id", "3"), value, math.MaxInt32); err != nil {
					t.Fatalf("AddUpdate: %v", err)
				}
			} else {
				if err := buf.AddNoValue(newTermFromString("id", "3"), math.MaxInt32); err != nil {
					t.Fatalf("AddNoValue: %v", err)
				}
			}
			if err := buf.AddUpdate(newTermFromString("id", "4"), value, math.MaxInt32); err != nil {
				t.Fatalf("AddUpdate: %v", err)
			}
			mustFinish(t, buf)

			it := mustIterator(t, buf)
			count := 0
			for {
				up, err := it.Next()
				if err != nil {
					t.Fatalf("Next: %v", err)
				}
				if up == nil {
					break
				}
				wantHas := count != 3 || valueForThree
				wantTerm := digitString(count)
				if got := string(up.TermValue.ValidBytes()); got != wantTerm {
					t.Fatalf("count %d: TermValue got %q want %q", count, got, wantTerm)
				}
				if up.TermField != "id" {
					t.Fatalf("count %d: TermField got %q want id", count, up.TermField)
				}
				if up.HasValue != wantHas {
					t.Fatalf("count %d: HasValue got %v want %v", count, up.HasValue, wantHas)
				}
				if wantHas {
					if up.NumericValue != value {
						t.Fatalf("count %d: NumericValue got %d want %d", count, up.NumericValue, value)
					}
				} else if up.NumericValue != 0 {
					t.Fatalf("count %d: NumericValue got %d want 0 (reset)", count, up.NumericValue)
				}
				if up.DocUpTo != math.MaxInt32 {
					t.Fatalf("count %d: DocUpTo got %d want MaxInt32", count, up.DocUpTo)
				}
				count++
			}
			if count != 5 {
				t.Fatalf("iterated %d updates, want 5", count)
			}
			if !buf.IsNumeric() {
				t.Fatalf("IsNumeric: want true")
			}
		})
	}
}

// TestFieldUpdatesBuffer_ShareValuesBinary mirrors testUpdateShareValuesBinary.
func TestFieldUpdatesBuffer_ShareValuesBinary(t *testing.T) {
	for _, valueForThree := range []bool{true, false} {
		valueForThree := valueForThree
		t.Run("valueForThree="+boolStr(valueForThree), func(t *testing.T) {
			counter := util.NewCounter()
			empty := util.NewBytesRef([]byte(""))

			buf, err := NewFieldUpdatesBufferBinary(counter, newTermFromString("id", "0"), math.MaxInt32, empty, true)
			if err != nil {
				t.Fatalf("constructor: %v", err)
			}
			if err := buf.AddBinaryUpdate(newTermFromString("id", "1"), empty, math.MaxInt32); err != nil {
				t.Fatalf("AddBinaryUpdate: %v", err)
			}
			if err := buf.AddBinaryUpdate(newTermFromString("id", "2"), empty, math.MaxInt32); err != nil {
				t.Fatalf("AddBinaryUpdate: %v", err)
			}
			if valueForThree {
				if err := buf.AddBinaryUpdate(newTermFromString("id", "3"), empty, math.MaxInt32); err != nil {
					t.Fatalf("AddBinaryUpdate: %v", err)
				}
			} else {
				if err := buf.AddNoValue(newTermFromString("id", "3"), math.MaxInt32); err != nil {
					t.Fatalf("AddNoValue: %v", err)
				}
			}
			if err := buf.AddBinaryUpdate(newTermFromString("id", "4"), empty, math.MaxInt32); err != nil {
				t.Fatalf("AddBinaryUpdate: %v", err)
			}
			mustFinish(t, buf)

			it := mustIterator(t, buf)
			count := 0
			for {
				up, err := it.Next()
				if err != nil {
					t.Fatalf("Next: %v", err)
				}
				if up == nil {
					break
				}
				wantHas := count != 3 || valueForThree
				wantTerm := digitString(count)
				if got := string(up.TermValue.ValidBytes()); got != wantTerm {
					t.Fatalf("count %d: TermValue got %q want %q", count, got, wantTerm)
				}
				if up.HasValue != wantHas {
					t.Fatalf("count %d: HasValue got %v want %v", count, up.HasValue, wantHas)
				}
				if wantHas {
					if up.BinaryValue == nil || string(up.BinaryValue.ValidBytes()) != "" {
						t.Fatalf("count %d: BinaryValue got %v want empty", count, up.BinaryValue)
					}
				} else if up.BinaryValue != nil {
					t.Fatalf("count %d: BinaryValue got %v want nil", count, up.BinaryValue)
				}
				count++
			}
			if count != 5 {
				t.Fatalf("iterated %d updates, want 5", count)
			}
			if buf.IsNumeric() {
				t.Fatalf("IsNumeric: want false")
			}
		})
	}
}

// TestFieldUpdatesBuffer_NoNumericValue mirrors testNoNumericValue.
func TestFieldUpdatesBuffer_NoNumericValue(t *testing.T) {
	counter := util.NewCounter()
	buf, err := NewFieldUpdatesBufferNumeric(counter, newTermFromString("id", "1"), 0, 0, false)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if got := buf.GetMinNumeric(); got != 0 {
		t.Fatalf("GetMinNumeric: got %d want 0", got)
	}
	if got := buf.GetMaxNumeric(); got != 0 {
		t.Fatalf("GetMaxNumeric: got %d want 0", got)
	}
}

// TestFieldUpdatesBuffer_SortAndDedup mirrors testSortAndDedupNumericUpdatesByTerms:
// when every update shares the same field and the same value, the buffer
// sorts terms and de-duplicates duplicate runs by keeping the entry with
// the lowest docUpTo (stable-sort preserves insertion order, so the entry
// kept is whichever was inserted first — equivalent to the lowest docUpTo
// in the random test since docUpTo increases monotonically with insertion).
func TestFieldUpdatesBuffer_SortAndDedup(t *testing.T) {
	const numUpdates = 200
	const value = int64(7)
	const termField = "id"

	rng := rand.New(rand.NewPCG(1, 2))
	counter := util.NewCounter()

	type rec struct {
		text    string
		docUpTo int
	}
	var inserted []rec

	first := digitString(int(rng.Int32N(50)))
	buf, err := NewFieldUpdatesBufferNumeric(counter, newTermFromString(termField, first), 0, value, true)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	inserted = append(inserted, rec{text: first, docUpTo: 0})

	for i := 0; i < numUpdates; i++ {
		text := digitString(int(rng.Int32N(50)))
		docUpTo := i + 1
		if err := buf.AddUpdate(newTermFromString(termField, text), value, docUpTo); err != nil {
			t.Fatalf("AddUpdate %d: %v", i, err)
		}
		inserted = append(inserted, rec{text: text, docUpTo: docUpTo})
	}

	mustFinish(t, buf)
	it := mustIterator(t, buf)
	if !it.IsSortedTerms() {
		t.Fatalf("IsSortedTerms: want true when all updates share field and value")
	}

	// Build the expected output: sort all inserted by text (stable, so
	// equal texts keep insertion order); then for each run of equal
	// texts, keep the LAST. Lucene's look-ahead dedup discards the
	// current entry whenever the next entry is equal with a larger
	// insertion ord, so the survivor of each run is the last-inserted.
	sort.SliceStable(inserted, func(i, j int) bool {
		return inserted[i].text < inserted[j].text
	})
	var expected []rec
	for i, r := range inserted {
		if i == len(inserted)-1 || r.text != inserted[i+1].text {
			expected = append(expected, r)
		}
	}

	var got []rec
	for {
		up, err := it.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if up == nil {
			break
		}
		got = append(got, rec{text: string(up.TermValue.ValidBytes()), docUpTo: up.DocUpTo})
	}
	if len(got) != len(expected) {
		t.Fatalf("iterated %d terms, want %d", len(got), len(expected))
	}
	for i := range expected {
		if got[i].text != expected[i].text {
			t.Fatalf("term %d: got %q want %q", i, got[i].text, expected[i].text)
		}
		if got[i].docUpTo != expected[i].docUpTo {
			t.Fatalf("term %d (%q): docUpTo got %d want %d", i, got[i].text, got[i].docUpTo, expected[i].docUpTo)
		}
	}
}

// TestFieldUpdatesBuffer_FinishTwice asserts the documented error contract
// on double-finish.
func TestFieldUpdatesBuffer_FinishTwice(t *testing.T) {
	buf, err := NewFieldUpdatesBufferNumeric(util.NewCounter(), newTermFromString("f", "v"), 0, 1, true)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if err := buf.Finish(); err != nil {
		t.Fatalf("first Finish: %v", err)
	}
	if err := buf.Finish(); err == nil {
		t.Fatalf("second Finish: want error, got nil")
	}
}

// TestFieldUpdatesBuffer_AddAfterFinish asserts that adds fail post-Finish.
func TestFieldUpdatesBuffer_AddAfterFinish(t *testing.T) {
	buf, err := NewFieldUpdatesBufferNumeric(util.NewCounter(), newTermFromString("f", "v"), 0, 1, true)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	mustFinish(t, buf)
	if err := buf.AddUpdate(newTermFromString("f", "v2"), 2, 1); err == nil {
		t.Fatalf("AddUpdate after Finish: want error")
	}
}

// TestFieldUpdatesBuffer_IteratorBeforeFinish asserts the Iterator
// pre-condition.
func TestFieldUpdatesBuffer_IteratorBeforeFinish(t *testing.T) {
	buf, err := NewFieldUpdatesBufferNumeric(util.NewCounter(), newTermFromString("f", "v"), 0, 1, true)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if _, err := buf.Iterator(); err == nil {
		t.Fatalf("Iterator before Finish: want error")
	}
}

// TestFieldUpdatesBuffer_BinaryNumericMix asserts that mixing modes on
// the wrong constructor variant returns an error.
func TestFieldUpdatesBuffer_BinaryNumericMix(t *testing.T) {
	numBuf, err := NewFieldUpdatesBufferNumeric(util.NewCounter(), newTermFromString("f", "v"), 0, 1, true)
	if err != nil {
		t.Fatalf("numeric constructor: %v", err)
	}
	if err := numBuf.AddBinaryUpdate(newTermFromString("f", "v"), util.NewBytesRef([]byte("x")), 0); err == nil {
		t.Fatalf("AddBinaryUpdate on numeric buffer: want error")
	}

	binBuf, err := NewFieldUpdatesBufferBinary(util.NewCounter(), newTermFromString("f", "v"), 0, util.NewBytesRef([]byte("x")), true)
	if err != nil {
		t.Fatalf("binary constructor: %v", err)
	}
	if err := binBuf.AddUpdate(newTermFromString("f", "v"), 1, 0); err == nil {
		t.Fatalf("AddUpdate on binary buffer: want error")
	}
}

// TestFieldUpdatesBuffer_GetNumericValueOnBinaryPanics asserts that
// GetNumericValue panics when invoked on a binary buffer, matching the
// GetMaxNumeric / GetMinNumeric assertion pattern. Without this guard
// the binary path would silently return data from the (nil) numeric
// slice via the getArrayIndex clamp.
func TestFieldUpdatesBuffer_GetNumericValueOnBinaryPanics(t *testing.T) {
	buf, err := NewFieldUpdatesBufferBinary(util.NewCounter(), newTermFromString("f", "v"), 0, util.NewBytesRef([]byte("x")), true)
	if err != nil {
		t.Fatalf("binary constructor: %v", err)
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("GetNumericValue on binary buffer: expected panic, got none")
		}
	}()
	_ = buf.GetNumericValue(0)
}

// TestFieldUpdatesBuffer_GetNumericValueReset asserts that GetNumericValue
// returns 0 for entries that lacked a value, mirroring Lucene.
func TestFieldUpdatesBuffer_GetNumericValueReset(t *testing.T) {
	buf, err := NewFieldUpdatesBufferNumeric(util.NewCounter(), newTermFromString("id", "0"), 0, 100, true)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	if err := buf.AddNoValue(newTermFromString("id", "1"), 0); err != nil {
		t.Fatalf("AddNoValue: %v", err)
	}
	if err := buf.AddUpdate(newTermFromString("id", "2"), 200, 0); err != nil {
		t.Fatalf("AddUpdate: %v", err)
	}
	mustFinish(t, buf)

	if got := buf.GetNumericValue(0); got != 100 {
		t.Fatalf("GetNumericValue(0): got %d want 100", got)
	}
	if got := buf.GetNumericValue(1); got != 0 {
		t.Fatalf("GetNumericValue(1) on reset entry: got %d want 0", got)
	}
	if got := buf.GetNumericValue(2); got != 200 {
		t.Fatalf("GetNumericValue(2): got %d want 200", got)
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func digitString(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
