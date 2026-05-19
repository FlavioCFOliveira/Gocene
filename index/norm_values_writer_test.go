// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// newTestNormWriter wires up a NormValuesWriter with a fresh field info and
// a dedicated counter.
func newTestNormWriter(t *testing.T, name string) (*NormValuesWriter, *util.Counter) {
	t.Helper()
	fi := NewFieldInfo(name, 0, FieldInfoOptions{})
	counter := util.NewCounter()
	w, err := NewNormValuesWriter(fi, counter)
	if err != nil {
		t.Fatalf("NewNormValuesWriter: %v", err)
	}
	return w, counter
}

// TestNormValuesWriter_NewValidatesArgs ensures the constructor refuses nil
// inputs rather than returning a writer that would NPE on first use.
func TestNormValuesWriter_NewValidatesArgs(t *testing.T) {
	if _, err := NewNormValuesWriter(nil, util.NewCounter()); err == nil {
		t.Fatalf("expected error on nil fieldInfo")
	}
	fi := NewFieldInfo("f", 0, FieldInfoOptions{})
	if _, err := NewNormValuesWriter(fi, nil); err == nil {
		t.Fatalf("expected error on nil counter")
	}
}

// TestNormValuesWriter_AddAndFlush exercises the common contiguous-docs path
// and verifies that the consumer sees values in the order they were added.
func TestNormValuesWriter_AddAndFlush(t *testing.T) {
	w, counter := newTestNormWriter(t, "norms")

	values := []int64{10, 20, 30, 40}
	for i, v := range values {
		if err := w.AddValue(i, v); err != nil {
			t.Fatalf("AddValue(%d,%d): %v", i, v, err)
		}
	}
	if got := counter.Get(); got <= 0 {
		t.Fatalf("expected positive iwBytesUsed after adds, got %d", got)
	}

	var seen []struct {
		doc int
		val int64
	}
	consumer := func(field *FieldInfo, dv NumericDocValues) error {
		for {
			d, err := dv.NextDoc()
			if err != nil {
				return err
			}
			if d == NO_MORE_DOCS {
				return nil
			}
			v, err := dv.Get(d)
			if err != nil {
				return err
			}
			seen = append(seen, struct {
				doc int
				val int64
			}{d, v})
		}
	}
	if err := w.Flush(nil, nil, consumer); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if len(seen) != len(values) {
		t.Fatalf("seen %d docs, want %d", len(seen), len(values))
	}
	for i, s := range seen {
		if s.doc != i || s.val != values[i] {
			t.Fatalf("seen[%d] = {%d,%d}, want {%d,%d}", i, s.doc, s.val, i, values[i])
		}
	}
}

// TestNormValuesWriter_SparseDocs covers the bitset path of DocsWithFieldSet,
// where added docIDs are not contiguous.
func TestNormValuesWriter_SparseDocs(t *testing.T) {
	w, _ := newTestNormWriter(t, "f")
	inputs := []struct {
		doc int
		v   int64
	}{
		{0, 1},
		{5, 2},
		{42, 3},
		{1000, 4},
	}
	for _, in := range inputs {
		if err := w.AddValue(in.doc, in.v); err != nil {
			t.Fatalf("AddValue(%d): %v", in.doc, err)
		}
	}
	var seen []int64
	var docs []int
	if err := w.Flush(nil, nil, func(_ *FieldInfo, dv NumericDocValues) error {
		for {
			d, err := dv.NextDoc()
			if err != nil {
				return err
			}
			if d == NO_MORE_DOCS {
				return nil
			}
			v, _ := dv.Get(d)
			docs = append(docs, d)
			seen = append(seen, v)
		}
	}); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if len(seen) != len(inputs) {
		t.Fatalf("seen %d values, want %d", len(seen), len(inputs))
	}
	for i, in := range inputs {
		if docs[i] != in.doc || seen[i] != in.v {
			t.Fatalf("seen[%d] = {%d,%d}, want {%d,%d}", i, docs[i], seen[i], in.doc, in.v)
		}
	}
}

