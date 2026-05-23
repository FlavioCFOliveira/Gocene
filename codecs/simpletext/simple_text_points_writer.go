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

// WriteField writes all point values for fieldInfo using the supplied reader.
//
// NOTE: deferred until SimpleTextBKDWriter (task 3195) is ported. This method
// returns an error rather than panicking so that callers handle the dependency
// cleanly.
//
// Port of SimpleTextPointsWriter.writeField(FieldInfo, PointsReader).
func (w *SimpleTextPointsWriter) WriteField(fieldInfo *index.FieldInfo, reader codecs.PointsReader) error {
	return errors.New("SimpleTextPointsWriter.WriteField: deferred — requires SimpleTextBKDWriter (task 3195)")
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
