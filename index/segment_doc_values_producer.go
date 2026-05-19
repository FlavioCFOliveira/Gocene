// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentDocValuesProducer encapsulates multiple doc-values producers behind
// a single producer facade, so a segment that has received doc-values updates
// can be read as if it carried only one producer.
//
// This is the Go port of Lucene's package-private
// org.apache.lucene.index.SegmentDocValuesProducer from Apache Lucene 10.4.0.
//
// Divergences from the Java reference:
//   - The Lucene class extends DocValuesProducer (an abstract class). Gocene
//     models DocValuesProducer-shaped types structurally in the index package
//     (see [EmptyDocValuesProducer]); this type satisfies that same shape.
//   - SegmentDocValues has not been ported yet, so the collaborator is
//     consumed through the [SegmentDocValuesAccessor] interface declared
//     below. When SegmentDocValues lands it must satisfy this interface.
//   - The Java close() throws UnsupportedOperationException because ref
//     counting is handled separately. The Go port mirrors that contract by
//     returning [ErrSegmentDocValuesProducerClose].
//   - The Java field number map uses an IntObjectHashMap; the Go port uses a
//     built-in map[int]DocValuesProducer with the same semantics.
type SegmentDocValuesProducer struct {
	dvProducersByField map[int]DocValuesProducer
	dvProducers        []DocValuesProducer
	dvGens             []int64
}

// DocValuesProducer is the structural contract for index-package doc-values
// producers. It mirrors the surface of
// org.apache.lucene.codecs.DocValuesProducer and is satisfied by
// [EmptyDocValuesProducer] as well as the per-format producers wired through
// SegmentCoreReaders.
//
// Gocene declares the contract locally so that this file stays self-contained
// while the canonical interface continues to live in the codecs package.
type DocValuesProducer interface {
	GetNumeric(field *FieldInfo) (NumericDocValues, error)
	GetBinary(field *FieldInfo) (BinaryDocValues, error)
	GetSorted(field *FieldInfo) (SortedDocValues, error)
	GetSortedNumeric(field *FieldInfo) (SortedNumericDocValues, error)
	GetSortedSet(field *FieldInfo) (SortedSetDocValues, error)
	GetSkipper(field *FieldInfo) (DocValuesSkipper, error)
	CheckIntegrity() error
	Close() error
}

// SegmentDocValuesAccessor is the minimal surface
// [NewSegmentDocValuesProducer] requires from the SegmentDocValues cache. It
// mirrors the two methods of org.apache.lucene.index.SegmentDocValues that
// SegmentDocValuesProducer touches: producer lookup by generation and
// decrement-on-failure.
type SegmentDocValuesAccessor interface {
	// GetDocValuesProducer returns the per-generation producer for the given
	// segment commit. The implementation is expected to ref-count producers
	// internally.
	GetDocValuesProducer(
		gen int64,
		si *SegmentCommitInfo,
		dir store.Directory,
		infos *FieldInfos,
	) (DocValuesProducer, error)

	// DecRef releases references for the given generations. It is invoked
	// during the cleanup path when construction fails partway through.
	DecRef(gens []int64) error
}

// ErrSegmentDocValuesProducerClose is returned by [SegmentDocValuesProducer.Close]
// because reference tracking is owned by SegmentDocValues, not by this
// facade. Mirrors the UnsupportedOperationException thrown by the Java code.
var ErrSegmentDocValuesProducerClose = errors.New(
	"SegmentDocValuesProducer.Close: separate ref tracking owns the lifecycle",
)

