// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene60

import (
	"fmt"

	bcstore "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// Lucene60PointsReader reads point values previously written with Lucene60PointsWriter.
//
// Port of org.apache.lucene.backward_codecs.lucene60.Lucene60PointsReader
// (Lucene 10.5.0).
type Lucene60PointsReader struct {
	dataIn    store.IndexInput
	readState *codecs.SegmentReadState
	readers   map[int]index.PointValues
}

// NewLucene60PointsReader opens the points data and index files, reads the
// field-to-file-offset index, and builds a BKD reader per field.
func NewLucene60PointsReader(state *codecs.SegmentReadState) (*Lucene60PointsReader, error) {
	ctx := store.IOContext{Context: store.ContextRead}

	// ── Read index file (.dii) ──────────────────────────────────────────────
	indexFile := codecs.GetSegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, pointsIndexExtension)
	indexIn, err := bcstore.OpenChecksumInput(state.Directory, indexFile, ctx)
	if err != nil {
		return nil, fmt.Errorf("Lucene60PointsReader: open index %s: %w", indexFile, err)
	}

	fieldToFP := make(map[int]int64)
	readIndexErr := func() error {
		if _, err := codecs.CheckIndexHeader(
			indexIn,
			pointsMetaCodecName,
			pointsIndexVersionStart, pointsIndexVersionCurrent,
			state.SegmentInfo.GetID(),
			state.SegmentSuffix,
		); err != nil {
			return err
		}
		count, err := store.ReadVInt(indexIn)
		if err != nil {
			return err
		}
		for i := int32(0); i < count; i++ {
			fieldNumber, err := store.ReadVInt(indexIn)
			if err != nil {
				return err
			}
			fp, err := indexIn.ReadVLong()
			if err != nil {
				return err
			}
			fieldToFP[int(fieldNumber)] = fp
		}
		return checkFooterWithChecksum(indexIn)
	}()
	if cerr := indexIn.Close(); cerr != nil && readIndexErr == nil {
		readIndexErr = cerr
	}
	if readIndexErr != nil {
		return nil, fmt.Errorf("Lucene60PointsReader: read index: %w", readIndexErr)
	}

	// ── Open data file (.dim) ──────────────────────────────────────────────
	dataFile := codecs.GetSegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, pointsDataExtension)
	dataIn, err := bcstore.OpenInput(state.Directory, dataFile, ctx)
	if err != nil {
		return nil, fmt.Errorf("Lucene60PointsReader: open data %s: %w", dataFile, err)
	}

	if _, err := codecs.CheckIndexHeader(
		dataIn,
		pointsDataCodecName,
		pointsDataVersionStart, pointsDataVersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	); err != nil {
		_ = dataIn.Close()
		return nil, fmt.Errorf("Lucene60PointsReader: check data header: %w", err)
	}

	// Initialize BKD readers for each field.
	readers := make(map[int]index.PointValues)
	for fieldNumber, fp := range fieldToFP {
		if err := dataIn.SetPosition(fp); err != nil {
			_ = dataIn.Close()
			return nil, fmt.Errorf("Lucene60PointsReader: seek to fp %d for field %d: %w", fp, fieldNumber, err)
		}
		// Java: new BKDReader(dataIn, dataIn, dataIn)
		bkdReader, err := bkd.NewBKDReader(dataIn, dataIn, dataIn)
		if err != nil {
			_ = dataIn.Close()
			return nil, fmt.Errorf("Lucene60PointsReader: init BKDReader for field %d at fp %d: %w", fieldNumber, fp, err)
		}
		readers[fieldNumber] = &bkdPointValues{bkdReader: bkdReader}
	}

	return &Lucene60PointsReader{
		dataIn:    dataIn,
		readState: state,
		readers:   readers,
	}, nil
}

// GetValues returns the PointValues for the given field.
func (r *Lucene60PointsReader) GetValues(fieldName string) (index.PointValues, error) {
	fi := r.readState.FieldInfos.GetByName(fieldName)
	if fi == nil {
		return nil, fmt.Errorf("Lucene60PointsReader.GetValues: field %q is unrecognized", fieldName)
	}
	if fi.PointDimensionCount() == 0 {
		return nil, fmt.Errorf("Lucene60PointsReader.GetValues: field %q did not index point values", fieldName)
	}

	pv, ok := r.readers[fi.Number()]
	if !ok {
		return nil, nil
	}
	return pv, nil
}

