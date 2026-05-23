// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
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
	indexFileName := index.SegmentFileName(
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
	dataFileName := index.SegmentFileName(
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
func (r *SimpleTextPointsReader) GetValues(fieldName string) (codecs.PointValues, error) {
	fi := r.readState.FieldInfos.GetByName(fieldName)
	if fi == nil {
		return nil, fmt.Errorf("SimpleTextPointsReader.GetValues: field %q is unrecognized", fieldName)
	}
	if fi.PointDimensionCount() == 0 {
		return nil, fmt.Errorf("SimpleTextPointsReader.GetValues: field %q did not index points", fieldName)
	}
	bkd, ok := r.readers[fieldName]
	if !ok {
		return nil, nil
	}
	return bkd, nil
}

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
