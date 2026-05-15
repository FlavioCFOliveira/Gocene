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

// BlockPacked* constants exposed for tests and downstream code that
// shares the same wire layout (e.g. MonotonicBlockPackedWriter).
const (
	BlockPackedMinBlockSize = 64
	BlockPackedMaxBlockSize = 1 << (30 - 3)

	bpwMinValueEqualsZero = 1 << 0
	bpwBpvShift           = 1
)

// BlockPackedWriter writes long sequences in fixed-size blocks. For
// each block, delta = (max - min) is computed and each value is
// stored as (v - min) using just enough bits for that delta, plus a
// per-block 1-byte token (bitsPerValue << 1 | minIsZero) and an
// optional zigzag-encoded minimum.
//
// This is the Go port of org.apache.lucene.util.packed.BlockPackedWriter
// in Apache Lucene 10.4.0.
type BlockPackedWriter struct {
	out      store.DataOutput
	values   []int64
	blocks   []byte
	off      int
	ord      int64
	finished bool
}

// NewBlockPackedWriter returns a writer over out using fixed-size
// blocks of blockSize values (must be a power of 2 in [64, 2^27]).
func NewBlockPackedWriter(out store.DataOutput, blockSize int) (*BlockPackedWriter, error) {
	if _, err := CheckBlockSize(blockSize, BlockPackedMinBlockSize, BlockPackedMaxBlockSize); err != nil {
		return nil, err
	}
	return &BlockPackedWriter{
		out:    out,
		values: make([]int64, blockSize),
	}, nil
}

// Reset rewires the writer to a new DataOutput, keeping the block size.
func (w *BlockPackedWriter) Reset(out store.DataOutput) {
	w.out = out
	w.off = 0
	w.ord = 0
	w.finished = false
}

func (w *BlockPackedWriter) checkNotFinished() error {
	if w.finished {
		return fmt.Errorf("packed: already finished")
	}
	return nil
}

// Add appends a value to the stream, flushing the buffer when full.
func (w *BlockPackedWriter) Add(v int64) error {
	if err := w.checkNotFinished(); err != nil {
		return err
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

// Finish writes any trailing partial block. The writer is no longer
// usable after Finish until Reset is called.
func (w *BlockPackedWriter) Finish() error {
	if err := w.checkNotFinished(); err != nil {
		return err
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
func (w *BlockPackedWriter) Ord() int64 { return w.ord }

func (w *BlockPackedWriter) flush() error {
	if w.off == 0 {
		return fmt.Errorf("packed: flush with empty buffer")
	}
	var min int64 = math.MaxInt64
	var max int64 = math.MinInt64
	for i := 0; i < w.off; i++ {
		if w.values[i] < min {
			min = w.values[i]
		}
		if w.values[i] > max {
			max = w.values[i]
		}
	}
	delta := max - min
	bitsRequired := 0
	if delta != 0 {
		bitsRequired = UnsignedBitsRequired(uint64(delta))
	}
	if bitsRequired == 64 {
		// 64-bit values: skip the delta encoding entirely.
		min = 0
	} else if min > 0 {
		// Shrink min so the zigzag-encoded VLong takes fewer bytes.
		lo := max - MaxValue(bitsRequired)
		if lo < 0 {
			lo = 0
		}
		min = lo
	}
	token := byte(bitsRequired<<bpwBpvShift) | boolToTokenBit(min == 0)
	if err := w.out.WriteByte(token); err != nil {
		return err
	}
	if min != 0 {
		if err := blockPackedWriteVLong(w.out, util.ZigZagEncodeInt64(min)-1); err != nil {
			return err
		}
	}
	if bitsRequired > 0 {
		if min != 0 {
			for i := 0; i < w.off; i++ {
				w.values[i] -= min
			}
		}
		if err := w.writeValues(bitsRequired); err != nil {
			return err
		}
	}
	w.off = 0
	return nil
}

func boolToTokenBit(b bool) byte {
	if b {
		return bpwMinValueEqualsZero
	}
	return 0
}

func (w *BlockPackedWriter) writeValues(bitsRequired int) error {
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

// blockPackedWriteVLong is BlockPacked's writeVLong: like DataOutput.writeVLong
// but tolerant of negative values (caps at 9 bytes regardless of high bits).
func blockPackedWriteVLong(out store.DataOutput, i int64) error {
	for k := 0; (i & ^int64(0x7F)) != 0 && k < 8; k++ {
		if err := out.WriteByte(byte((i & 0x7F) | 0x80)); err != nil {
			return err
		}
		i = int64(uint64(i) >> 7)
	}
	return out.WriteByte(byte(i))
}