// NewSegmentDocValuesProducer builds a facade producer that fans field
// lookups out to the appropriate per-generation producer.
//
// si is the commit point, dir is the segment directory, coreInfos are the
// FieldInfos the base producer originally wrote, allInfos are the FieldInfos
// including updated fields, and segDocValues is the producer cache.
//
// If construction fails partway through, every successfully acquired
// generation is released through segDocValues.DecRef before the error
// propagates. Cleanup errors are joined to the original failure via
// [errors.Join].
func NewSegmentDocValuesProducer(
	si *SegmentCommitInfo,
	dir store.Directory,
	coreInfos *FieldInfos,
	allInfos *FieldInfos,
	segDocValues SegmentDocValuesAccessor,
) (*SegmentDocValuesProducer, error) {
	if si == nil {
		return nil, fmt.Errorf("SegmentDocValuesProducer: si is nil")
	}
	if coreInfos == nil {
		return nil, fmt.Errorf("SegmentDocValuesProducer: coreInfos is nil")
	}
	if allInfos == nil {
		return nil, fmt.Errorf("SegmentDocValuesProducer: allInfos is nil")
	}
	if segDocValues == nil {
		return nil, fmt.Errorf("SegmentDocValuesProducer: segDocValues is nil")
	}

	p := &SegmentDocValuesProducer{
		dvProducersByField: make(map[int]DocValuesProducer),
	}
	// dvProducers behaves like Lucene's IdentityHashMap-backed set: it tracks
	// unique producer instances by pointer identity. Linear scans are fine
	// because the cardinality equals the number of distinct generations,
	// which is small in practice.
	seenProducer := func(target DocValuesProducer) bool {
		for _, existing := range p.dvProducers {
			if sameProducer(existing, target) {
				return true
			}
		}
		return false
	}

	var baseProducer DocValuesProducer
	build := func() error {
		it := allInfos.Iterator()
		for it.HasNext() {
			fi := it.Next()
			if fi.DocValuesType() == DocValuesTypeNone {
				continue
			}
			gen := fi.DocValuesGen()
			if gen == -1 {
				if baseProducer == nil {
					// The base producer keeps the original FieldInfos it wrote.
					base, err := segDocValues.GetDocValuesProducer(gen, si, dir, coreInfos)
					if err != nil {
						return err
					}
					baseProducer = base
					p.dvGens = append(p.dvGens, gen)
					p.dvProducers = append(p.dvProducers, base)
				}
				p.dvProducersByField[fi.Number()] = baseProducer
				continue
			}

			// A per-field generation producer sees only the one FieldInfo it
			// wrote. The Lucene code asserts !dvGens.contains(docValuesGen);
			// we promote that to a real check to surface caller bugs early.
			if containsGen(p.dvGens, gen) {
				return fmt.Errorf(
					"SegmentDocValuesProducer: duplicate docValuesGen %d for field %q",
					gen, fi.Name(),
				)
			}
			singleton, err := newFieldInfosSingleton(fi)
			if err != nil {
				return err
			}
			dvp, err := segDocValues.GetDocValuesProducer(gen, si, dir, singleton)
			if err != nil {
				return err
			}
			p.dvGens = append(p.dvGens, gen)
			p.dvProducers = append(p.dvProducers, dvp)
			p.dvProducersByField[fi.Number()] = dvp
			_ = seenProducer // retained for documentation; identity is guaranteed
			// because each generation produces a distinct instance.
		}
		return nil
	}

	if err := build(); err != nil {
		if decErr := segDocValues.DecRef(p.dvGens); decErr != nil {
			return nil, errors.Join(err, decErr)
		}
		return nil, err
	}
	return p, nil
}

// newFieldInfosSingleton builds a FieldInfos containing a single FieldInfo,
// mirroring Lucene's `new FieldInfos(new FieldInfo[]{fi})` for per-generation
// producers.
func newFieldInfosSingleton(fi *FieldInfo) (*FieldInfos, error) {
	out := NewFieldInfos()
	if err := out.Add(fi); err != nil {
		return nil, fmt.Errorf("SegmentDocValuesProducer: build singleton FieldInfos: %w", err)
	}
	return out, nil
}

// containsGen reports whether gen is already tracked.
func containsGen(gens []int64, gen int64) bool {
	for _, g := range gens {
		if g == gen {
			return true
		}
	}
	return false
}

