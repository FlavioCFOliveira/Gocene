// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// trackingDVProducer is a test double satisfying DocValuesProducer that
// counts close invocations. It coexists with fakeDVProducer in the
// SegmentDocValuesProducer suite without colliding.
type trackingDVProducer struct {
	id         string
	closeCalls atomic.Int32
	closeErr   error
}

func (t *trackingDVProducer) GetNumeric(*FieldInfo) (NumericDocValues, error) { return nil, nil }
func (t *trackingDVProducer) GetBinary(*FieldInfo) (BinaryDocValues, error)   { return nil, nil }
func (t *trackingDVProducer) GetSorted(*FieldInfo) (SortedDocValues, error)   { return nil, nil }
func (t *trackingDVProducer) GetSortedNumeric(*FieldInfo) (SortedNumericDocValues, error) {
	return nil, nil
}
func (t *trackingDVProducer) GetSortedSet(*FieldInfo) (SortedSetDocValues, error) {
	return nil, nil
}
func (t *trackingDVProducer) GetSkipper(*FieldInfo) (DocValuesSkipper, error) { return nil, nil }
func (t *trackingDVProducer) CheckIntegrity() error                           { return nil }
func (t *trackingDVProducer) Close() error {
	t.closeCalls.Add(1)
	return t.closeErr
}

// newFactory returns a DocValuesProducerFactory whose successive invocations
// return distinct producers (one per gen). The factory records every call
// and optionally fails on a single gen.
type factoryHarness struct {
	mu        sync.Mutex
	calls     []int64
	failGen   *int64
	failErr   error
	producers map[int64]*trackingDVProducer
	nilOnGen  *int64
}

func newFactoryHarness() *factoryHarness {
	return &factoryHarness{producers: make(map[int64]*trackingDVProducer)}
}

func (h *factoryHarness) factory() DocValuesProducerFactory {
	return func(_ *SegmentCommitInfo, _ store.Directory, gen int64, _ *FieldInfos) (DocValuesProducer, error) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.calls = append(h.calls, gen)
		if h.failGen != nil && *h.failGen == gen {
			return nil, h.failErr
		}
		if h.nilOnGen != nil && *h.nilOnGen == gen {
			return nil, nil
		}
		p := &trackingDVProducer{id: fmt.Sprintf("gen=%d", gen)}
		h.producers[gen] = p
		return p, nil
	}
}

func TestSegmentDocValues_NilFactoryRejected(t *testing.T) {
	if _, err := NewSegmentDocValues(nil); err == nil {
		t.Fatalf("expected error for nil factory")
	}
}

func TestSegmentDocValues_GetCachesAndIncRefs(t *testing.T) {
	h := newFactoryHarness()
	sdv, err := NewSegmentDocValues(h.factory())
	if err != nil {
		t.Fatalf("NewSegmentDocValues: %v", err)
	}
	infos := NewFieldInfos()

	p1, err := sdv.GetDocValuesProducer(7, newTestCommit(), nil, infos)
	if err != nil {
		t.Fatalf("first GetDocValuesProducer: %v", err)
	}
	p2, err := sdv.GetDocValuesProducer(7, newTestCommit(), nil, infos)
	if err != nil {
		t.Fatalf("second GetDocValuesProducer: %v", err)
	}
	if p1 != p2 {
		t.Fatalf("expected the same producer instance to be returned on cache hit")
	}
	if got, want := len(h.calls), 1; got != want {
		t.Fatalf("factory invocations: got %d, want %d", got, want)
	}

	// Two outstanding references; first DecRef must NOT close.
	if err := sdv.DecRef([]int64{7}); err != nil {
		t.Fatalf("first DecRef: %v", err)
	}
	if got := h.producers[7].closeCalls.Load(); got != 0 {
		t.Fatalf("Close calls after first DecRef: got %d, want 0", got)
	}
	// Second DecRef must close and evict.
	if err := sdv.DecRef([]int64{7}); err != nil {
		t.Fatalf("second DecRef: %v", err)
	}
	if got := h.producers[7].closeCalls.Load(); got != 1 {
		t.Fatalf("Close calls after second DecRef: got %d, want 1", got)
	}
	// After eviction a new Get must re-invoke the factory.
	if _, err := sdv.GetDocValuesProducer(7, newTestCommit(), nil, infos); err != nil {
		t.Fatalf("Get after eviction: %v", err)
	}
	if got, want := len(h.calls), 2; got != want {
		t.Fatalf("factory invocations after eviction: got %d, want %d", got, want)
	}
}

func TestSegmentDocValues_FactoryErrorPropagates(t *testing.T) {
	h := newFactoryHarness()
	wantErr := errors.New("boom")
	g := int64(3)
	h.failGen = &g
	h.failErr = wantErr
	sdv, err := NewSegmentDocValues(h.factory())
	if err != nil {
		t.Fatalf("NewSegmentDocValues: %v", err)
	}
	if _, err := sdv.GetDocValuesProducer(3, newTestCommit(), nil, NewFieldInfos()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want wrap of %v", err, wantErr)
	}
	// Failed creation must not leave a phantom entry behind.
	if _, err := sdv.GetDocValuesProducer(3, newTestCommit(), nil, NewFieldInfos()); !errors.Is(err, wantErr) {
		t.Fatalf("second attempt should re-invoke factory (and still fail), got %v", err)
	}
	if got, want := len(h.calls), 2; got != want {
		t.Fatalf("factory invocations: got %d, want %d", got, want)
	}
}

