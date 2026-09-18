// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// SimpleTextPointsReader reads point values from the plain-text ".dim" file
// written by SimpleTextPointsWriter and SimpleTextBKDWriter.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextPointsReader
// (Lucene 10.4.0).
type SimpleTextPointsReader struct {
	dataIn    store.IndexInput
	readState *codecs.SegmentReadState
	readers   map[string]*SimpleTextBKDReader
	scratch   *util.BytesRefBuilder
}

// NewSimpleTextPointsReader opens the points data and index files, reads the
// field-to-file-offset index, and builds a SimpleTextBKDReader per field.
//
// Port of SimpleTextPointsReader(SegmentReadState).
func NewSimpleTextPointsReader(state *codecs.SegmentReadState) (*SimpleTextPointsReader, error) {
	// -----------------------------------------------------------------------
	// 1. Read the index file (.dii) to build field → data-file-offset map.
	// -----------------------------------------------------------------------
	indexFileName := store.SegmentFileName(
		state.SegmentInfo.Name(),
		state.SegmentSuffix,
		PointIndexExtension,
	)
	rawIndex, err := state.Directory.OpenInput(indexFileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("SimpleTextPointsReader: open index %s: %w", indexFileName, err)
	}
	fieldToOffset := make(map[string]int64)
	{
		in := store.NewChecksumIndexInput(rawIndex)
		defer func() { _ = in.Close() }()

		scratch := util.NewBytesRefBuilder()
		readFn := func() error { return stReadLine(in, scratch) }
		getLine := func() []byte { return scratch.Bytes()[:scratch.Length()] }

		if err := readFn(); err != nil {
			return nil, fmt.Errorf("SimpleTextPointsReader: read index field count: %w", err)
		}
		line := getLine()
		count, err := stParseInt(line, len(PwFieldCount))
		if err != nil {
			return nil, fmt.Errorf("SimpleTextPointsReader: parse field count: %w", err)
		}
		for i := 0; i < count; i++ {
			if err := readFn(); err != nil {
				return nil, fmt.Errorf("SimpleTextPointsReader: read field name line %d: %w", i, err)
			}
			fieldName := string(getLine()[len(PwFieldFPName):])
			if err := readFn(); err != nil {
				return nil, fmt.Errorf("SimpleTextPointsReader: read field fp line %d: %w", i, err)
			}
			fp, err := stParseLong(getLine(), len(PwFieldFP))
			if err != nil {
				return nil, fmt.Errorf("SimpleTextPointsReader: parse field fp %d: %w", i, err)
			}
			fieldToOffset[fieldName] = fp
		}
		// validate checksum footer
		expectedCS := fmt.Sprintf("%020d", in.GetChecksum())
		if err := readFn(); err != nil {
			return nil, fmt.Errorf("SimpleTextPointsReader: read checksum line: %w", err)
		}
		line = getLine()
		if !bytes.HasPrefix(line, []byte("checksum ")) {
			return nil, fmt.Errorf("SimpleTextPointsReader: expected checksum line, got: %s", line)
		}
		actualCS := string(line[len("checksum "):])
		if expectedCS != actualCS {
			return nil, fmt.Errorf("SimpleTextPointsReader: index checksum mismatch: expected %s got %s",
				expectedCS, actualCS)
		}
	}

	// -----------------------------------------------------------------------
	// 2. Open the data file (.dim) and build a BKD reader per field.
	// -----------------------------------------------------------------------
	dataFileName := store.SegmentFileName(
		state.SegmentInfo.Name(),
		state.SegmentSuffix,
		PointExtension,
	)
	dataIn, err := state.Directory.OpenInput(dataFileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("SimpleTextPointsReader: open data %s: %w", dataFileName, err)
	}

	r := &SimpleTextPointsReader{
		dataIn:    dataIn,
		readState: state,
		readers:   make(map[string]*SimpleTextBKDReader),
		scratch:   util.NewBytesRefBuilder(),
	}

	for fieldName, fp := range fieldToOffset {
		bkd, err := r.initReader(fp)
		if err != nil {
			_ = dataIn.Close()
			return nil, fmt.Errorf("SimpleTextPointsReader: initReader(%q): %w", fieldName, err)
		}
		r.readers[fieldName] = bkd
	}

	return r, nil
}

