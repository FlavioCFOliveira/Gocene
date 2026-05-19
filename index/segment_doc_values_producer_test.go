// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// fakeDVProducer is a test double satisfying DocValuesProducer.
type fakeDVProducer struct {
	id              string
	checkErr        error
	getNumericErr   error
	checkIntegrity  int
	getSkipperCalls int
}

func (f *fakeDVProducer) GetNumeric(*FieldInfo) (NumericDocValues, error) {
	return nil, f.getNumericErr
}
func (f *fakeDVProducer) GetBinary(*FieldInfo) (BinaryDocValues, error) { return nil, nil }
func (f *fakeDVProducer) GetSorted(*FieldInfo) (SortedDocValues, error) { return nil, nil }
func (f *fakeDVProducer) GetSortedNumeric(*FieldInfo) (SortedNumericDocValues, error) {
	return nil, nil
}
func (f *fakeDVProducer) GetSortedSet(*FieldInfo) (SortedSetDocValues, error) { return nil, nil }
func (f *fakeDVProducer) GetSkipper(*FieldInfo) (DocValuesSkipper, error) {
	f.getSkipperCalls++
	return nil, nil
}
func (f *fakeDVProducer) CheckIntegrity() error { f.checkIntegrity++; return f.checkErr }
func (f *fakeDVProducer) Close() error          { return nil }

// fakeSegDV tracks producer lookups and DecRef invocations.
type fakeSegDV struct {
	// failOnGen: if non-nil and equal to the requested gen, returns that error.
	failOnGen *int64
	failErr   error

	// producerByGen records a stable producer per gen; new producers are
	// created lazily so the same gen always returns the same pointer.
	producerByGen map[int64]*fakeDVProducer

	lookups     []int64
	decRefCalls [][]int64
	decRefErr   error
}

func newFakeSegDV() *fakeSegDV {
	return &fakeSegDV{producerByGen: make(map[int64]*fakeDVProducer)}
}

func (f *fakeSegDV) GetDocValuesProducer(gen int64, _ *SegmentCommitInfo, _ store.Directory, _ *FieldInfos) (DocValuesProducer, error) {
	f.lookups = append(f.lookups, gen)
	if f.failOnGen != nil && *f.failOnGen == gen {
		return nil, f.failErr
	}
	if existing, ok := f.producerByGen[gen]; ok {
		return existing, nil
	}
	p := &fakeDVProducer{id: fmt.Sprintf("gen=%d", gen)}
	f.producerByGen[gen] = p
	return p, nil
}

func (f *fakeSegDV) DecRef(gens []int64) error {
	cp := make([]int64, len(gens))
	copy(cp, gens)
	f.decRefCalls = append(f.decRefCalls, cp)
	return f.decRefErr
}

// mustField builds a FieldInfo with the given number, name, doc-values type
// and generation. Test fields are kept minimal and only set what the producer
// touches.
func mustField(t *testing.T, name string, number int, dvType DocValuesType, gen int64) *FieldInfo {
	t.Helper()
	return NewFieldInfoBuilder(name, number).
		SetDocValuesType(dvType).
		SetDocValuesGen(gen).
		Build()
}

func mustInfos(t *testing.T, fields ...*FieldInfo) *FieldInfos {
	t.Helper()
	infos := NewFieldInfos()
	for _, fi := range fields {
		if err := infos.Add(fi); err != nil {
			t.Fatalf("FieldInfos.Add(%q): %v", fi.Name(), err)
		}
	}
	return infos
}

func newTestCommit() *SegmentCommitInfo {
	return NewSegmentCommitInfo(nil, 0, -1)
}

