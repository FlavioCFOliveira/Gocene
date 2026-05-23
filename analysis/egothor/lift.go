// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package egothor

// Lift is a Trie reducer that implements the Lift-Up reduction method, which
// propagates leaf patch-commands upward toward the root to reduce overstemming.
//
// This is the Go port of org.egothor.stemmer.Lift (Lucene 10.4.0).
type Lift struct {
	changeSkip bool
}

// NewLift creates a Lift reducer. When changeSkip is true, cell-skip values
// are taken into account during the lift comparison.
func NewLift(changeSkip bool) *Lift {
	return &Lift{changeSkip: changeSkip}
}

// Optimize applies the Lift-Up reduction to orig and returns the reduced Trie.
func (l *Lift) Optimize(orig *Trie) *Trie {
	remap := make([]int, len(orig.rows))
	for i := range remap {
		remap[i] = -1
	}

	for j := len(orig.rows) - 1; j >= 0; j-- {
		l.liftUp(orig.rows[j], orig.rows)
	}

	newRows := make([]*Row, 0, len(orig.rows))
	newRows = removeGaps(orig.root, orig.rows, newRows, remap)
	return newTrieWithData(orig.forward, remap[orig.root], orig.cmds, newRows)
}

// liftUp propagates uniform leaf commands upward in the trie.
func (l *Lift) liftUp(in *Row, nodes []*Row) {
	for _, c := range in.cells {
		if c.ref >= 0 {
			to := nodes[c.ref]
			sum := to.uniformCmd(l.changeSkip)
			if sum >= 0 {
				if sum == c.cmd {
					if l.changeSkip {
						if c.skip != to.uniformSkip+1 {
							continue
						}
						c.skip = to.uniformSkip + 1
					} else {
						c.skip = 0
					}
					c.cnt += to.uniformCnt
					c.ref = -1
				} else if c.cmd < 0 {
					c.cnt = to.uniformCnt
					c.cmd = sum
					c.ref = -1
					if l.changeSkip {
						c.skip = to.uniformSkip + 1
					} else {
						c.skip = 0
					}
				}
			}
		}
	}
}
