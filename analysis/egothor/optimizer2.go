// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package egothor

// Optimizer2 is a stricter variant of Optimizer that only merges two cells
// when they are identical (same cmd, ref, and skip). This prevents information
// loss and allows the stemmer to be self-teaching.
//
// This is the Go port of org.egothor.stemmer.Optimizer2 (Lucene 10.4.0).
type Optimizer2 struct {
	Optimizer
}

// Optimize calls Optimizer.Optimize but with the Optimizer2 cell-merge rule.
func (op2 *Optimizer2) Optimize(orig *Trie) *Trie {
	remap := make([]int, len(orig.rows))
	for i := range remap {
		remap[i] = -1
	}

	var rows []*Row

	for j := len(orig.rows) - 1; j >= 0; j-- {
		now := remapRow(orig.rows[j], remap)
		merged := false

		for i, existing := range rows {
			q := op2.mergeRows2(now, existing)
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

// mergeRows2 merges master into existing using the Optimizer2 cell-merge rule.
func (op2 *Optimizer2) mergeRows2(master, existing *Row) *Row {
	n := &Row{cells: make(map[rune]*cell, len(master.cells)+len(existing.cells))}
	for ch, a := range master.cells {
		b := existing.at(ch)
		var s *cell
		if b == nil {
			s = newCellFrom(a)
		} else {
			s = op2.mergeCells2(a, b)
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

// mergeCells2 merges two cells only when cmd, ref, and skip all match.
func (op2 *Optimizer2) mergeCells2(m, e *cell) *cell {
	if m.cmd == e.cmd && m.ref == e.ref && m.skip == e.skip {
		c := newCellFrom(m)
		c.cnt += e.cnt
		return c
	}
	return nil
}
