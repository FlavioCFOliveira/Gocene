// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.FacetFieldLeafCollector.
package facet

import (
	"github.com/FlavioCFOliveira/Gocene/sandbox/facet/cutters"
	"github.com/FlavioCFOliveira/Gocene/sandbox/facet/recorders"
)

// LeafFacetCutterEx is the per-segment cutter interface that
// FacetFieldLeafCollector requires.
//
// This extends the Gocene cutters.LeafFacetCutter stub with the Java API
// (advanceExact / nextOrd) that the sandbox facet pipeline uses. Callers
// that hold a full sandbox LeafFacetCutter must bridge to this interface.
//
// Mirrors org.apache.lucene.sandbox.facet.cutters.LeafFacetCutter.
type LeafFacetCutterEx interface {
	// AdvanceExact advances to doc and returns true when the doc has facet
	// values.
	AdvanceExact(doc int) (bool, error)
	// NextOrd returns the next facet ordinal for the current document, or
	// NoMoreFacetOrds when there are no more ordinals.
	NextOrd() int
}

// NoMoreFacetOrds is the sentinel returned by LeafFacetCutterEx.NextOrd
// when there are no more ordinals for the current document.
//
// Mirrors LeafFacetCutter.NO_MORE_ORDS.
const NoMoreFacetOrds = -1

// LeafFacetRecorderEx is the per-segment recorder interface that
// FacetFieldLeafCollector requires.
//
// Mirrors org.apache.lucene.sandbox.facet.recorders.LeafFacetRecorder.
type LeafFacetRecorderEx interface {
	// Record records information for the given (doc, ordinal) pair.
	Record(doc, ord int) error
}

// FacetCutterFactory creates a LeafFacetCutterEx for the given segment.
// This mirrors FacetCutter.createLeafCutter(LeafReaderContext) but avoids
// a direct dependency on index.LeafReaderContext.
type FacetCutterFactory interface {
	// CreateLeafCutter creates a per-segment cutter. segmentKey is an
	// opaque value identifying the segment (e.g. its index within the
	// reader).
	CreateLeafCutter(segmentKey any) (LeafFacetCutterEx, error)
}

// FacetRecorderFactory creates a LeafFacetRecorderEx for the given segment.
// This mirrors FacetRecorder.getLeafRecorder(LeafReaderContext).
type FacetRecorderFactory interface {
	CreateLeafRecorder(segmentKey any) (LeafFacetRecorderEx, error)
}

// FacetFieldLeafCollector is the per-segment LeafCollector that, for each
// matching document, retrieves facet ordinals from a LeafFacetCutterEx and
// records them with a LeafFacetRecorderEx.
//
// Mirrors org.apache.lucene.sandbox.facet.FacetFieldLeafCollector.
//
// Deviations from Java:
//   - Uses factory interfaces instead of FacetCutter/FacetRecorder + context
//     to avoid depending on index.LeafReaderContext, which is not yet
//     fully wired in Gocene (backlog #2707).
//   - SetScorer is a no-op (same as Java's TODO comment).
type FacetFieldLeafCollector struct {
	cutterFactory   FacetCutterFactory
	recorderFactory FacetRecorderFactory
	segmentKey      any

	leafCutter   LeafFacetCutterEx
	leafRecorder LeafFacetRecorderEx

	// These fields satisfy the existing sandbox cutters/recorders package
	// types for callers that use the simplified Gocene stub API.
	_ cutters.LeafFacetCutter
	_ recorders.LeafFacetRecorder
}

// NewFacetFieldLeafCollector creates a FacetFieldLeafCollector for the
// segment identified by segmentKey.
//
// Mirrors FacetFieldLeafCollector(LeafReaderContext, FacetCutter,
// FacetRecorder).
func NewFacetFieldLeafCollector(
	cutterFactory FacetCutterFactory,
	recorderFactory FacetRecorderFactory,
	segmentKey any,
) *FacetFieldLeafCollector {
	return &FacetFieldLeafCollector{
		cutterFactory:   cutterFactory,
		recorderFactory: recorderFactory,
		segmentKey:      segmentKey,
	}
}

// Collect processes a single matching document.
//
// On the first call for a segment the leaf cutter and recorder are
// initialised lazily. For each facet ordinal returned by the cutter,
// Record is called on the recorder.
//
// Mirrors FacetFieldLeafCollector.collect(int).
func (c *FacetFieldLeafCollector) Collect(doc int) error {
	if c.leafCutter == nil {
		var err error
		c.leafCutter, err = c.cutterFactory.CreateLeafCutter(c.segmentKey)
		if err != nil {
			return err
		}
		c.leafRecorder, err = c.recorderFactory.CreateLeafRecorder(c.segmentKey)
		if err != nil {
			return err
		}
	}
	ok, err := c.leafCutter.AdvanceExact(doc)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	for {
		ord := c.leafCutter.NextOrd()
		if ord == NoMoreFacetOrds {
			break
		}
		if err := c.leafRecorder.Record(doc, ord); err != nil {
			return err
		}
	}
	return nil
}

// SetScorer is a no-op (see TODO in Java source).
//
// Mirrors FacetFieldLeafCollector.setScorer(Scorable).
func (c *FacetFieldLeafCollector) SetScorer(_ any) {}
