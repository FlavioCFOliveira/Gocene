// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package egothor

// Reducer is the interface implemented by Trie-reduction strategies.
//
// This corresponds to the Reduce base class in Lucene 10.4.0.
type Reducer interface {
	// Optimize restructures and returns the given Trie.
	Optimize(orig *Trie) *Trie
}

// reduce is the base implementation: it removes gaps (unreachable rows) from
// the Trie by compacting only reachable rows.
//
// This is the Go port of org.egothor.stemmer.Reduce (Lucene 10.4.0).
type reduce struct{}

// Optimize removes holes in the rows of orig and returns the restructured Trie.
func (red *reduce) Optimize(orig *Trie) *Trie {
	remap := make([]int, len(orig.rows))
	for i := range remap {
		remap[i] = -1
	}
	newRows := make([]*Row, 0, len(orig.rows))
	newRows = removeGaps(orig.root, orig.rows, newRows, remap)
	return newTrieWithData(orig.forward, remap[orig.root], orig.cmds, newRows)
}

// removeGaps recursively walks the trie from ind, collecting reachable rows
// into to and recording their remapped indices in remap.
func removeGaps(ind int, old, to []*Row, remap []int) []*Row {
	remap[ind] = len(to)
	now := old[ind]
	to = append(to, now)
	for _, c := range now.cells {
		if c.ref >= 0 && remap[c.ref] < 0 {
			to = removeGaps(c.ref, old, to, remap)
		}
	}
	to[remap[ind]] = remapRow(now, remap)
	return to
}

// remapRow returns a copy of old with all cell refs remapped.
func remapRow(old *Row, remap []int) *Row {
	nr := &Row{cells: make(map[rune]*cell, len(old.cells))}
	for ch, c := range old.cells {
		nc := newCellFrom(c)
		if c.ref >= 0 {
			nc.ref = remap[nc.ref]
		}
		nr.cells[ch] = nc
	}
	return nr
}
