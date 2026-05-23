// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package egothor contains the Egothor stemmer data structures used by the
// Stempel Polish stemmer. This is a Go port of the org.egothor.stemmer
// package from Apache Lucene 10.4.0.
package egothor

import "fmt"

// cell is a portion of a trie node.
//
// This is the Go port of org.egothor.stemmer.Cell (Lucene 10.4.0).
type cell struct {
	// ref is the next row id in this direction (-1 = none).
	ref int
	// cmd is the patch command index (-1 = none).
	cmd int
	// cnt is the number of commands in the subtrie before pack.
	cnt int
	// skip is the number of chars discarded from the input key in this way.
	skip int
}

// newCell creates a default cell with cmd=-1, ref=-1.
func newCell() *cell {
	return &cell{cmd: -1, ref: -1}
}

// newCellFrom constructs a cell copying all fields from a.
func newCellFrom(a *cell) *cell {
	return &cell{ref: a.ref, cmd: a.cmd, cnt: a.cnt, skip: a.skip}
}

// String returns a debugging representation of the cell.
func (c *cell) String() string {
	return fmt.Sprintf("ref(%d)cmd(%d)cnt(%d)skp(%d)", c.ref, c.cmd, c.cnt, c.skip)
}
