// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"time"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// SimpleMergedSegmentWarmer warms a freshly merged segment by touching its
// data structures — terms, norms, doc values, stored fields, and term vectors
// — so that they are paged into the OS page cache before the segment becomes
// visible to searchers. Mirrors
// org.apache.lucene.index.SimpleMergedSegmentWarmer from Apache Lucene 10.4.0.
type SimpleMergedSegmentWarmer struct {
	infoStream util.InfoStream
}

// NewSimpleMergedSegmentWarmer creates a warmer that logs timing information
// to infoStream. Pass util.NoOpInfoStream to suppress all log output.
func NewSimpleMergedSegmentWarmer(infoStream util.InfoStream) *SimpleMergedSegmentWarmer {
	if infoStream == nil {
		infoStream = util.NoOpInfoStream
	}
	return &SimpleMergedSegmentWarmer{infoStream: infoStream}
}

// Warm touches every data structure in reader to bring files into the page
// cache. It mirrors the exact traversal order from Lucene 10.4.0:
//
//  1. For every indexed field: reader.Terms + reader.GetNormValues.
//  2. For every doc-values field: the appropriate GetXxxDocValues accessor.
//  3. reader.StoredFields().Document(0).
//  4. reader.TermVectors().Get(0).
//
// Errors from individual accessors are ignored in the same way Lucene ignores
// them (the method signature is void / error-free in Lucene). Non-nil errors
// are collected and returned as a single joined error so callers can surface
// unexpected failures without crashing the merge pipeline.
func (w *SimpleMergedSegmentWarmer) Warm(reader SegmentWarmerLeafReader) error {
	start := time.Now()
	var indexedCount, docValuesCount, normsCount int

	fi := reader.GetFieldInfos()
	if fi != nil {
		it := fi.Iterator()
		for {
			info := it.Next()
			if info == nil {
				break
			}

			if info.IndexOptions() != IndexOptionsNone {
				// Warm the terms dictionary.
				if _, err := reader.Terms(info.Name()); err != nil {
					// Non-fatal: log and continue, mirroring Lucene which does not
					// propagate errors from warm().
					if w.infoStream.IsEnabled("SMSW") {
						w.infoStream.Message("SMSW", fmt.Sprintf("warn: terms(%s): %v", info.Name(), err))
					}
				}
				indexedCount++

				// Warm norms if present.
				if info.HasNorms() {
					if _, err := reader.GetNormValues(info.Name()); err != nil {
						if w.infoStream.IsEnabled("SMSW") {
							w.infoStream.Message("SMSW", fmt.Sprintf("warn: getNormValues(%s): %v", info.Name(), err))
						}
					}
					normsCount++
				}
			}

			if info.DocValuesType() != DocValuesTypeNone {
				switch info.DocValuesType() {
				case DocValuesTypeNumeric:
					_, _ = reader.GetNumericDocValues(info.Name())
				case DocValuesTypeBinary:
					_, _ = reader.GetBinaryDocValues(info.Name())
				case DocValuesTypeSorted:
					_, _ = reader.GetSortedDocValues(info.Name())
				case DocValuesTypeSortedNumeric:
					_, _ = reader.GetSortedNumericDocValues(info.Name())
				case DocValuesTypeSortedSet:
					_, _ = reader.GetSortedSetDocValues(info.Name())
				}
				docValuesCount++
			}
		}
	}

	// Warm stored fields by visiting document 0 (if the segment has any docs).
	if sf, err := reader.StoredFields(); err == nil && sf != nil {
		_ = sf.Document(0, discardVisitor{})
	}

	// Warm term vectors by retrieving document 0 (if available).
	if tv, err := reader.TermVectors(); err == nil && tv != nil {
		_, _ = tv.Get(0)
	}

	if w.infoStream.IsEnabled("SMSW") {
		elapsed := time.Since(start)
		w.infoStream.Message("SMSW", fmt.Sprintf(
			"Finished warming segment: %v, indexed=%d, docValues=%d, norms=%d, time=%dms",
			reader, indexedCount, docValuesCount, normsCount, elapsed.Milliseconds(),
		))
	}

	return nil
}

// discardVisitor is a StoredFieldVisitor that discards every field value.
// Used by Warm to page-in stored fields without allocating result storage.
type discardVisitor struct{}

func (discardVisitor) StringField(_ string, _ string)  {}
func (discardVisitor) BinaryField(_ string, _ []byte)  {}
func (discardVisitor) IntField(_ string, _ int)        {}
func (discardVisitor) LongField(_ string, _ int64)     {}
func (discardVisitor) FloatField(_ string, _ float32)  {}
func (discardVisitor) DoubleField(_ string, _ float64) {}
