// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// packedWriter implements Writer for both FormatPacked and
// FormatPackedSingleBlock. It buffers up to `iterations` worth of
// values, then encodes them with a BulkOperation and flushes the
// resulting byte block to the underlying DataOutput.
//
// The on-disk byte layout matches Lucene's PackedWriter exactly: the
// PACKED encoder writes the most significant byte of each 64-bit
// block first.
type packedWriter struct {
	out          store.DataOutput
	format       Format
	encoder      BulkOperation
	nextBlocks   []byte
	nextValues   []int64
	iterations   int
	valueCount   int
	bitsPerValue int
	off          int
	written      int
	finished     bool
}

func newPackedWriter(format Format, out store.DataOutput, valueCount, bitsPerValue, mem int) (*packedWriter, error) {
	if bitsPerValue > 64 {
		return nil, fmt.Errorf("packed: bitsPerValue must be <= 64, got %d", bitsPerValue)
	}
	if valueCount < 0 && valueCount != -1 {
		return nil, fmt.Errorf("packed: valueCount must be >= 0 or -1, got %d", valueCount)
	}
	encoder, err := BulkOperationOf(format, bitsPerValue)
	if err != nil {
		return nil, err
	}
	iterations := encoder.ComputeIterations(valueCount, mem)
	return &packedWriter{
		out:          out,
		format:       format,
		encoder:      encoder,
		nextBlocks:   make([]byte, iterations*encoder.ByteBlockCount()),
		nextValues:   make([]int64, iterations*encoder.ByteValueCount()),
		iterations:   iterations,
		valueCount:   valueCount,
		bitsPerValue: bitsPerValue,
	}, nil
}

// Format returns the on-disk format of this writer.
func (w *packedWriter) Format() Format { return w.format }

// BitsPerValue returns the bit width used to encode values.
func (w *packedWriter) BitsPerValue() int { return w.bitsPerValue }

// Ord returns the index of the most recently added value.
func (w *packedWriter) Ord() int { return w.written - 1 }

// Add appends a value to the stream.
func (w *packedWriter) Add(v int64) error {
	if UnsignedBitsRequired(uint64(v)) > w.bitsPerValue {
		return fmt.Errorf("packed: value %d does not fit in %d bits", v, w.bitsPerValue)
	}
	if w.finished {
		return errors.New("packed: writer is finished")
	}
	if w.valueCount != -1 && w.written >= w.valueCount {
		return errors.New("packed: writing past end of stream")
	}
	w.nextValues[w.off] = v
	w.off++
	if w.off == len(w.nextValues) {
		if err := w.flush(); err != nil {
			return err
		}
	}
	w.written++
	return nil
}

// Finish flushes any buffered values, padding with zeros so that the
// declared valueCount is reached.
func (w *packedWriter) Finish() error {
	if w.finished {
		return errors.New("packed: writer already finished")
	}
	if w.valueCount != -1 {
		for w.written < w.valueCount {
			if err := w.Add(0); err != nil {
				return err
			}
		}
	}
	if err := w.flush(); err != nil {
		return err
	}
	w.finished = true
	return nil
}

func (w *packedWriter) flush() error {
	w.encoder.EncodeLongsToBytes(w.nextValues, 0, w.nextBlocks, 0, w.iterations)
	blockCount := int(w.format.ByteCount(VersionCurrent, w.off, w.bitsPerValue))
	if err := w.out.WriteBytes(w.nextBlocks[:blockCount]); err != nil {
		return err
	}
	for i := range w.nextValues {
		w.nextValues[i] = 0
	}
	w.off = 0
	return nil
}
