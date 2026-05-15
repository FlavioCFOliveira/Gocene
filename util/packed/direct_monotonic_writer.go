// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// DirectMonotonicWriter encodes a monotonically-increasing sequence of
// int64 values. The input is split into blocks of 2^blockShift values;
// for each block the writer computes a linear approximation (min value
// and average slope) and only encodes the per-element delta from the
// expected linear value with a DirectWriter.
//
// This is the Go port of org.apache.lucene.util.packed.DirectMonotonicWriter
// in Apache Lucene 10.4.0; the on-disk layout matches Lucene byte-for-byte.
type DirectMonotonicWriter struct {
	meta            DataOutputAt
	data            DataOutputAt
	numValues       int64
	baseDataPointer int64
	buffer          []int64
	bufferSize      int
	count           int64
	finished        bool
	previous        int64
}

// DataOutputAt is the writer surface DirectMonotonicWriter requires:
// the basic DataOutput methods plus a way to ask for the current
// position in the underlying file. The Lucene Java version takes an
// IndexOutput; in Go we model only the bits we actually use to keep
// the dependency surface narrow.
type DataOutputAt interface {
	store.DataOutput
	GetFilePointer() int64
}

// Allowed block-shift bounds, matching Lucene's static constants.
const (
	DirectMonotonicMinBlockShift = 2
	DirectMonotonicMaxBlockShift = 22
)

// NewDirectMonotonicWriter returns an instance suitable for encoding
// numValues into monotonic blocks of 2^blockShift values. Metadata is
// written to metaOut; encoded data to dataOut.
func NewDirectMonotonicWriter(metaOut, dataOut DataOutputAt, numValues int64, blockShift int) (*DirectMonotonicWriter, error) {
	if blockShift < DirectMonotonicMinBlockShift || blockShift > DirectMonotonicMaxBlockShift {
		return nil, fmt.Errorf("packed: blockShift must be in [%d-%d], got %d",
			DirectMonotonicMinBlockShift, DirectMonotonicMaxBlockShift, blockShift)
	}
	if numValues < 0 {
		return nil, fmt.Errorf("packed: numValues can't be negative, got %d", numValues)
	}
	var numBlocks int64
	if numValues == 0 {
		numBlocks = 0
	} else {
		numBlocks = ((numValues - 1) >> uint(blockShift)) + 1
	}
	const maxArrayLength = int64(0x7FFFFFF7) // Lucene ArrayUtil.MAX_ARRAY_LENGTH
	if numBlocks > maxArrayLength {
		return nil, fmt.Errorf("packed: blockShift is too low for the provided number of values: blockShift=%d, numValues=%d", blockShift, numValues)
	}
	blockSize := int64(1) << uint(blockShift)
	bufSize := blockSize
	if numValues < bufSize {
		bufSize = numValues
	}
	return &DirectMonotonicWriter{
		meta:            metaOut,
		data:            dataOut,
		numValues:       numValues,
		baseDataPointer: dataOut.GetFilePointer(),
		buffer:          make([]int64, bufSize),
		previous:        math.MinInt64,
	}, nil
}

// Add writes a new value into the writer. Values must be strictly
// monotonically non-decreasing; out-of-order input is rejected.
func (w *DirectMonotonicWriter) Add(v int64) error {
	if v < w.previous {
		return fmt.Errorf("packed: values do not come in order: %d, %d", w.previous, v)
	}
	if w.bufferSize == len(w.buffer) {
		if err := w.flush(); err != nil {
			return err
		}
	}
	w.buffer[w.bufferSize] = v
	w.bufferSize++
	w.previous = v
	w.count++
	return nil
}

// Finish must be called exactly once after the final Add. It flushes
// the trailing partial block.
func (w *DirectMonotonicWriter) Finish() error {
	if w.count != w.numValues {
		return fmt.Errorf("packed: wrong number of values added, expected %d, got %d", w.numValues, w.count)
	}
	if w.finished {
		return fmt.Errorf("packed: Finish has been called already")
	}
	if w.bufferSize > 0 {
		if err := w.flush(); err != nil {
			return err
		}
	}
	w.finished = true
	return nil
}

func (w *DirectMonotonicWriter) flush() error {
	if w.bufferSize == 0 {
		return fmt.Errorf("packed: flush called with empty buffer")
	}
	denom := w.bufferSize - 1
	if denom < 1 {
		denom = 1
	}
	avgInc := float32(float64(w.buffer[w.bufferSize-1]-w.buffer[0]) / float64(denom))

	var min int64 = math.MaxInt64
	for i := 0; i < w.bufferSize; i++ {
		expected := int64(avgInc * float32(int64(i)))
		w.buffer[i] -= expected
		if w.buffer[i] < min {
			min = w.buffer[i]
		}
	}

	var maxDelta int64
	for i := 0; i < w.bufferSize; i++ {
		w.buffer[i] -= min
		// Or-ing matches Lucene: works correctly even with negative overflow.
		maxDelta |= w.buffer[i]
	}

	if err := w.meta.WriteLong(min); err != nil {
		return err
	}
	if err := w.meta.WriteInt(int32(math.Float32bits(avgInc))); err != nil {
		return err
	}
	if err := w.meta.WriteLong(w.data.GetFilePointer() - w.baseDataPointer); err != nil {
		return err
	}
	if maxDelta == 0 {
		if err := w.meta.WriteByte(0); err != nil {
			return err
		}
	} else {
		bitsRequired := DirectWriterUnsignedBitsRequired(uint64(maxDelta))
		dw, err := GetDirectWriter(w.data, int64(w.bufferSize), bitsRequired)
		if err != nil {
			return err
		}
		for i := 0; i < w.bufferSize; i++ {
			if err := dw.Add(w.buffer[i]); err != nil {
				return err
			}
		}
		if err := dw.Finish(); err != nil {
			return err
		}
		if err := w.meta.WriteByte(byte(bitsRequired)); err != nil {
			return err
		}
	}
	w.bufferSize = 0
	return nil
}
