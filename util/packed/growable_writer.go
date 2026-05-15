// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

// defaultCopyMem is the default per-copy buffer size in bytes used
// when growing the underlying Mutable. Lucene exposes this as
// PackedInts.DEFAULT_BUFFER_SIZE (= 1024) and re-uses it across all
// of util/packed.
const defaultCopyMem = 1024

// GrowableWriter is a Mutable whose underlying packed-int storage
// resizes its bitsPerValue on demand when a Set call sees a value
// that overflows the current width.
//
// Note: setting a negative value forces a grow to 64 bits per value,
// matching Lucene's behavior.
//
// This is the Go port of org.apache.lucene.util.packed.GrowableWriter
// in Apache Lucene 10.4.0.
type GrowableWriter struct {
	currentMask             int64
	current                 Mutable
	acceptableOverheadRatio float32
}

// NewGrowableWriter creates a GrowableWriter starting at the given
// bitsPerValue; the underlying Mutable is grown if a later Set call
// stores a value that doesn't fit.
func NewGrowableWriter(startBitsPerValue, valueCount int, acceptableOverheadRatio float32) *GrowableWriter {
	gw := &GrowableWriter{
		acceptableOverheadRatio: acceptableOverheadRatio,
		current:                 GetMutable(valueCount, startBitsPerValue, acceptableOverheadRatio),
	}
	gw.currentMask = growableMask(gw.current.GetBitsPerValue())
	return gw
}

func growableMask(bitsPerValue int) int64 {
	if bitsPerValue == 64 {
		return ^int64(0)
	}
	return MaxValue(bitsPerValue)
}

// Get returns the value at index.
func (g *GrowableWriter) Get(index int) int64 { return g.current.Get(index) }

// GetBulk reads up to length values into arr[off:].
func (g *GrowableWriter) GetBulk(index int, arr []int64, off, length int) int {
	return g.current.GetBulk(index, arr, off, length)
}

// Size returns the configured value count.
func (g *GrowableWriter) Size() int { return g.current.Size() }

// GetBitsPerValue returns the current bits-per-value.
func (g *GrowableWriter) GetBitsPerValue() int { return g.current.GetBitsPerValue() }

// GetMutable returns the underlying Mutable so callers can pass it to
// PackedInts.copy or similar APIs without going through this wrapper.
func (g *GrowableWriter) GetMutable() Mutable { return g.current }

// GetFormat returns the format of the underlying Mutable.
func (g *GrowableWriter) GetFormat() Format { return g.current.GetFormat() }

func (g *GrowableWriter) ensureCapacity(value int64) {
	if value&g.currentMask == value {
		return
	}
	bitsRequired := UnsignedBitsRequired(uint64(value))
	valueCount := g.Size()
	next := GetMutable(valueCount, bitsRequired, g.acceptableOverheadRatio)
	Copy(g.current, 0, next, 0, valueCount, defaultCopyMem)
	g.current = next
	g.currentMask = growableMask(g.current.GetBitsPerValue())
}

// Set writes value at index, growing the storage if needed.
func (g *GrowableWriter) Set(index int, value int64) {
	g.ensureCapacity(value)
	g.current.Set(index, value)
}

// SetBulk writes len values from arr[off:] starting at index, growing
// the storage if any of the values doesn't fit the current width.
func (g *GrowableWriter) SetBulk(index int, arr []int64, off, length int) int {
	var max int64
	for i := off; i < off+length; i++ {
		// OR is correct: positives accumulate into a max-bit-width OR;
		// any negative input flips the sign bit, forcing 64-bit grow.
		max |= arr[i]
	}
	g.ensureCapacity(max)
	return g.current.SetBulk(index, arr, off, length)
}

// Fill assigns val to every position in [fromIndex, toIndex).
func (g *GrowableWriter) Fill(fromIndex, toIndex int, val int64) {
	g.ensureCapacity(val)
	g.current.Fill(fromIndex, toIndex, val)
}

// Clear resets every value to zero.
func (g *GrowableWriter) Clear() { g.current.Clear() }

// Resize returns a new GrowableWriter of the requested size, copying
// the prefix that fits (newSize may be shorter or longer than Size()).
func (g *GrowableWriter) Resize(newSize int) *GrowableWriter {
	next := NewGrowableWriter(g.GetBitsPerValue(), newSize, g.acceptableOverheadRatio)
	limit := g.Size()
	if newSize < limit {
		limit = newSize
	}
	Copy(g.current, 0, next, 0, limit, defaultCopyMem)
	return next
}

// RamBytesUsed approximates Lucene's accounting: object header +
// ref + mask + ratio + underlying Mutable.
func (g *GrowableWriter) RamBytesUsed() int64 {
	const headerOverhead = 16 + 8 + 8 + 4 // approximate, see Lucene
	return headerOverhead + g.current.RamBytesUsed()
}

// Ensure GrowableWriter satisfies the Mutable contract.
var _ Mutable = (*GrowableWriter)(nil)
