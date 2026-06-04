// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// SimpleText points file extensions.
const (
	// PointExtension is the extension of the points data file.
	PointExtension = "dim"
	// PointIndexExtension is the extension of the points index file.
	PointIndexExtension = "dii"
)

// SimpleText points-writer token prefixes (public so SimpleTextBKDWriter can
// share them).
var (
	PwNumDataDims  = []byte("num data dims ")
	PwNumIndexDims = []byte("num index dims ")
	PwBytesPerDim  = []byte("bytes per dim ")
	PwMaxLeafPts   = []byte("max leaf points ")
	PwIndexCount   = []byte("index count ")
	PwBlockCount   = []byte("block count ")
	PwBlockDocID   = []byte("  doc ")
	PwBlockFP      = []byte("  block fp ")
	PwBlockValue   = []byte("  block value ")
	PwSplitCount   = []byte("split count ")
	PwSplitDim     = []byte("  split dim ")
	PwSplitValue   = []byte("  split value ")
	PwFieldCount   = []byte("field count ")
	PwFieldFPName  = []byte("  field fp name ")
	PwFieldFP      = []byte("  field fp ")
	PwMinValue     = []byte("min value ")
	PwMaxValue     = []byte("max value ")
	PwPointCount   = []byte("point count ")
	PwDocCount     = []byte("doc count ")
	PwEnd          = []byte("END")
)

// SimpleTextPointsWriter writes point values as plain text for debugging.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextPointsWriter
// (Lucene 10.4.0).
type SimpleTextPointsWriter struct {
	dataOut    *store.ChecksumIndexOutput
	scratch    *util.BytesRefBuilder
	writeState *codecs.SegmentWriteState
	// indexFPs maps field name → file pointer where the field's BKD data starts.
	indexFPs map[string]int64
	closed   bool
}

// NewSimpleTextPointsWriter opens the points data file and prepares the writer.
//
// Port of SimpleTextPointsWriter(SegmentWriteState).
func NewSimpleTextPointsWriter(state *codecs.SegmentWriteState) (*SimpleTextPointsWriter, error) {
	fileName := index.SegmentFileName(
		state.SegmentInfo.Name(),
		state.SegmentSuffix,
		PointExtension,
	)
	raw, err := state.Directory.CreateOutput(fileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("SimpleTextPointsWriter: create %s: %w", fileName, err)
	}
	return &SimpleTextPointsWriter{
		dataOut:    store.NewChecksumIndexOutput(raw),
		scratch:    util.NewBytesRefBuilder(),
		writeState: state,
		indexFPs:   make(map[string]int64),
	}, nil
}

// pointsSource is the in-memory surface SimpleTextPointsWriter.WriteField
// pulls buffered point values from. It mirrors the duck-typed contract the
// indexing chain (index.dwptPointsSource) and segment merger expose, and is
// structurally identical to codecs/lucene90.PointsSource: the same concrete
// source value satisfies both interfaces. Defining it locally keeps the
// simpletext codec free of an import dependency on the lucene90 package.
//
// This is the Gocene write-path analogue of Lucene's
// reader.getValues(field).getPointTree() — the indexing chain replays the
// buffered (docID, packedValue) pairs through VisitPoints in document order,
// the order SimpleTextBKDWriter.Add expects.
type pointsSource interface {
	// PointValueCount returns the number of buffered point values for field.
	PointValueCount(field string) int64
	// VisitPoints invokes fn for every buffered (docID, packedValue) pair.
	VisitPoints(field string, fn func(docID int, packedValue []byte) error) error
}

// WriteField writes all point values for fieldInfo using the supplied reader.
//
// It builds a BKD configuration from the field's point dimensions, creates a
// SimpleTextBKDWriter sized to the field's buffered point count, replays every
// (docID, packedValue) pair through SimpleTextBKDWriter.Add, and — when at
// least one point survived (the merge path can drop every point of a field
// whose documents were all deleted) — finishes the tree and records the index
// file pointer keyed by field name.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextPointsWriter.writeField
// (Lucene 10.4.0). Lucene drives the writer from
// reader.getValues(fieldInfo.name).getPointTree().visitDocValues(...); Gocene's
// indexing chain instead hands the writer an in-memory pointsSource (the same
// established convention used by Lucene90PointsWriter), so this method consumes
// VisitPoints in place of visitDocValues. The resulting on-disk text is
// byte-identical: both replay the points in document order through
// SimpleTextBKDWriter.add, and SimpleTextBKDWriter.finish performs the
// reordering and serialisation.
func (w *SimpleTextPointsWriter) WriteField(fieldInfo *index.FieldInfo, reader codecs.PointsReader) error {
	if w.closed {
		return errors.New("SimpleTextPointsWriter.WriteField: writer closed")
	}
	if w.dataOut == nil {
		return errors.New("SimpleTextPointsWriter.WriteField: data file already finalised")
	}

	src, ok := reader.(pointsSource)
	if !ok {
		return fmt.Errorf(
			"SimpleTextPointsWriter.WriteField: reader %T does not implement pointsSource", reader,
		)
	}

	// Mirror Lucene: new BKDConfig(pointDimensionCount, pointIndexDimensionCount,
	// pointNumBytes, BKDConfig.DEFAULT_MAX_POINTS_IN_LEAF_NODE).
	config, err := bkd.NewBKDConfig(
		fieldInfo.PointDimensionCount(),
		fieldInfo.PointIndexDimensionCount(),
		fieldInfo.PointNumBytes(),
		bkd.DefaultMaxPointsInLeafNode,
	)
	if err != nil {
		return fmt.Errorf("SimpleTextPointsWriter.WriteField: field %q config: %w", fieldInfo.Name(), err)
	}

	// totalPointCount is an upper bound (== values.size() in Lucene). The
	// indexing chain reports the exact buffered count.
	totalPointCount := src.PointValueCount(fieldInfo.Name())

	writer, err := NewSimpleTextBKDWriter(
		w.writeState.SegmentInfo.DocCount(),
		w.writeState.Directory,
		w.writeState.SegmentInfo.Name(),
		config,
		DefaultMaxMBSortInHeap,
		totalPointCount,
	)
	if err != nil {
		return fmt.Errorf("SimpleTextPointsWriter.WriteField: field %q new bkd writer: %w", fieldInfo.Name(), err)
	}
	defer func() { _ = writer.Close() }()

	if err := src.VisitPoints(fieldInfo.Name(), func(docID int, packedValue []byte) error {
		return writer.Add(packedValue, docID)
	}); err != nil {
		return fmt.Errorf("SimpleTextPointsWriter.WriteField: field %q add: %w", fieldInfo.Name(), err)
	}

	// We could have 0 points on merge since all docs with points may be deleted.
	if writer.GetPointCount() > 0 {
		indexFP, err := writer.Finish(w.dataOut)
		if err != nil {
			return fmt.Errorf("SimpleTextPointsWriter.WriteField: field %q finish: %w", fieldInfo.Name(), err)
		}
		w.indexFPs[fieldInfo.Name()] = indexFP
	}
	return nil
}