func TestSegmentDocValuesProducer_BaseAndUpdates(t *testing.T) {
	base1 := mustField(t, "f1", 0, DocValuesTypeNumeric, -1)
	base2 := mustField(t, "f2", 1, DocValuesTypeBinary, -1)
	upd := mustField(t, "f3", 2, DocValuesTypeSorted, 7)
	all := mustInfos(t, base1, base2, upd)
	core := mustInfos(t, base1, base2)

	seg := newFakeSegDV()
	p, err := NewSegmentDocValuesProducer(newTestCommit(), nil, core, all, seg)
	if err != nil {
		t.Fatalf("NewSegmentDocValuesProducer: %v", err)
	}

	// The base producer must be looked up exactly once for gen=-1, and the
	// update producer once for gen=7.
	if got, want := len(seg.lookups), 2; got != want {
		t.Fatalf("lookups: got %d, want %d (lookups=%v)", got, want, seg.lookups)
	}
	if seg.lookups[0] != -1 || seg.lookups[1] != 7 {
		t.Fatalf("lookup order: got %v, want [-1, 7]", seg.lookups)
	}

	// All three fields must be routed correctly.
	gens := p.Generations()
	if len(gens) != 2 || gens[0] != -1 || gens[1] != 7 {
		t.Fatalf("Generations: got %v, want [-1, 7]", gens)
	}

	// Both base fields share the same producer instance.
	if p.dvProducersByField[base1.Number()] != p.dvProducersByField[base2.Number()] {
		t.Fatalf("base fields must share the same producer")
	}
	// The updated field uses a different producer.
	if p.dvProducersByField[upd.Number()] == p.dvProducersByField[base1.Number()] {
		t.Fatalf("updated field must not share the base producer")
	}

	// CheckIntegrity fans out to every unique producer exactly once.
	if err := p.CheckIntegrity(); err != nil {
		t.Fatalf("CheckIntegrity: %v", err)
	}
	for gen, fp := range seg.producerByGen {
		if fp.checkIntegrity != 1 {
			t.Fatalf("producer gen=%d: CheckIntegrity calls = %d, want 1", gen, fp.checkIntegrity)
		}
	}

	// Lookup methods forward to the matching producer.
	if _, err := p.GetSkipper(upd); err != nil {
		t.Fatalf("GetSkipper: %v", err)
	}
	if seg.producerByGen[7].getSkipperCalls != 1 {
		t.Fatalf("update producer GetSkipper calls = %d, want 1", seg.producerByGen[7].getSkipperCalls)
	}

	// Close must return the sentinel.
	if err := p.Close(); !errors.Is(err, ErrSegmentDocValuesProducerClose) {
		t.Fatalf("Close error: got %v, want %v", err, ErrSegmentDocValuesProducerClose)
	}

	// String reflects the unique producer count.
	if got := p.String(); !strings.Contains(got, "producers=2") {
		t.Fatalf("String: got %q, want substring producers=2", got)
	}
}

func TestSegmentDocValuesProducer_SkipsNoneType(t *testing.T) {
	noneField := mustField(t, "skip-me", 5, DocValuesTypeNone, -1)
	keepField := mustField(t, "keep", 6, DocValuesTypeNumeric, -1)
	all := mustInfos(t, noneField, keepField)
	core := mustInfos(t, noneField, keepField)

	seg := newFakeSegDV()
	p, err := NewSegmentDocValuesProducer(newTestCommit(), nil, core, all, seg)
	if err != nil {
		t.Fatalf("NewSegmentDocValuesProducer: %v", err)
	}
	if _, ok := p.dvProducersByField[noneField.Number()]; ok {
		t.Fatalf("DocValuesType=None field must not be routed")
	}
	if _, ok := p.dvProducersByField[keepField.Number()]; !ok {
		t.Fatalf("doc-values field must be routed")
	}
	if len(seg.lookups) != 1 {
		t.Fatalf("lookups: got %d, want 1 (only base producer)", len(seg.lookups))
	}
}

func TestSegmentDocValuesProducer_DuplicateGenIsRejected(t *testing.T) {
	a := mustField(t, "a", 0, DocValuesTypeNumeric, 3)
	b := mustField(t, "b", 1, DocValuesTypeBinary, 3) // same gen, should fail
	all := mustInfos(t, a, b)
	core := mustInfos(t)

	seg := newFakeSegDV()
	_, err := NewSegmentDocValuesProducer(newTestCommit(), nil, core, all, seg)
	if err == nil {
		t.Fatalf("expected duplicate-gen error, got nil")
	}
	// DecRef must be invoked on failure to release the producer already
	// acquired for gen=3.
	if len(seg.decRefCalls) != 1 {
		t.Fatalf("DecRef calls: got %d, want 1", len(seg.decRefCalls))
	}
	if got := seg.decRefCalls[0]; len(got) != 1 || got[0] != 3 {
		t.Fatalf("DecRef args: got %v, want [3]", got)
	}
}

