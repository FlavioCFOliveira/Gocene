// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"errors"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// stubFieldsProducer is a minimal FieldsProducer test double for
// per_field_merge_state_test. It records Close invocations and reports
// any Terms call (the helper must short-circuit forbidden fields before
// they reach inner.Terms).
type stubFieldsProducer struct {
	closed  int
	visited []string
}

func (s *stubFieldsProducer) Terms(field string) (index.Terms, error) {
	s.visited = append(s.visited, field)
	// nil Terms is sufficient: the helper forwards to inner.Terms without
	// inspecting the result; the test asserts on inner.visited.
	return nil, nil
}

func (s *stubFieldsProducer) Close() error {
	s.closed++
	return nil
}

// newTestFieldInfos builds a frozen *index.FieldInfos containing one
// indexed FieldInfo per name, with deterministic numbering matching the
// argument order.
func newTestFieldInfos(t *testing.T, names ...string) *index.FieldInfos {
	t.Helper()
	fis := index.NewFieldInfos()
	for i, name := range names {
		opts := index.DefaultFieldInfoOptions()
		opts.IndexOptions = index.IndexOptionsDocs
		if err := fis.Add(index.NewFieldInfo(name, i, opts)); err != nil {
			t.Fatalf("FieldInfos.Add(%q) failed: %v", name, err)
		}
	}
	fis.Freeze()
	return fis
}

func TestRestrictFields_FiltersFieldInfosAndProducers(t *testing.T) {
	seg0 := newTestFieldInfos(t, "title", "body", "id")
	seg1 := newTestFieldInfos(t, "title", "tags")
	merged := newTestFieldInfos(t, "title", "body", "id", "tags")

	in := &index.MergeState{
		FieldInfos:      []*index.FieldInfos{seg0, seg1},
		MergeFieldInfos: merged,
		MaxDocs:         []int{10, 4},
	}
	producers := []FieldsProducer{&stubFieldsProducer{}, nil}

	out, restricted, err := RestrictFields(in, producers, []string{"title", "tags"})
	if err != nil {
		t.Fatalf("RestrictFields returned error: %v", err)
	}
	if len(out.FieldInfos) != 2 {
		t.Fatalf("FieldInfos length = %d, want 2", len(out.FieldInfos))
	}

	wantSeg0 := map[string]bool{"title": true}
	if got := fieldNameSet(out.FieldInfos[0]); !mapEqual(got, wantSeg0) {
		t.Errorf("seg0 restricted = %v, want %v", got, wantSeg0)
	}
	wantSeg1 := map[string]bool{"title": true, "tags": true}
	if got := fieldNameSet(out.FieldInfos[1]); !mapEqual(got, wantSeg1) {
		t.Errorf("seg1 restricted = %v, want %v", got, wantSeg1)
	}
	wantMerged := map[string]bool{"title": true, "tags": true}
	if got := fieldNameSet(out.MergeFieldInfos); !mapEqual(got, wantMerged) {
		t.Errorf("merged restricted = %v, want %v", got, wantMerged)
	}

	// nil per-segment producer must stay nil; non-nil must be wrapped.
	if restricted[1] != nil {
		t.Errorf("nil producer at index 1 must remain nil, got %T", restricted[1])
	}
	if _, ok := restricted[0].(*filterFieldsProducer); !ok {
		t.Fatalf("producer at index 0 = %T, want *filterFieldsProducer", restricted[0])
	}

	// Shared, immutable slots are passed through unchanged.
	if out.MaxDocs[0] != 10 || out.MaxDocs[1] != 4 {
		t.Errorf("MaxDocs not preserved: got %v", out.MaxDocs)
	}
}

func TestRestrictFields_NilMergeStateReturnsError(t *testing.T) {
	if _, _, err := RestrictFields(nil, nil, []string{"x"}); err == nil {
		t.Fatalf("RestrictFields(nil, ...) = nil error, want non-nil")
	}
}

func TestRestrictFields_LengthMismatchReturnsError(t *testing.T) {
	in := &index.MergeState{
		FieldInfos:      []*index.FieldInfos{newTestFieldInfos(t, "a")},
		MergeFieldInfos: newTestFieldInfos(t, "a"),
	}
	_, _, err := RestrictFields(in, nil, []string{"a"})
	if err == nil {
		t.Fatalf("RestrictFields with mismatched lengths = nil error, want non-nil")
	}
	if !strings.Contains(err.Error(), "length") {
		t.Errorf("error message %q does not mention length", err.Error())
	}
}

func TestFilterFieldsProducer_AllowsListedAndRejectsOthers(t *testing.T) {
	inner := &stubFieldsProducer{}
	fp := newFilterFieldsProducer(inner, newFieldAllowSet([]string{"body"}))

	if _, err := fp.Terms("body"); err != nil {
		t.Fatalf("Terms(body) = %v, want nil", err)
	}
	if len(inner.visited) != 1 || inner.visited[0] != "body" {
		t.Errorf("inner.visited = %v, want [body]", inner.visited)
	}

	_, err := fp.Terms("forbidden")
	if err == nil {
		t.Fatalf("Terms(forbidden) = nil error, want non-nil")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("error %q does not name the forbidden field", err.Error())
	}
	// Inner must not have been touched for the forbidden field.
	if len(inner.visited) != 1 {
		t.Errorf("inner was called for forbidden field; visited=%v", inner.visited)
	}

	if err := fp.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if inner.closed != 1 {
		t.Errorf("inner.closed = %d, want 1", inner.closed)
	}
}

func TestFilterFieldsProducer_PropagatesInnerError(t *testing.T) {
	want := errors.New("boom")
	fp := newFilterFieldsProducer(&failingFieldsProducer{err: want}, newFieldAllowSet([]string{"f"}))
	_, err := fp.Terms("f")
	if !errors.Is(err, want) {
		t.Fatalf("Terms err = %v, want %v", err, want)
	}
}

// failingFieldsProducer is the second test double: Terms returns the
// configured error so that we can assert error propagation through the
// filter (no wrapping is required by Lucene, the original simply
// returns in.terms(field)).
type failingFieldsProducer struct{ err error }

func (f *failingFieldsProducer) Terms(string) (index.Terms, error) { return nil, f.err }
func (f *failingFieldsProducer) Close() error                      { return nil }

func fieldNameSet(fis *index.FieldInfos) map[string]bool {
	if fis == nil {
		return nil
	}
	out := make(map[string]bool)
	it := fis.Iterator()
	for it.HasNext() {
		fi := it.Next()
		if fi != nil {
			out[fi.Name()] = true
		}
	}
	return out
}

func mapEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
