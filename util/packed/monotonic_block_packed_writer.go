// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// MonotonicBlockPackedWriter encodes large monotonically-increasing
// sequences of positive longs. Each block is modeled as a linear
// function f(x) = A*x + B and the per-element deltas from f(i) are
// packed at minimum bitsPerValue.
//
// This is the Go port of org.apache.lucene.util.packed.MonotonicBlockPackedWriter
// in Apache Lucene 10.4.0. The on-disk format matches Lucene byte-for-byte.
type MonotonicBlockPackedWriter struct {
	out      monotonicWriterDataOutput
	values   []int64
	blocks   []byte
	off      int
	ord      int64
	finished bool
}

// monotonicWriterDataOutput narrows the writer's dependency surface to
// the methods it actually uses: a DataOutput plus WriteVInt/WriteVLong.
type monotonicWriterDataOutput interface {
	store.DataOutput
	WriteVInt(i int32) error
	WriteVLong(i int64) error
}

// NewMonotonicBlockPackedWriter creates a writer over out using a
// fixed block size (must be a multiple of 64 in [64, 2^27]).
func NewMonotonicBlockPackedWriter(out monotonicWriterDataOutput, blockSize int) (*MonotonicBlockPackedWriter, error) {
	if _, err := CheckBlockSize(blockSize, BlockPackedMinBlockSize, BlockPackedMaxBlockSize); err != nil {
		return nil, err
	}
	return &MonotonicBlockPackedWriter{out: out, values: make([]int64, blockSize)}, nil
}

// Add appends a value to the stream. Values must be >= 0.
func (w *MonotonicBlockPackedWriter) Add(v int64) error {
	if w.finished {
		return fmt.Errorf("packed: already finished")
	}
	if v < 0 {
		return fmt.Errorf("packed: monotonic writer requires non-negative values, got %d", v)
	}
	if w.off == len(w.values) {
		if err := w.flush(); err != nil {
			return err
		}
	}
	w.values[w.off] = v
	w.off++
	w.ord++
	return nil
}

// Finish writes any trailing partial block.
func (w *MonotonicBlockPackedWriter) Finish() error {
	if w.finished {
		return fmt.Errorf("packed: already finished")
	}
	if w.off > 0 {
		if err := w.flush(); err != nil {
			return err
		}
	}
	w.finished = true
	return nil
}

// Ord returns the number of values added so far.
func (w *MonotonicBlockPackedWriter) Ord() int64 { return w.ord }

func (w *MonotonicBlockPackedWriter) flush() error {
	if w.off == 0 {
		return fmt.Errorf("packed: flush with empty buffer")
	}
	var avg float32
	if w.off > 1 {
		avg = float32(float64(w.values[w.off-1]-w.values[0]) / float64(w.off-1))
	}
	min := w.values[0]
	for i := 1; i < w.off; i++ {
		actual := w.values[i]
		expected := monotonicExpected(min, avg, i)
		if expected > actual {
			min -= expected - actual
		}
	}
	var maxDelta int64
	for i := 0; i < w.off; i++ {
		w.values[i] = w.values[i] - monotonicExpected(min, avg, i)
		if w.values[i] > maxDelta {
			maxDelta = w.values[i]
		}
	}
	// writeZLong(min) = writeVLong(zigZagEncode(min))
	if err := w.out.WriteVLong(util.ZigZagEncodeInt64(min)); err != nil {
		return err
	}
	if err := w.out.WriteInt(int32(math.Float32bits(avg))); err != nil {
		return err
	}
	if maxDelta == 0 {
		if err := w.out.WriteVInt(0); err != nil {
			return err
		}
	} else {
		bitsRequired := BitsRequired(maxDelta)
		if err := w.out.WriteVInt(int32(bitsRequired)); err != nil {
			return err
		}
		if err := w.writeValues(bitsRequired); err != nil {
			return err
		}
	}
	w.off = 0
	return nil
}

func (w *MonotonicBlockPackedWriter) writeValues(bitsRequired int) error {
	enc, err := GetEncoder(FormatPacked, VersionCurrent, bitsRequired)
	if err != nil {
		return err
	}
	iterations := len(w.values) / enc.ByteValueCount()
	blockSize := enc.ByteBlockCount() * iterations
	if cap(w.blocks) < blockSize {
		w.blocks = make([]byte, blockSize)
	} else {
		w.blocks = w.blocks[:blockSize]
	}
	if w.off < len(w.values) {
		for i := w.off; i < len(w.values); i++ {
			w.values[i] = 0
		}
	}
	enc.EncodeLongsToBytes(w.values, 0, w.blocks, 0, iterations)
	blockCount := FormatPacked.ByteCount(VersionCurrent, w.off, bitsRequired)
	return w.out.WriteBytesN(w.blocks, int(blockCount))
}

// monotonicExpected returns origin + (long)(avg * (long)index).
// Matches the static helper in Lucene's MonotonicBlockPackedReader.
func monotonicExpected(origin int64, average float32, index int) int64 {
	return origin + int64(average*float32(int64(index)))
}
