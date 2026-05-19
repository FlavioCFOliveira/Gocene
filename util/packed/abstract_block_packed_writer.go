// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// abstractBlockPackedWriter is the package-private base for the block-packed
// writer family (BlockPackedWriter, MonotonicBlockPackedWriter, etc). It owns
// the buffer state and lifecycle and delegates the per-block layout to a
// caller-supplied flusher.
//
// This is the Go port of org.apache.lucene.util.packed.AbstractBlockPackedWriter
// in Apache Lucene 10.4.0. Java's abstract method `flush()` is modelled here
// as the `flusher` callback because Go has no inheritance: a concrete writer
// composes this struct and supplies its own flushing strategy.
//
// Unexported by design: the in-package concrete writers either embed this
// struct or compose it; external code interacts exclusively with the public
// concrete types.
type abstractBlockPackedWriter struct {
	out      store.DataOutput
	values   []int64
	blocks   []byte
	off      int
	ord      int64
	finished bool

	// flusher writes one block of size off (in [1, len(values)]) to out and
	// is invoked from add when the buffer is full and from finish on any
	// trailing partial block. Implementations are free to consult and mutate
	// any exported field via the receiver they captured.
	flusher func() error
}

// abstractBlockPackedConstants mirrors the package-private constants of
// Lucene's AbstractBlockPackedWriter. The public exported names continue to
// live in block_packed_writer.go (BlockPackedMinBlockSize, BlockPackedMaxBlockSize)
// for back-compat; these unexported aliases match Lucene's surface 1:1.
const (
	abpwMinBlockSize       = BlockPackedMinBlockSize
	abpwMaxBlockSize       = BlockPackedMaxBlockSize
	abpwMinValueEqualsZero = bpwMinValueEqualsZero
	abpwBpvShift           = bpwBpvShift
)

// init initialises the base struct with a fresh output and value buffer.
// It mirrors Lucene's protected constructor: blockSize is validated and the
// writer is reset against out before the buffer is allocated.
func (w *abstractBlockPackedWriter) init(out store.DataOutput, blockSize int, flusher func() error) error {
	if _, err := CheckBlockSize(blockSize, abpwMinBlockSize, abpwMaxBlockSize); err != nil {
		return err
	}
	if flusher == nil {
		return fmt.Errorf("packed: abstractBlockPackedWriter requires a non-nil flusher")
	}
	w.flusher = flusher
	w.reset(out)
	w.values = make([]int64, blockSize)
	return nil
}

// reset rewires the writer to wrap out. The block size and flusher are
// preserved. Equivalent to Lucene's public reset(DataOutput).
func (w *abstractBlockPackedWriter) reset(out store.DataOutput) {
	if out == nil {
		// Java asserts out != null; in Go we surface this as a panic to
		// keep the contract loud during development. Production callers
		// should never pass nil.
		panic("packed: abstractBlockPackedWriter.reset called with nil out")
	}
	w.out = out
	w.off = 0
	w.ord = 0
	w.finished = false
}

// checkNotFinished returns an error if Finish has already been called.
// Equivalent to Lucene's private checkNotFinished() which throws
// IllegalStateException.
func (w *abstractBlockPackedWriter) checkNotFinished() error {
	if w.finished {
		return fmt.Errorf("packed: already finished")
	}
	return nil
}

// add appends a new long, flushing the buffer when full.
func (w *abstractBlockPackedWriter) add(l int64) error {
	if err := w.checkNotFinished(); err != nil {
		return err
	}
	if w.off == len(w.values) {
		if err := w.flusher(); err != nil {
			return err
		}
	}
	w.values[w.off] = l
	w.off++
	w.ord++
	return nil
}

// addBlockOfZeros appends a full block of zeros. Lucene marks this method
// "for testing only" and requires off to be either 0 or len(values).
func (w *abstractBlockPackedWriter) addBlockOfZeros() error {
	if err := w.checkNotFinished(); err != nil {
		return err
	}
	if w.off != 0 && w.off != len(w.values) {
		return fmt.Errorf("packed: addBlockOfZeros requires an empty or full buffer, off=%d", w.off)
	}
	if w.off == len(w.values) {
		if err := w.flusher(); err != nil {
			return err
		}
	}
	for i := range w.values {
		w.values[i] = 0
	}
	w.off = len(w.values)
	w.ord += int64(len(w.values))
	return nil
}

// finish flushes any trailing partial block and marks the writer unusable
// until reset is called again.
func (w *abstractBlockPackedWriter) finish() error {
	if err := w.checkNotFinished(); err != nil {
		return err
	}
	if w.off > 0 {
		if err := w.flusher(); err != nil {
			return err
		}
	}
	w.finished = true
	return nil
}

// ordinal returns the number of values added so far. Named ordinal rather
// than ord because the embedded field is already named ord.
func (w *abstractBlockPackedWriter) ordinal() int64 { return w.ord }

// writeValues encodes the buffered values at bitsRequired bits-per-value
// and emits exactly the number of bytes required by FormatPacked. Off may
// be smaller than len(values), in which case the tail is zero-filled
// before encoding so the encoder operates on a complete iteration count.
func (w *abstractBlockPackedWriter) writeValues(bitsRequired int) error {
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

// abstractWriteVLong is the package-private static helper writeVLong(DataOutput, long)
// from Lucene: like DataOutput.writeVLong but tolerant of negative values
// (caps the continuation chain at 9 bytes total regardless of high bits).
//
// blockPackedWriteVLong in block_packed_writer.go is the same function and
// is preserved for source compatibility within the package.
func abstractWriteVLong(out store.DataOutput, i int64) error {
	for k := 0; (i & ^int64(0x7F)) != 0 && k < 8; k++ {
		if err := out.WriteByte(byte((i & 0x7F) | 0x80)); err != nil {
			return err
		}
		i = int64(uint64(i) >> 7)
	}
	return out.WriteByte(byte(i))
}
