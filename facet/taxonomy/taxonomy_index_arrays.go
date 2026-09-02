// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/facet"
	"github.com/FlavioCFOliveira/Gocene/internal/index"
)

const (
	chunkSizeBits = 13
	chunkSize     = 1 << chunkSizeBits
	chunkMask     = chunkSize - 1
)

type chunkedIntArray struct {
	values [][]int32
}

func (a *chunkedIntArray) Get(i int) int {
	return int(a.values[i>>chunkSizeBits][i&chunkMask])
}

func (a *chunkedIntArray) set(i int, val int) {
	a.values[i>>chunkSizeBits][i&chunkMask] = int32(val)
}

func (a *chunkedIntArray) Length() int {
	if len(a.values) == 0 {
		return 0
	}
	return ((len(a.values) - 1) << chunkSizeBits) + len(a.values[len(a.values)-1])
}

type TaxonomyIndexArrays struct {
	parents *chunkedIntArray
	children *chunkedIntArray
	siblings *chunkedIntArray
	initializedChildren bool
}

func NewTaxonomyIndexArrays(reader index.IndexReader) error {
	maxDoc := reader.MaxDoc()
	parentArray := allocateChunkedArray(maxDoc, 0)
	if len(parentArray) > 0 && len(parentArray[0]) > 0 {
		if err := initParents(parentArray, reader, 0); err != nil {
			return err
		}
		parentArray[0][0] = -1 // Root ordinal's parent is INVALID_ORDINAL (-1)
	}

	tia := &TaxonomyIndexArrays{
		parents: &chunkedIntArray{values: parentArray},
	}
	return nil
}

func allocateChunkedArray(size int, startFrom int) [][]int32 {
	chunkCount := (size >> chunkSizeBits) + 1
	array := make([][]int32, chunkCount)
	for i := startFrom; i < chunkCount-1; i++ {
		array[i] = make([]int32, chunkSize)
	}
	if chunkCount > 0 {
		array[chunkCount-1] = make([]int32, size&chunkMask)
	}
	return array
}

func initParents(parentArray [][]int32, reader index.IndexReader, first int) error {
	if reader.MaxDoc() == first {
		return nil
	}

	for _, ctx := range reader.Leaves() {
		leafReader := ctx.Reader()
		leafDocNum := leafReader.MaxDoc()
		if ctx.DocBase+leafDocNum <= first {
			continue
		}

		parentValues := leafReader.GetNumericDocValues("$parent_ndv$")
		if parentValues == nil {
			return fmt.Errorf("parent data field $parent_ndv$ does not exist")
		}

		for doc := max(first-ctx.DocBase, 0); doc < leafDocNum; doc++ {
			if !parentValues.AdvanceExact(doc) {
				return fmt.Errorf("missing parent data for category %d", doc+ctx.DocBase)
			}
			pos := doc + ctx.DocBase
			parentArray[pos>>chunkSizeBits][pos&chunkMask] = int32(parentValues.LongValue())
		}
	}
	return nil
}

func (tia *TaxonomyIndexArrays) Parents() ParallelTaxonomyArrays.IntArray {
	return tia.parents
}

func (tia *TaxonomyIndexArrays) Children() ParallelTaxonomyArrays.IntArray {
	if !tia.initializedChildren {
		tia.initChildrenSiblings()
	}
	return tia.children
}

func (tia *TaxonomyIndexArrays) Siblings() ParallelTaxonomyArrays.IntArray {
	if !tia.initializedChildren {
		tia.initChildrenSiblings()
	}
	return tia.siblings
}

func (tia *TaxonomyIndexArrays) initChildrenSiblings() {
	length := tia.parents.Length()
	childrenArray := allocateChunkedArray(length, 0)
	siblingsArray := allocateChunkedArray(length, 0)
	tia.children = &chunkedIntArray{values: childrenArray}
	tia.siblings = &chunkedIntArray{values: siblingsArray}

	for i := 0; i < length; i++ {
		tia.children.set(i, -1)
	}

	if length > 0 {
		tia.siblings.set(0, -1)
	}

	for i := 1; i < length; i++ {
		parent := tia.parents.Get(i)
		tia.siblings.set(i, tia.children.Get(parent))
		tia.children.set(parent, i)
	}
	tia.initializedChildren = true
}

func (tia *TaxonomyIndexArrays) Add(ordinal int, parentOrdinal int) *TaxonomyIndexArrays {
	if ordinal >= tia.parents.Length() {
		newParents := allocateChunkedArray(oversize(ordinal+1), len(tia.parents.values)-1)
		copyChunkedArray(tia.parents.values, newParents)
		newParents[ordinal>>chunkSizeBits][ordinal&chunkMask] = int32(parentOrdinal)
		return &TaxonomyIndexArrays{
			parents: &chunkedIntArray{values: newParents},
		}
	}
	tia.parents.set(ordinal, parentOrdinal)
	return tia
}

func copyChunkedArray(oldArray [][]int32, newArray [][]int32) {
	if len(oldArray) > 1 {
		copy(newArray, oldArray[:len(oldArray)-1])
	}
	lastCopyChunk := oldArray[len(oldArray)-1]
	copy(newArray[len(oldArray)-1], lastCopyChunk)
}

func oversize(n int) int {
	// Simple version of Lucene's ArrayUtil.oversize
	return n + (n >> 3)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