func TestSegmentDocValuesProducer_CleanupOnLookupFailure(t *testing.T) {
	base := mustField(t, "base", 0, DocValuesTypeNumeric, -1)
	upd := mustField(t, "upd", 1, DocValuesTypeBinary, 4)
	all := mustInfos(t, base, upd)
	core := mustInfos(t, base)

	failGen := int64(4)
	wantErr := errors.New("boom")
	seg := newFakeSegDV()
	seg.failOnGen = &failGen
	seg.failErr = wantErr

	_, err := NewSegmentDocValuesProducer(newTestCommit(), nil, core, all, seg)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want wrap of %v", err, wantErr)
	}
	if len(seg.decRefCalls) != 1 {
		t.Fatalf("DecRef calls: got %d, want 1", len(seg.decRefCalls))
	}
	got := seg.decRefCalls[0]
	if len(got) != 1 || got[0] != -1 {
		t.Fatalf("DecRef args: got %v, want [-1] (only the base gen was acquired)", got)
	}
}

func TestSegmentDocValuesProducer_CleanupErrorIsJoined(t *testing.T) {
	upd := mustField(t, "upd", 0, DocValuesTypeBinary, 9)
	all := mustInfos(t, upd)
	core := mustInfos(t)

	failGen := int64(9)
	primary := errors.New("primary")
	cleanup := errors.New("cleanup")
	seg := newFakeSegDV()
	seg.failOnGen = &failGen
	seg.failErr = primary
	seg.decRefErr = cleanup

	_, err := NewSegmentDocValuesProducer(newTestCommit(), nil, core, all, seg)
	if !errors.Is(err, primary) || !errors.Is(err, cleanup) {
		t.Fatalf("err = %v, must wrap both %v and %v", err, primary, cleanup)
	}
}

func TestSegmentDocValuesProducer_UnknownFieldErrors(t *testing.T) {
	base := mustField(t, "base", 0, DocValuesTypeNumeric, -1)
	all := mustInfos(t, base)
	core := mustInfos(t, base)
	seg := newFakeSegDV()
	p, err := NewSegmentDocValuesProducer(newTestCommit(), nil, core, all, seg)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	ghost := mustField(t, "ghost", 99, DocValuesTypeNumeric, -1)
	if _, err := p.GetNumeric(ghost); err == nil {
		t.Fatalf("expected error for unknown field")
	}
	if _, err := p.GetNumeric(nil); err == nil {
		t.Fatalf("expected error for nil field")
	}
}

func TestSegmentDocValuesProducer_CheckIntegrityJoinsErrors(t *testing.T) {
	a := mustField(t, "a", 0, DocValuesTypeNumeric, -1)
	b := mustField(t, "b", 1, DocValuesTypeBinary, 1)
	all := mustInfos(t, a, b)
	core := mustInfos(t, a)
	seg := newFakeSegDV()
	p, err := NewSegmentDocValuesProducer(newTestCommit(), nil, core, all, seg)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	errBase := errors.New("base bad")
	errUpd := errors.New("upd bad")
	seg.producerByGen[-1].checkErr = errBase
	seg.producerByGen[1].checkErr = errUpd
	err = p.CheckIntegrity()
	if !errors.Is(err, errBase) || !errors.Is(err, errUpd) {
		t.Fatalf("CheckIntegrity error = %v, must wrap both base and upd", err)
	}
}

func TestSegmentDocValuesProducer_NilArgumentsAreRejected(t *testing.T) {
	infos := mustInfos(t)
	seg := newFakeSegDV()
	cases := []struct {
		name string
		si   *SegmentCommitInfo
		core *FieldInfos
		all  *FieldInfos
		seg  SegmentDocValuesAccessor
	}{
		{"nil si", nil, infos, infos, seg},
		{"nil core", newTestCommit(), nil, infos, seg},
		{"nil all", newTestCommit(), infos, nil, seg},
		{"nil seg", newTestCommit(), infos, infos, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewSegmentDocValuesProducer(tc.si, nil, tc.core, tc.all, tc.seg); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}