// initReader seeks to fp in the data file, reads the BKD metadata, and
// returns a SimpleTextBKDReader for that field.
//
// Port of SimpleTextPointsReader.initReader(long).
func (r *SimpleTextPointsReader) initReader(fp int64) (*SimpleTextBKDReader, error) {
	if err := r.dataIn.SetPosition(fp); err != nil {
		return nil, fmt.Errorf("SimpleTextPointsReader.initReader: SetPosition(%d): %w", fp, err)
	}

	readLine := func() ([]byte, error) {
		if err := stReadLine(r.dataIn, r.scratch); err != nil {
			return nil, err
		}
		return r.scratch.Bytes()[:r.scratch.Length()], nil
	}
	parseInt := func(prefix []byte) (int, error) {
		line, err := readLine()
		if err != nil {
			return 0, err
		}
		v, err := strconv.Atoi(string(line[len(prefix):]))
		if err != nil {
			return 0, fmt.Errorf("parseInt: %w", err)
		}
		return v, nil
	}
	parseLong := func(prefix []byte) (int64, error) {
		line, err := readLine()
		if err != nil {
			return 0, err
		}
		v, err := strconv.ParseInt(string(line[len(prefix):]), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parseLong: %w", err)
		}
		return v, nil
	}

	numDataDims, err := parseInt(PwNumDataDims)
	if err != nil {
		return nil, fmt.Errorf("initReader: numDataDims: %w", err)
	}
	numIndexDims, err := parseInt(PwNumIndexDims)
	if err != nil {
		return nil, fmt.Errorf("initReader: numIndexDims: %w", err)
	}
	bytesPerDim, err := parseInt(PwBytesPerDim)
	if err != nil {
		return nil, fmt.Errorf("initReader: bytesPerDim: %w", err)
	}
	maxPointsInLeafNode, err := parseInt(PwMaxLeafPts)
	if err != nil {
		return nil, fmt.Errorf("initReader: maxPointsInLeafNode: %w", err)
	}

	indexCount, err := parseInt(PwIndexCount)
	if err != nil {
		return nil, fmt.Errorf("initReader: indexCount: %w", err)
	}

	// min/max packed values
	minLine, err := readLine()
	if err != nil {
		return nil, fmt.Errorf("initReader: min value line: %w", err)
	}
	minPackedValue, err := fromBytesRefString(string(minLine[len(PwMinValue):]))
	if err != nil {
		return nil, fmt.Errorf("initReader: min value: %w", err)
	}

	maxLine, err := readLine()
	if err != nil {
		return nil, fmt.Errorf("initReader: max value line: %w", err)
	}
	maxPackedValue, err := fromBytesRefString(string(maxLine[len(PwMaxValue):]))
	if err != nil {
		return nil, fmt.Errorf("initReader: max value: %w", err)
	}

	pointCount, err := parseLong(PwPointCount)
	if err != nil {
		return nil, fmt.Errorf("initReader: pointCount: %w", err)
	}
	docCount, err := parseInt(PwDocCount)
	if err != nil {
		return nil, fmt.Errorf("initReader: docCount: %w", err)
	}

	leafBlockFPs := make([]int64, indexCount)
	for i := range leafBlockFPs {
		leafBlockFPs[i], err = parseLong(PwBlockFP)
		if err != nil {
			return nil, fmt.Errorf("initReader: blockFP[%d]: %w", i, err)
		}
	}

	splitCount, err := parseInt(PwSplitCount)
	if err != nil {
		return nil, fmt.Errorf("initReader: splitCount: %w", err)
	}

	var bytesPerIndexEntry int
	if numIndexDims == 1 {
		bytesPerIndexEntry = bytesPerDim
	} else {
		bytesPerIndexEntry = 1 + bytesPerDim
	}
	splitPackedValues := make([]byte, splitCount*bytesPerIndexEntry)
	for i := 0; i < splitCount; i++ {
		address := bytesPerIndexEntry * i
		splitDim, err := parseInt(PwSplitDim)
		if err != nil {
			return nil, fmt.Errorf("initReader: splitDim[%d]: %w", i, err)
		}
		if numIndexDims != 1 {
			splitPackedValues[address] = byte(splitDim)
			address++
		}
		splitLine, err := readLine()
		if err != nil {
			return nil, fmt.Errorf("initReader: splitValue[%d]: %w", i, err)
		}
		splitVal, err := fromBytesRefString(string(splitLine[len(PwSplitValue):]))
		if err != nil {
			return nil, fmt.Errorf("initReader: parse splitValue[%d]: %w", i, err)
		}
		copy(splitPackedValues[address:], splitVal[:bytesPerDim])
	}

	return NewSimpleTextBKDReader(
		r.dataIn,
		numDataDims,
		numIndexDims,
		maxPointsInLeafNode,
		bytesPerDim,
		leafBlockFPs,
		splitPackedValues,
		minPackedValue,
		maxPackedValue,
		pointCount,
		docCount,
	)
}