func TestSegmentDocValues_FactoryNilProducerRejected(t *testing.T) {
	h := newFactoryHarness()
	g := int64(5)
	h.nilOnGen = &g
	sdv, err := NewSegmentDocValues(h.factory())
	if err != nil {
		t.Fatalf("NewSegmentDocValues: %v", err)
	}
	if _, err := sdv.GetDocValuesProducer(5, newTestCommit(), nil, NewFieldInfos()); err == nil {
		t.Fatalf("expected error when factory returns (nil, nil)")
	}
}

func TestSegmentDocValues_DecRefUnknownGenReportsError(t *testing.T) {
	sdv, err := NewSegmentDocValues(newFactoryHarness().factory())
	if err != nil {
		t.Fatalf("NewSegmentDocValues: %v", err)
	}
	err = sdv.DecRef([]int64{42})
	if err == nil || !strings.Contains(err.Error(), "gen=42") {
		t.Fatalf("DecRef unknown gen: got %v, want error mentioning gen=42", err)
	}
}

func TestSegmentDocValues_DecRefProcessesAllOnFailure(t *testing.T) {
	h := newFactoryHarness()
	sdv, err := NewSegmentDocValues(h.factory())
	if err != nil {
		t.Fatalf("NewSegmentDocValues: %v", err)
	}
	infos := NewFieldInfos()
	for _, g := range []int64{1, 2, 3} {
		if _, err := sdv.GetDocValuesProducer(g, newTestCommit(), nil, infos); err != nil {
			t.Fatalf("seed gen=%d: %v", g, err)
		}
	}
	// Force gen=2's close to fail; gens 1 and 3 must still close.
	h.producers[2].closeErr = errors.New("close-fail")

	err = sdv.DecRef([]int64{1, 2, 3})
	if err == nil {
		t.Fatalf("expected joined error from DecRef")
	}
	if got := h.producers[1].closeCalls.Load(); got != 1 {
		t.Fatalf("gen=1 close calls: got %d, want 1", got)
	}
	if got := h.producers[3].closeCalls.Load(); got != 1 {
		t.Fatalf("gen=3 close calls: got %d, want 1", got)
	}
}

func TestSegmentDocValues_SatisfiesAccessorInterface(t *testing.T) {
	// Compile-time assurance that the cache implements the contract the
	// SegmentDocValuesProducer companion file declares.
	var _ SegmentDocValuesAccessor = (*SegmentDocValues)(nil)
}

func TestSegmentDocValues_IntegrationWithProducer(t *testing.T) {
	// End-to-end: the producer must drive its acquisitions through the
	// cache, and a successful close path must release every reference.
	h := newFactoryHarness()
	sdv, err := NewSegmentDocValues(h.factory())
	if err != nil {
		t.Fatalf("NewSegmentDocValues: %v", err)
	}
	base := mustField(t, "f1", 0, DocValuesTypeNumeric, -1)
	upd := mustField(t, "f2", 1, DocValuesTypeBinary, 4)
	all := mustInfos(t, base, upd)
	core := mustInfos(t, base)

	p, err := NewSegmentDocValuesProducer(newTestCommit(), nil, core, all, sdv)
	if err != nil {
		t.Fatalf("NewSegmentDocValuesProducer: %v", err)
	}
	gens := p.Generations()
	if len(gens) != 2 {
		t.Fatalf("Generations: got %v, want 2 entries", gens)
	}
	if err := sdv.DecRef(gens); err != nil {
		t.Fatalf("DecRef: %v", err)
	}
	for g, prod := range h.producers {
		if prod.closeCalls.Load() != 1 {
			t.Fatalf("gen=%d close calls: got %d, want 1", g, prod.closeCalls.Load())
		}
	}
}

func TestSegmentDocValues_ConcurrentGetIsSafe(t *testing.T) {
	h := newFactoryHarness()
	sdv, err := NewSegmentDocValues(h.factory())
	if err != nil {
		t.Fatalf("NewSegmentDocValues: %v", err)
	}
	infos := NewFieldInfos()

	const goroutines = 16
	const itersPerG = 64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < itersPerG; j++ {
				if _, err := sdv.GetDocValuesProducer(0, newTestCommit(), nil, infos); err != nil {
					t.Errorf("Get: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	// Exactly one factory call, regardless of contention. Outstanding refs:
	// goroutines*itersPerG. Release them all.
	if got, want := len(h.calls), 1; got != want {
		t.Fatalf("factory invocations: got %d, want %d", got, want)
	}
	total := goroutines * itersPerG
	gens := make([]int64, total)
	for i := range gens {
		gens[i] = 0
	}
	if err := sdv.DecRef(gens); err != nil {
		t.Fatalf("DecRef: %v", err)
	}
	if got := h.producers[0].closeCalls.Load(); got != 1 {
		t.Fatalf("close calls: got %d, want 1", got)
	}
}