// sameProducer reports identity equality of two producers, matching the
// IdentityHashMap-backed set Lucene uses to dedupe producers.
func sameProducer(a, b DocValuesProducer) bool {
	// Comparing interface values with == compares both the dynamic type and
	// the underlying pointer for pointer-receiver implementations, which is
	// the Go equivalent of Java reference equality used by IdentityHashMap.
	return a == b
}

// GetNumeric returns the NumericDocValues for the given field by dispatching
// to the producer that owns it.
func (p *SegmentDocValuesProducer) GetNumeric(field *FieldInfo) (NumericDocValues, error) {
	dvp, err := p.producerFor(field)
	if err != nil {
		return nil, err
	}
	return dvp.GetNumeric(field)
}

// GetBinary returns the BinaryDocValues for the given field.
func (p *SegmentDocValuesProducer) GetBinary(field *FieldInfo) (BinaryDocValues, error) {
	dvp, err := p.producerFor(field)
	if err != nil {
		return nil, err
	}
	return dvp.GetBinary(field)
}

// GetSorted returns the SortedDocValues for the given field.
func (p *SegmentDocValuesProducer) GetSorted(field *FieldInfo) (SortedDocValues, error) {
	dvp, err := p.producerFor(field)
	if err != nil {
		return nil, err
	}
	return dvp.GetSorted(field)
}

// GetSortedNumeric returns the SortedNumericDocValues for the given field.
func (p *SegmentDocValuesProducer) GetSortedNumeric(field *FieldInfo) (SortedNumericDocValues, error) {
	dvp, err := p.producerFor(field)
	if err != nil {
		return nil, err
	}
	return dvp.GetSortedNumeric(field)
}

// GetSortedSet returns the SortedSetDocValues for the given field.
func (p *SegmentDocValuesProducer) GetSortedSet(field *FieldInfo) (SortedSetDocValues, error) {
	dvp, err := p.producerFor(field)
	if err != nil {
		return nil, err
	}
	return dvp.GetSortedSet(field)
}

// GetSkipper returns the DocValuesSkipper for the given field.
func (p *SegmentDocValuesProducer) GetSkipper(field *FieldInfo) (DocValuesSkipper, error) {
	dvp, err := p.producerFor(field)
	if err != nil {
		return nil, err
	}
	return dvp.GetSkipper(field)
}

// CheckIntegrity calls CheckIntegrity on every unique underlying producer.
// It returns the first non-nil error, joined with any subsequent failures so
// callers see the complete picture.
func (p *SegmentDocValuesProducer) CheckIntegrity() error {
	var joined error
	for _, dvp := range p.dvProducers {
		if err := dvp.CheckIntegrity(); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

// Close always returns [ErrSegmentDocValuesProducerClose] because the facade
// does not own the lifecycle of its underlying producers; SegmentDocValues
// holds the reference counts.
func (p *SegmentDocValuesProducer) Close() error {
	return ErrSegmentDocValuesProducerClose
}

// String returns a debug representation matching Lucene's toString.
func (p *SegmentDocValuesProducer) String() string {
	return fmt.Sprintf("SegmentDocValuesProducer(producers=%d)", len(p.dvProducers))
}

// Generations returns a copy of the tracked generations. Primarily intended
// for tests and callers that need to release references via SegmentDocValues.
func (p *SegmentDocValuesProducer) Generations() []int64 {
	out := make([]int64, len(p.dvGens))
	copy(out, p.dvGens)
	return out
}

// producerFor returns the producer responsible for the given field, or an
// error if no producer was registered for it. Mirrors the Java assert.
func (p *SegmentDocValuesProducer) producerFor(field *FieldInfo) (DocValuesProducer, error) {
	if field == nil {
		return nil, fmt.Errorf("SegmentDocValuesProducer: field is nil")
	}
	dvp, ok := p.dvProducersByField[field.Number()]
	if !ok || dvp == nil {
		return nil, fmt.Errorf(
			"SegmentDocValuesProducer: no producer registered for field %q (#%d)",
			field.Name(), field.Number(),
		)
	}
	return dvp, nil
}