// GetValues returns the PointValues for the given field.
//
// Port of SimpleTextPointsReader.getValues(String).
func (r *SimpleTextPointsReader) GetValues(fieldName string) (index.PointValues, error) {
	fi := r.readState.FieldInfos.GetByName(fieldName)
	if fi == nil {
		return nil, fmt.Errorf("SimpleTextPointsReader.GetValues: field %q is unrecognized", fieldName)
	}
	if fi.PointDimensionCount() == 0 {
		return nil, fmt.Errorf("SimpleTextPointsReader.GetValues: field %q did not index points", fieldName)
	}
	reader, ok := r.readers[fieldName]
	if !ok {
		return nil, nil
	}
	return newSimpleTextPointValues(reader), nil
}

// ---------------------------------------------------------------------------
// simpleTextPointValues — index.PointValues view over a SimpleTextBKDReader.
// ---------------------------------------------------------------------------

// simpleTextPointValues projects a SimpleTextBKDReader onto the canonical
// index.PointValues surface that spi.PointsReader.GetValues is declared to
// return.
//
// In Apache Lucene 10.5.0 no projection is needed: SimpleTextBKDReader itself
// extends org.apache.lucene.index.PointValues, and
// SimpleTextPointsReader.getValues returns it directly. Gocene carries two
// renderings of that Java class — codecs.PointValues (the wide
// Intersect/EstimatePointCount surface, which SimpleTextBKDReader implements)
// and index.PointValues (the canonical SPI surface, whose packed-value
// accessors also return an error) — and the two cannot be satisfied by one Go
// type because GetMinPackedValue/GetMaxPackedValue differ in arity. This view
// is therefore the same bridge codecs/lucene90 uses for its BKD reader
// (codecs/lucene90/lucene90_points.go:241-311).
type simpleTextPointValues struct {
	reader *SimpleTextBKDReader
}

// newSimpleTextPointValues wraps reader as an index.PointValues.
func newSimpleTextPointValues(reader *SimpleTextBKDReader) *simpleTextPointValues {
	return &simpleTextPointValues{reader: reader}
}

// Intersect walks the BKD tree, driving visitor for every matching cell and
// point. It bridges index.PointTreeIntersectVisitor (Compare returns an int)
// to codecs.IntersectVisitor (Compare returns a geo.Relation).
func (pv *simpleTextPointValues) Intersect(visitor index.PointTreeIntersectVisitor) error {
	return pv.reader.Intersect(&simpleTextVisitorBridge{v: visitor})
}

// EstimatePointCount returns the reader's estimate of how many points the
// visitor will match.
func (pv *simpleTextPointValues) EstimatePointCount(visitor index.PointTreeIntersectVisitor) int64 {
	count := pv.reader.EstimatePointCount(&simpleTextVisitorBridge{v: visitor})
	if count < 0 {
		return 0
	}
	return count
}

// GetMinPackedValue returns the per-dimension minimum packed value. The error
// return matches index.PointValues; SimpleTextBKDReader never fails, so it is
// always nil.
func (pv *simpleTextPointValues) GetMinPackedValue() ([]byte, error) {
	return pv.reader.GetMinPackedValue(), nil
}

// GetMaxPackedValue returns the per-dimension maximum packed value.
func (pv *simpleTextPointValues) GetMaxPackedValue() ([]byte, error) {
	return pv.reader.GetMaxPackedValue(), nil
}

// GetNumDimensions returns the number of indexed point dimensions.
func (pv *simpleTextPointValues) GetNumDimensions() int { return pv.reader.GetNumDimensions() }

// GetNumIndexDimensions returns the number of dimensions used for indexing.
func (pv *simpleTextPointValues) GetNumIndexDimensions() int {
	return pv.reader.GetNumIndexDimensions()
}

// GetBytesPerDimension returns the number of bytes per dimension.
func (pv *simpleTextPointValues) GetBytesPerDimension() int {
	return pv.reader.GetBytesPerDimension()
}

// GetDocCount returns the number of documents with at least one point value.
// Port of PointValues.getDocCount().
func (pv *simpleTextPointValues) GetDocCount() int { return pv.reader.GetDocCount() }