// CheckIntegrity verifies the CRC32 checksum of the data file.
// GetMergeInstance returns an instance optimised for merging.
//
// Port of org.apache.lucene.codecs.PointsReader#getMergeInstance(), whose
// default body in Apache Lucene 10.5.0 is `return this;`. Lucene60PointsReader
// does not override it.
func (r *Lucene60PointsReader) GetMergeInstance() codecs.PointsReader { return r }

func (r *Lucene60PointsReader) CheckIntegrity() error {
	_, err := codecs.ChecksumEntireFile(r.dataIn)
	return err
}

// Close releases the data file handle.
func (r *Lucene60PointsReader) Close() error {
	if r.dataIn != nil {
		err := r.dataIn.Close()
		r.dataIn = nil
		r.readers = nil
		return err
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// BKD wrapper to satisfy codecs.PointValues
// ─────────────────────────────────────────────────────────────────────────────

type bkdPointValues struct {
	bkdReader *bkd.BKDReader
}

// Intersect walks the BKD tree, driving visitor for every matching cell and
// point. It bridges index.PointTreeIntersectVisitor (Compare returns an int)
// to util/bkd.IntersectVisitor (Compare returns a geo.Relation), exactly as
// the Lucene90 points reader does.
func (w *bkdPointValues) Intersect(visitor index.PointTreeIntersectVisitor) error {
	return w.bkdReader.Intersect(&bkdVisitorBridge{v: visitor})
}

// EstimatePointCount returns the BKDReader's estimate of how many points the
// visitor will match; a failing estimate is reported as 0.
func (w *bkdPointValues) EstimatePointCount(visitor index.PointTreeIntersectVisitor) int64 {
	count, err := w.bkdReader.EstimatePointCount(&bkdVisitorBridge{v: visitor})
	if err != nil || count < 0 {
		return 0
	}
	return count
}

// GetMinPackedValue returns the per-dimension minimum packed value across the
// tree. The error return matches index.PointValues; the BKDReader accessor
// never fails, so the error is always nil.
func (w *bkdPointValues) GetMinPackedValue() ([]byte, error) {
	return w.bkdReader.GetMinPackedValue(), nil
}

// GetMaxPackedValue returns the per-dimension maximum packed value.
func (w *bkdPointValues) GetMaxPackedValue() ([]byte, error) {
	return w.bkdReader.GetMaxPackedValue(), nil
}

// GetNumDimensions returns the number of indexed point dimensions.
func (w *bkdPointValues) GetNumDimensions() int {
	return w.bkdReader.GetNumDimensions()
}

// GetBytesPerDimension returns the number of bytes per dimension.
func (w *bkdPointValues) GetBytesPerDimension() int {
	return w.bkdReader.GetBytesPerDimension()
}

// GetDocCount returns the number of documents with at least one point value.
func (w *bkdPointValues) GetDocCount() int {
	return w.bkdReader.GetDocCount()
}

// GetDocCountWithValue returns the document count (BKD tracks doc count, not
// per-document value multiplicity).
func (w *bkdPointValues) GetDocCountWithValue() int64 {
	return int64(w.bkdReader.GetDocCount())
}

// GetValueCount returns the total number of indexed point values
// (PointValues.size()).
func (w *bkdPointValues) GetValueCount() int64 { return w.bkdReader.Size() }

// GetPointTree returns a fresh BKD PointTree cursor positioned at the root of
// the field's tree, rendering org.apache.lucene.index.PointValues#getPointTree().
func (w *bkdPointValues) GetPointTree() (bkd.PointTree, error) {
	return w.bkdReader.GetPointTree()
}

var _ index.PointValues = (*bkdPointValues)(nil)

// bkdVisitorBridge adapts an index.PointTreeIntersectVisitor (Compare returns
// an int in {0,1,2}) to a util/bkd.IntersectVisitor (Compare returns a
// geo.Relation). The int convention matches the Relation enum order, so the
// conversion is a direct cast.
type bkdVisitorBridge struct {
	v index.PointTreeIntersectVisitor
}

func (b *bkdVisitorBridge) Visit(docID int) error { return b.v.Visit(docID) }

func (b *bkdVisitorBridge) VisitByPackedValue(docID int, packedValue []byte) error {
	return b.v.VisitByPackedValue(docID, packedValue)
}

func (b *bkdVisitorBridge) Compare(minPackedValue, maxPackedValue []byte) geo.Relation {
	return geo.Relation(b.v.Compare(minPackedValue, maxPackedValue))
}

func (b *bkdVisitorBridge) Grow(count int) { b.v.Grow(count) }

var _ bkd.IntersectVisitor = (*bkdVisitorBridge)(nil)

var _ codecs.PointsReader = (*Lucene60PointsReader)(nil)
