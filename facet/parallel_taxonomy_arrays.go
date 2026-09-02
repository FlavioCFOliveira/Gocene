// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"fmt"
)

// ParallelTaxonomyArrays is the interface for arrays that store taxonomy info.
type ParallelTaxonomyArrays interface {
	Parents() IntArray
	Children() IntArray
	Siblings() IntArray
}

// IntArray is a simple interface for an integer array.
type IntArray interface {
	Get(i int) int
	Length() int
}

// ChunkedIntArray is an implementation of IntArray that uses chunks.
type ChunkedIntArray struct {
	values [][]int
}

const chunkSizeBits = 13
const chunkSize = 1 << chunkSizeBits
const chunkMask = chunkSize - 1

func NewChunkedIntArray(size int) *ChunkedIntArray {
	chunkCount := (size >> chunkSizeBits) + 1
	values := make([][]int, chunkCount)
	for i := 0; i < chunkCount-1; i++ {
		values[i] = make([]int, chunkSize)
	}
	if chunkCount > 0 {
		values[chunkCount-1] = make([]int, size&chunkMask)
	}
	return &ChunkedIntArray{values: values}
}

func (c *ChunkedIntArray) Get(i int) int {
	return c.values[i>>chunkSizeBits][i&chunkMask]
}

func (c *ChunkedIntArray) Set(i int, val int) {
	c.values[i>>chunkSizeBits][i&chunkMask] = val
}

func (c *ChunkedIntArray) Length() int {
	if len(c.values) == 0 {
		return 0
	}
	return ((len(c.values) - 1) << chunkSizeBits) + len(c.values[len(c.values)-1])
}

func (c *ChunkedIntArray) grow(newSize int) {
	oldChunkCount := len(c.values)
	newChunkCount := (newSize >> chunkSizeBits) + 1
	if newChunkCount <= oldChunkCount {
		return
	}

	newValues := make([][]int, newChunkCount)
	copy(newValues, c.values)
	for i := oldChunkCount; i < newChunkCount-1; i++ {
		newValues[i] = make([]int, chunkSize)
	}
	newValues[newChunkCount-1] = make([]int, newSize&chunkMask)
	c.values = newValues
}
