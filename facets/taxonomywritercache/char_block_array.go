// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomywritercache

import (
	"fmt"
	"math"
)

// defaultBlockSize matches the Java default of 32 * 1024 runes.
const defaultBlockSize = 32 * 1024

// CharBlockArray grows a rune sequence in fixed-size blocks instead of
// doubling a single backing slice, keeping individual allocations small.
// Mirrors org.apache.lucene.facet.taxonomy.writercache.CharBlockArray.
//
// Note: the Java class operates on Java char (UTF-16 code unit), not full
// Unicode code points. Gocene uses Go rune (UTF-32) to preserve the
// byte-level semantics; callers that construct this from Go string data
// work correctly since Go strings are UTF-8.
type CharBlockArray struct {
	blocks    []charBlock
	current   *charBlock
	blockSize int
	length    int
}

// charBlock is one page of the CharBlockArray.
type charBlock struct {
	chars []rune
	used  int
}

// NewCharBlockArray creates a CharBlockArray with the default 32-KiB block size.
func NewCharBlockArray() *CharBlockArray {
	return NewCharBlockArrayWithBlockSize(defaultBlockSize)
}

// NewCharBlockArrayWithBlockSize creates a CharBlockArray with the given block size.
func NewCharBlockArrayWithBlockSize(blockSize int) *CharBlockArray {
	if blockSize <= 0 {
		panic("blockSize must be > 0")
	}
	c := &CharBlockArray{blockSize: blockSize}
	c.addBlock()
	return c
}

func (c *CharBlockArray) addBlock() {
	// Guard against exceeding 2 GB worth of runes (matches Java Integer.MAX_VALUE).
	if int64(c.blockSize)*int64(len(c.blocks)+1) > math.MaxInt32 {
		panic("cannot store more than 2 GB in CharBlockArray")
	}
	b := charBlock{chars: make([]rune, c.blockSize)}
	c.blocks = append(c.blocks, b)
	c.current = &c.blocks[len(c.blocks)-1]
}

// blockIndex returns which block the given overall index belongs to.
func (c *CharBlockArray) blockIndex(index int) int { return index / c.blockSize }

// indexInBlock returns the offset within its block of the given overall index.
func (c *CharBlockArray) indexInBlock(index int) int { return index % c.blockSize }

// AppendRune appends a single rune to the array.
func (c *CharBlockArray) AppendRune(r rune) *CharBlockArray {
	if c.current.used == c.blockSize {
		c.addBlock()
	}
	c.current.chars[c.current.used] = r
	c.current.used++
	c.length++
	return c
}

// AppendString appends all runes in s to the array.
func (c *CharBlockArray) AppendString(s string) *CharBlockArray {
	for _, r := range s {
		c.AppendRune(r)
	}
	return c
}

// AppendRunes appends runes[start:start+length] to the array.
func (c *CharBlockArray) AppendRunes(chars []rune, start, length int) *CharBlockArray {
	end := start + length
	for i := start; i < end; i++ {
		c.AppendRune(chars[i])
	}
	return c
}

// CharAt returns the rune at the given overall index.
func (c *CharBlockArray) CharAt(index int) rune {
	if index < 0 || index >= c.length {
		panic(fmt.Sprintf("CharBlockArray: index %d out of range [0, %d)", index, c.length))
	}
	b := c.blocks[c.blockIndex(index)]
	return b.chars[c.indexInBlock(index)]
}

// Length returns the total number of runes stored.
func (c *CharBlockArray) Length() int { return c.length }

// SubSequence returns the sub-string from start (inclusive) to end (exclusive).
// Mirrors CharBlockArray.subSequence.
func (c *CharBlockArray) SubSequence(start, end int) string {
	if start < 0 || end > c.length || start > end {
		panic(fmt.Sprintf("CharBlockArray.SubSequence: invalid range [%d, %d) for length %d",
			start, end, c.length))
	}
	remaining := end - start
	buf := make([]rune, 0, remaining)
	blockIdx := c.blockIndex(start)
	inBlock := c.indexInBlock(start)
	for remaining > 0 {
		b := c.blocks[blockIdx]
		canRead := b.used - inBlock
		if canRead > remaining {
			canRead = remaining
		}
		buf = append(buf, b.chars[inBlock:inBlock+canRead]...)
		remaining -= canRead
		blockIdx++
		inBlock = 0
	}
	return string(buf)
}

// String returns the full content of the array as a Go string.
func (c *CharBlockArray) String() string {
	buf := make([]rune, 0, c.length)
	for _, b := range c.blocks {
		buf = append(buf, b.chars[:b.used]...)
	}
	return string(buf)
}
