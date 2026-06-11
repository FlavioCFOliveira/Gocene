// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"github.com/FlavioCFOliveira/Gocene/spatial/prefixtree"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// CellToBytesRefIterator wraps a slice of prefix-tree cells as a
// [util.BytesRefIterator]. Each call to Next advances through the cells
// and returns the token bytes (with leaf flag) of the current cell.
//
// Port of org.apache.lucene.spatial.prefix.CellToBytesRefIterator from
// Apache Lucene 10.4.0.
//
// Deviation: Java wraps a java.util.Iterator[Cell]; Go uses a slice with
// an index cursor, which is idiomatic and functionally equivalent.
type CellToBytesRefIterator struct {
	cells    []prefixtree.Cell
	pos      int
	bytesRef util.BytesRef
}

// NewCellToBytesRefIterator creates an iterator that yields the token
// bytes of each cell in the supplied slice, including the leaf byte for
// leaf cells.
func NewCellToBytesRefIterator(cells []prefixtree.Cell) *CellToBytesRefIterator {
	return &CellToBytesRefIterator{cells: cells}
}

// Reset replaces the underlying cell slice and resets the cursor.
// After Reset the iterator is positioned before the first cell; the next
// call to Next will return the first cell's token bytes.
func (it *CellToBytesRefIterator) Reset(cells []prefixtree.Cell) {
	it.cells = cells
	it.pos = 0
}

// Next satisfies [util.BytesRefIterator]. It returns the token bytes of
// the current cell and advances the cursor, or (nil, nil) when all cells
// have been consumed.
func (it *CellToBytesRefIterator) Next() (*util.BytesRef, error) {
	if it.pos >= len(it.cells) {
		return nil, nil
	}
	cell := it.cells[it.pos]
	it.pos++
	result := cell.GetTokenBytesWithLeaf(&it.bytesRef)
	return result, nil
}

// Ensure CellToBytesRefIterator satisfies util.BytesRefIterator.
var _ util.BytesRefIterator = (*CellToBytesRefIterator)(nil)