// TestNormValuesWriter_RejectsDuplicateDoc enforces the single-value-per-doc
// invariant: same docID twice must error with a message mentioning the field.
func TestNormValuesWriter_RejectsDuplicateDoc(t *testing.T) {
	w, _ := newTestNormWriter(t, "title")
	if err := w.AddValue(0, 1); err != nil {
		t.Fatalf("AddValue(0): %v", err)
	}
	err := w.AddValue(0, 2)
	if err == nil {
		t.Fatalf("expected duplicate-doc error")
	}
	if !strings.Contains(err.Error(), "title") || !strings.Contains(err.Error(), "more than once") {
		t.Fatalf("error %q lacks field/diagnostic text", err.Error())
	}
}

// TestNormValuesWriter_RejectsOutOfOrderDoc enforces strictly increasing
// docIDs; a step backwards must error.
func TestNormValuesWriter_RejectsOutOfOrderDoc(t *testing.T) {
	w, _ := newTestNormWriter(t, "f")
	if err := w.AddValue(5, 1); err != nil {
		t.Fatalf("AddValue(5): %v", err)
	}
	if err := w.AddValue(3, 1); err == nil {
		t.Fatalf("expected out-of-order error")
	}
}

// TestNormValuesWriter_FlushNilConsumer guards against a nil callback.
func TestNormValuesWriter_FlushNilConsumer(t *testing.T) {
	w, _ := newTestNormWriter(t, "f")
	_ = w.AddValue(0, 7)
	if err := w.Flush(nil, nil, nil); err == nil {
		t.Fatalf("expected error on nil consumer")
	}
}

// TestNormValuesWriter_FlushSortMapNotSupported documents the deferred
// sort-aware path: Flush with a non-nil sortMap must surface a clear error
// until NumericDocValuesWriter.sortDocValues is ported.
func TestNormValuesWriter_FlushSortMapNotSupported(t *testing.T) {
	w, _ := newTestNormWriter(t, "f")
	_ = w.AddValue(0, 1)
	err := w.Flush(nil, identityDocMap(1), func(*FieldInfo, NumericDocValues) error { return nil })
	if err == nil {
		t.Fatalf("expected unsupported-sort error")
	}
	if !strings.Contains(err.Error(), "not yet ported") {
		t.Fatalf("error %q lacks deferred-port marker", err.Error())
	}
}

// TestBufferedNorms_AdvanceUnsupported confirms that Advance returns the
// documented sentinel and never silently succeeds.
func TestBufferedNorms_AdvanceUnsupported(t *testing.T) {
	w, _ := newTestNormWriter(t, "f")
	_ = w.AddValue(0, 1)
	_ = w.Flush(nil, nil, func(_ *FieldInfo, dv NumericDocValues) error {
		if _, err := dv.Advance(7); !errors.Is(err, errBufferedNormsAdvance) {
			t.Fatalf("Advance err = %v, want %v", err, errBufferedNormsAdvance)
		}
		return nil
	})
}

// TestNormValuesWriter_FinishIsNoop covers the no-op Finish hook for parity
// with the Java DocValuesWriter contract.
func TestNormValuesWriter_FinishIsNoop(t *testing.T) {
	w, _ := newTestNormWriter(t, "f")
	_ = w.AddValue(0, 1)
	w.Finish(1) // must not panic and must leave the writer flushable.
	if err := w.Flush(nil, nil, func(*FieldInfo, NumericDocValues) error { return nil }); err != nil {
		t.Fatalf("Flush after Finish: %v", err)
	}
}

// identityDocMap returns a SorterDocMap that is the identity permutation,
// usable as a sentinel sortMap to drive the deferred-flush branch.
func identityDocMap(size int) SorterDocMap { return identitySorterDocMap(size) }

type identitySorterDocMap int

func (i identitySorterDocMap) OldToNew(d int) int { return d }
func (i identitySorterDocMap) NewToOld(d int) int { return d }
func (i identitySorterDocMap) Size() int          { return int(i) }
