// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SegmentDocValues manages the [DocValuesProducer] instances held by a
// [SegmentReader] and keeps track of their reference counts.
//
// This is the Go port of Lucene's package-private
// org.apache.lucene.index.SegmentDocValues from Apache Lucene 10.4.0.
//
// Divergences from the Java reference:
//   - The Java class resolves the per-segment [DocValuesProducer] through
//     si.info.getCodec().docValuesFormat().fieldsProducer(srs). The Gocene
//     [Codec] interface lives in the index package and does not expose
//     DocValuesFormat (the format types live in the codecs package and
//     importing them here would create a cycle). The producer factory is
//     therefore injected at construction as [DocValuesProducerFactory], the
//     same shape the codecs package implements when wiring SegmentReader.
//   - The Java map type is LongObjectHashMap; the Go port uses a built-in
//     map[int64]*util.RefCount[DocValuesProducer] with identical semantics.
//   - Java's synchronized methods are mirrored by a sync.Mutex held for the
//     full scope of GetDocValuesProducer/DecRef, including the release
//     callback that mutates the map (matching the synchronized block inside
//     newDocValuesProducer.release).
//
// SegmentDocValues is safe for concurrent use.
type SegmentDocValues struct {
	mu           sync.Mutex
	genDVP       map[int64]*util.RefCount[DocValuesProducer]
	producerFunc DocValuesProducerFactory
}

// DocValuesProducerFactory is the dependency injected into [SegmentDocValues]
// to materialise a [DocValuesProducer] for a given generation. It mirrors the
// si.info.getCodec().docValuesFormat().fieldsProducer(srs) chain from the
// Java reference without forcing the index package to depend on codecs.
//
// gen is the doc-values generation: -1 for the base producer, or the
// per-update generation otherwise.
//
// The implementation is expected to build a SegmentReadState equivalent to
// Lucene's (using IOContextDefault and, for gen != -1, the segment's own
// directory and a base-MaxRadix segment suffix) and return the resulting
// producer.
type DocValuesProducerFactory func(
	si *SegmentCommitInfo,
	dir store.Directory,
	gen int64,
	infos *FieldInfos,
) (DocValuesProducer, error)

// NewSegmentDocValues constructs an empty SegmentDocValues that resolves
// producers through producerFunc. producerFunc must not be nil.
func NewSegmentDocValues(producerFunc DocValuesProducerFactory) (*SegmentDocValues, error) {
	if producerFunc == nil {
		return nil, fmt.Errorf("SegmentDocValues: producerFunc is nil")
	}
	return &SegmentDocValues{
		genDVP:       make(map[int64]*util.RefCount[DocValuesProducer]),
		producerFunc: producerFunc,
	}, nil
}

// newDocValuesProducer is the Go port of Lucene's
// SegmentDocValues.newDocValuesProducer. It is called with sdv.mu already
// held by the caller. The returned RefCount starts with an implicit
// reference count of 1 (see [util.NewRefCount]).
//
// The release closure mirrors the synchronized block in the Java reference:
// it closes the underlying producer and removes the entry from the map.
// Unlike Java's re-entrant synchronized, sync.Mutex is not re-entrant; the
// only call site that triggers the release path is [SegmentDocValues.DecRef],
// which already holds sdv.mu, so the closure mutates the map directly
// without re-locking. This invariant is enforced by routing all producer
// acquisitions through GetDocValuesProducer/DecRef.
func (sdv *SegmentDocValues) newDocValuesProducer(
	si *SegmentCommitInfo,
	dir store.Directory,
	gen int64,
	infos *FieldInfos,
) (*util.RefCount[DocValuesProducer], error) {
	producer, err := sdv.producerFunc(si, dir, gen, infos)
	if err != nil {
		return nil, err
	}
	if producer == nil {
		return nil, fmt.Errorf("SegmentDocValues: producer factory returned nil for gen=%d", gen)
	}
	rc := util.NewRefCount[DocValuesProducer](producer, func(p DocValuesProducer) error {
		// Caller (DecRef) holds sdv.mu; map mutation is safe here.
		delete(sdv.genDVP, gen)
		return p.Close()
	})
	return rc, nil
}

// GetDocValuesProducer returns the [DocValuesProducer] for the given
// generation, instantiating it through the injected factory on first use and
// incrementing its reference count on subsequent calls.
//
// This is the Go port of Lucene's synchronized getDocValuesProducer.
func (sdv *SegmentDocValues) GetDocValuesProducer(
	gen int64,
	si *SegmentCommitInfo,
	dir store.Directory,
	infos *FieldInfos,
) (DocValuesProducer, error) {
	sdv.mu.Lock()
	defer sdv.mu.Unlock()

	if rc, ok := sdv.genDVP[gen]; ok {
		rc.IncRef()
		return rc.Get(), nil
	}
	rc, err := sdv.newDocValuesProducer(si, dir, gen, infos)
	if err != nil {
		return nil, err
	}
	sdv.genDVP[gen] = rc
	return rc.Get(), nil
}

// DecRef decrements the reference count of every generation listed in
// dvProducersGens. Mirrors Lucene's synchronized decRef plus the
// IOUtils.applyToAll contract: every generation is processed even when a
// release fails, and the collected errors are joined into the returned
// error so callers see the full picture.
//
// Missing generations are reported as errors (the Java assert dvp != null
// is promoted to a runtime check because the caller — typically the
// SegmentReader cleanup path — should never see this).
func (sdv *SegmentDocValues) DecRef(dvProducersGens []int64) error {
	sdv.mu.Lock()
	defer sdv.mu.Unlock()

	var joined error
	for _, gen := range dvProducersGens {
		rc, ok := sdv.genDVP[gen]
		if !ok {
			joined = errors.Join(joined,
				fmt.Errorf("SegmentDocValues.DecRef: gen=%d not tracked", gen),
			)
			continue
		}
		if err := rc.DecRef(); err != nil {
			joined = errors.Join(joined,
				fmt.Errorf("SegmentDocValues.DecRef gen=%d: %w", gen, err),
			)
		}
	}
	return joined
}
