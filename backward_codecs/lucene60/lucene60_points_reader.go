// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene60

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/codecs"
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
	readers   map[int]codecs.PointValues
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
			fp, err := store.ReadVLong(indexIn)
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
	readers := make(map[int]codecs.PointValues)
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
func (r *Lucene60PointsReader) GetValues(fieldName string) (codecs.PointValues, error) {
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

func (w *bkdPointValues) Intersect(visitor codecs.IntersectVisitor) error {
	return w.bkdReader.Intersect(visitor)
}

func (w *bkdPointValues) EstimatePointCount(visitor codecs.IntersectVisitor) int64 {
	count, err := w.bkdReader.EstimatePointCount(visitor)
	if err != nil {
		// In a real scenario, we should log this, but the interface requires int64.
		return 0
	}
	return count
}

func (w *bkdPointValues) GetMinPackedValue() []byte {
	return w.bkdReader.GetMinPackedValue()
}

func (w *bkdPointValues) GetMaxPackedValue() []byte {
	return w.bkdReader.GetMaxPackedValue()
}

func (w *bkdPointValues) GetNumDimensions() int {
	return w.bkdReader.GetNumDimensions()
}

func (w *bkdPointValues) GetBytesPerDimension() int {
	return w.bkdReader.GetBytesPerDimension()
}

func (w *bkdPointValues) GetDocCount() int {
	return w.bkdReader.GetDocCount()
}

var _ codecs.PointsReader = (*Lucene60PointsReader)(nil)
