// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package egothor

// Optimizer is a Reducer that removes empty rows by merging a row into another
// when the first is a subset of the second.
//
// This is the Go port of org.egothor.stemmer.Optimizer (Lucene 10.4.0).
type Optimizer struct{}

// Optimize removes empty rows from orig and returns the reduced Trie.
func (op *Optimizer) Optimize(orig *Trie) *Trie {
	remap := make([]int, len(orig.rows))
	for i := range remap {
		remap[i] = -1
	}

	var rows []*Row

	for j := len(orig.rows) - 1; j >= 0; j-- {
		now := remapRow(orig.rows[j], remap)
		merged := false

		for i, existing := range rows {
			q := op.mergeRows(now, existing)
			if q != nil {
				rows[i] = q
				merged = true
				remap[j] = i
				break
			}
		}

		if !merged {
			remap[j] = len(rows)
			rows = append(rows, now)
		}
	}

	root := remap[orig.root]
	for i := range remap {
		remap[i] = -1
	}
	newRows := make([]*Row, 0, len(rows))
	newRows = removeGaps(root, rows, newRows, remap)
	return newTrieWithData(orig.forward, remap[root], orig.cmds, newRows)
}

// mergeRows merges master into existing and returns the combined row, or nil
// if they cannot be merged.
func (op *Optimizer) mergeRows(master, existing *Row) *Row {
	n := &Row{cells: make(map[rune]*cell, len(master.cells)+len(existing.cells))}
	for ch, a := range master.cells {
		b := existing.at(ch)
		var s *cell
		if b == nil {
			s = newCellFrom(a)
		} else {
			s = op.mergeCells(a, b)
		}
		if s == nil {
			return nil
		}
		n.cells[ch] = s
	}
	for ch, ec := range existing.cells {
		if master.at(ch) != nil {
			continue
		}
		n.cells[ch] = ec
	}
	return n
}

// mergeCells merges two cells and returns the combined cell, or nil if
// they cannot be merged.
func (op *Optimizer) mergeCells(m, e *cell) *cell {
	n := newCell()
	if m.skip != e.skip {
		return nil
	}
	if m.cmd >= 0 {
		if e.cmd >= 0 {
			if m.cmd == e.cmd {
				n.cmd = m.cmd
			} else {
				return nil
			}
		} else {
			n.cmd = m.cmd
		}
	} else {
		n.cmd = e.cmd
	}
	if m.ref >= 0 {
		if e.ref >= 0 {
			if m.ref == e.ref && m.skip == e.skip {
				n.ref = m.ref
			} else {
				return nil
			}
		} else {
			n.ref = m.ref
		}
	} else {
		n.ref = e.ref
	}
	n.cnt = m.cnt + e.cnt
	n.skip = m.skip
	return n
}