// GetDocCountWithValue returns the document count (BKD tracks doc count, not
// per-document value multiplicity), as in codecs/lucene90.
func (pv *simpleTextPointValues) GetDocCountWithValue() int64 {
	return int64(pv.reader.GetDocCount())
}

// GetValueCount returns the total number of indexed point values.
// Port of PointValues.size().
func (pv *simpleTextPointValues) GetValueCount() int64 { return pv.reader.Size() }

// GetPointTree returns a cursor at the root of the field's tree.
// Port of PointValues.getPointTree().
func (pv *simpleTextPointValues) GetPointTree() (bkd.PointTree, error) {
	return pv.reader.GetPointTree(), nil
}

var _ index.PointValues = (*simpleTextPointValues)(nil)

// simpleTextVisitorBridge adapts an index.PointTreeIntersectVisitor (Compare
// returns an int in {0,1,2}) to a codecs.IntersectVisitor (Compare returns a
// geo.Relation). The int convention matches the geo.Relation enum order, so the
// conversion is a direct cast.
type simpleTextVisitorBridge struct {
	v index.PointTreeIntersectVisitor
}

func (b *simpleTextVisitorBridge) Visit(docID int) error { return b.v.Visit(docID) }

func (b *simpleTextVisitorBridge) VisitByPackedValue(docID int, packedValue []byte) error {
	return b.v.VisitByPackedValue(docID, packedValue)
}

func (b *simpleTextVisitorBridge) Compare(minPackedValue, maxPackedValue []byte) geo.Relation {
	return geo.Relation(b.v.Compare(minPackedValue, maxPackedValue))
}

func (b *simpleTextVisitorBridge) Grow(count int) { b.v.Grow(count) }

var _ codecs.IntersectVisitor = (*simpleTextVisitorBridge)(nil)

// CheckIntegrity validates the checksum of the data file.
//
// Port of SimpleTextPointsReader.checkIntegrity().
func (r *SimpleTextPointsReader) CheckIntegrity() error {
	clone := r.dataIn.Clone()
	if err := clone.SetPosition(0); err != nil {
		return fmt.Errorf("SimpleTextPointsReader.CheckIntegrity: seek(0): %w", err)
	}
	input := store.NewBufferedChecksumIndexInput(clone)
	scratch := util.NewBytesRefBuilder()

	// The checksum footer is at fileLength - (len("checksum ") + 20 + 1 newline).
	footerStartPos := clone.Length() - int64(len("checksum ")+21)

	for {
		if err := stReadLine(input, scratch); err != nil {
			return fmt.Errorf("SimpleTextPointsReader.CheckIntegrity: readLine: %w", err)
		}
		if input.GetFilePointer() >= footerStartPos {
			if input.GetFilePointer() != footerStartPos {
				return fmt.Errorf(
					"SimpleTextPointsReader.CheckIntegrity: footer at wrong position %d, expected %d",
					input.GetFilePointer(), footerStartPos)
			}
			break
		}
	}

	// Validate checksum footer.
	expectedCS := fmt.Sprintf("%020d", input.GetChecksum())
	if err := stReadLine(input, scratch); err != nil {
		return fmt.Errorf("SimpleTextPointsReader.CheckIntegrity: read checksum line: %w", err)
	}
	line := scratch.Bytes()[:scratch.Length()]
	if !bytes.HasPrefix(line, []byte("checksum ")) {
		return fmt.Errorf("SimpleTextPointsReader.CheckIntegrity: expected checksum line, got: %s", line)
	}
	actualCS := string(line[len("checksum "):])
	if expectedCS != actualCS {
		return fmt.Errorf("SimpleTextPointsReader.CheckIntegrity: checksum mismatch: expected %s got %s",
			expectedCS, actualCS)
	}
	return nil
}

// GetMergeInstance returns the receiver.
//
// SimpleTextPointsReader does not override getMergeInstance, so it inherits the
// PointsReader default (PointsReader.java:56), which is {@code return this;}.
func (r *SimpleTextPointsReader) GetMergeInstance() codecs.PointsReader { return r }

// Close releases the data file.
//
// Port of SimpleTextPointsReader.close().
func (r *SimpleTextPointsReader) Close() error {
	if err := r.dataIn.Close(); err != nil {
		return fmt.Errorf("SimpleTextPointsReader.Close: %w", err)
	}
	return nil
}

// compile-time assertion.
var _ codecs.PointsReader = (*SimpleTextPointsReader)(nil)
