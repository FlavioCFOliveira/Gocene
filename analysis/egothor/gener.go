// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package egothor

// Gener is a Reducer that discards infrequent nodes which would hinder
// the reduction process, defending the trie against excessive reductions.
//
// This is the Go port of org.egothor.stemmer.Gener (Lucene 10.4.0).
type Gener struct{}

// Optimize returns a new Trie with infrequent values removed.
func (g *Gener) Optimize(orig *Trie) *Trie {
	remap := make([]int, len(orig.rows))
	for i := range remap {
		remap[i] = 1
	}

	for j := len(orig.rows) - 1; j >= 0; j-- {
		if g.eat(orig.rows[j], remap) {
			remap[j] = 0
		}
	}

	for i := range remap {
		remap[i] = -1
	}
	newRows := make([]*Row, 0, len(orig.rows))
	newRows = removeGaps(orig.root, orig.rows, newRows, remap)
	return newTrieWithData(orig.forward, remap[orig.root], orig.cmds, newRows)
}

// eat returns true if in should be removed (no live cells remain after pruning).
func (g *Gener) eat(in *Row, remap []int) bool {
	sum := 0
	for _, c := range in.cells {
		sum += c.cnt
		if c.ref >= 0 && remap[c.ref] == 0 {
			c.ref = -1
		}
	}
	frame := sum / 10
	live := false
	for _, c := range in.cells {
		if c.cnt < frame && c.cmd >= 0 {
			c.cnt = 0
			c.cmd = -1
		}
		if c.cmd >= 0 || c.ref >= 0 {
			live = true
		}
	}
	return !live
}