// Finish writes the END sentinel and checksum to the data file.
//
// Port of SimpleTextPointsWriter.finish().
func (w *SimpleTextPointsWriter) Finish() error {
	if err := stWrite(w.dataOut, PwEnd, w.scratch); err != nil {
		return err
	}
	if err := stWriteNewline(w.dataOut); err != nil {
		return err
	}
	return stWriteChecksum(w.dataOut, w.scratch)
}

// Close finalises the data file and writes the index file.
//
// Port of SimpleTextPointsWriter.close().
func (w *SimpleTextPointsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	if err := w.dataOut.Close(); err != nil {
		return fmt.Errorf("SimpleTextPointsWriter.Close: data file: %w", err)
	}
	w.dataOut = nil

	// Write the index file.
	fileName := index.SegmentFileName(
		w.writeState.SegmentInfo.Name(),
		w.writeState.SegmentSuffix,
		PointIndexExtension,
	)
	rawIndex, err := w.writeState.Directory.CreateOutput(fileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return fmt.Errorf("SimpleTextPointsWriter.Close: create index file: %w", err)
	}
	indexOut := store.NewChecksumIndexOutput(rawIndex)
	defer func() {
		_ = indexOut.Close()
	}()

	count := len(w.indexFPs)
	if err := stWrite(indexOut, PwFieldCount, w.scratch); err != nil {
		return err
	}
	if err := stWriteStr(indexOut, strconv.Itoa(count), w.scratch); err != nil {
		return err
	}
	if err := stWriteNewline(indexOut); err != nil {
		return err
	}
	for name, fp := range w.indexFPs {
		if err := stWrite(indexOut, PwFieldFPName, w.scratch); err != nil {
			return err
		}
		if err := stWriteStr(indexOut, name, w.scratch); err != nil {
			return err
		}
		if err := stWriteNewline(indexOut); err != nil {
			return err
		}
		if err := stWrite(indexOut, PwFieldFP, w.scratch); err != nil {
			return err
		}
		if err := stWriteStr(indexOut, strconv.FormatInt(fp, 10), w.scratch); err != nil {
			return err
		}
		if err := stWriteNewline(indexOut); err != nil {
			return err
		}
	}
	return stWriteChecksum(indexOut, w.scratch)
}

// ---------------------------------------------------------------------------
// SimpleTextUtil write helpers (inline; full implementation deferred to
// task 3202 / simple_text_util.go).
// ---------------------------------------------------------------------------

// stWrite writes a raw BytesRef-style byte slice to out with escape processing.
func stWrite(out store.DataOutput, b []byte, _ *util.BytesRefBuilder) error {
	for _, bx := range b {
		if bx == 10 || bx == 92 { // NEWLINE or ESCAPE
			if err := out.WriteByte(92); err != nil {
				return err
			}
		}
		if err := out.WriteByte(bx); err != nil {
			return err
		}
	}
	return nil
}

// stWriteStr converts s to UTF-8, then calls stWrite.
func stWriteStr(out store.DataOutput, s string, scratch *util.BytesRefBuilder) error {
	scratch.CopyChars(s)
	return stWrite(out, scratch.Bytes()[:scratch.Length()], scratch)
}

// stWriteNewline writes a single newline byte.
func stWriteNewline(out store.DataOutput) error {
	return out.WriteByte(10)
}

// stWriteChecksum writes the "checksum NNNN\n" footer used by all SimpleText
// files.  Mirrors SimpleTextUtil.writeChecksum.
func stWriteChecksum(out *store.ChecksumIndexOutput, scratch *util.BytesRefBuilder) error {
	cs := fmt.Sprintf("%020d", out.GetChecksum())
	checksum := []byte("checksum ")
	if err := stWrite(out, checksum, scratch); err != nil {
		return err
	}
	if err := stWriteStr(out, cs, scratch); err != nil {
		return err
	}
	return stWriteNewline(out)
}

// compile-time assertion.
var _ codecs.PointsWriter = (*SimpleTextPointsWriter)(nil)
