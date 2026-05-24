// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"errors"
	"testing"
)

// stubCutterFactory returns a fixed set of ordinals per document.
type stubCutterFactory struct {
	ordinals map[int][]int // doc → ordinals
}

func (f *stubCutterFactory) CreateLeafCutter(_ any) (LeafFacetCutterEx, error) {
	return &stubLeafCutter{ordinals: f.ordinals}, nil
}

type stubLeafCutter struct {
	ordinals map[int][]int
	cur      []int
	pos      int
}

func (c *stubLeafCutter) AdvanceExact(doc int) (bool, error) {
	ords, ok := c.ordinals[doc]
	c.cur = ords
	c.pos = 0
	return ok && len(ords) > 0, nil
}

func (c *stubLeafCutter) NextOrd() int {
	if c.pos >= len(c.cur) {
		return NoMoreFacetOrds
	}
	v := c.cur[c.pos]
	c.pos++
	return v
}

// stubRecorderFactory records (doc, ord) pairs.
type stubRecorderFactory struct {
	recorded []struct{ doc, ord int }
}

func (f *stubRecorderFactory) CreateLeafRecorder(_ any) (LeafFacetRecorderEx, error) {
	return &stubLeafRecorder{store: f}, nil
}

type stubLeafRecorder struct {
	store *stubRecorderFactory
}

func (r *stubLeafRecorder) Record(doc, ord int) error {
	r.store.recorded = append(r.store.recorded, struct{ doc, ord int }{doc, ord})
	return nil
}

// TestFacetFieldLeafCollector_Collect verifies that each matching (doc, ord)
// pair is forwarded to the recorder.
func TestFacetFieldLeafCollector_Collect(t *testing.T) {
	cutter := &stubCutterFactory{
		ordinals: map[int][]int{
			0: {1, 3},
			1: {2},
			2: {}, // no ordinals — AdvanceExact returns false
		},
	}
	recorder := &stubRecorderFactory{}
	lc := NewFacetFieldLeafCollector(cutter, recorder, "seg0")

	for doc := 0; doc <= 2; doc++ {
		if err := lc.Collect(doc); err != nil {
			t.Fatalf("Collect(%d): %v", doc, err)
		}
	}

	want := []struct{ doc, ord int }{{0, 1}, {0, 3}, {1, 2}}
	if len(recorder.recorded) != len(want) {
		t.Fatalf("recorded %d pairs; want %d: %v", len(recorder.recorded), len(want), recorder.recorded)
	}
	for i, w := range want {
		if recorder.recorded[i] != w {
			t.Errorf("recorded[%d] = %v; want %v", i, recorder.recorded[i], w)
		}
	}
}

// TestFacetFieldLeafCollector_LazyInit verifies the cutter is initialised
// lazily on the first Collect call.
func TestFacetFieldLeafCollector_LazyInit(t *testing.T) {
	called := false
	cf := &funcCutterFactory{fn: func(_ any) (LeafFacetCutterEx, error) {
		called = true
		return &stubLeafCutter{}, nil
	}}
	rf := &funcRecorderFactory{fn: func(_ any) (LeafFacetRecorderEx, error) {
		return &stubLeafRecorder{store: &stubRecorderFactory{}}, nil
	}}
	lc := NewFacetFieldLeafCollector(cf, rf, "seg0")
	if called {
		t.Error("expected lazy initialisation: cutter must not be created before Collect")
	}
	_ = lc.Collect(0)
	if !called {
		t.Error("expected cutter to be initialised on first Collect call")
	}
}

// TestFacetFieldLeafCollector_CutterError verifies error propagation from
// CreateLeafCutter.
func TestFacetFieldLeafCollector_CutterError(t *testing.T) {
	sentinelErr := errors.New("stub error")
	cf := &funcCutterFactory{fn: func(_ any) (LeafFacetCutterEx, error) {
		return nil, sentinelErr
	}}
	rf := &funcRecorderFactory{fn: func(_ any) (LeafFacetRecorderEx, error) {
		return &stubLeafRecorder{store: &stubRecorderFactory{}}, nil
	}}
	lc := NewFacetFieldLeafCollector(cf, rf, "seg0")
	if err := lc.Collect(0); !errors.Is(err, sentinelErr) {
		t.Errorf("expected sentinelErr, got %v", err)
	}
}

// helpers

type funcCutterFactory struct {
	fn func(any) (LeafFacetCutterEx, error)
}

func (f *funcCutterFactory) CreateLeafCutter(k any) (LeafFacetCutterEx, error) { return f.fn(k) }

type funcRecorderFactory struct {
	fn func(any) (LeafFacetRecorderEx, error)
}

func (f *funcRecorderFactory) CreateLeafRecorder(k any) (LeafFacetRecorderEx, error) {
	return f.fn(k)
}
