// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package directory

import "testing"

// TestTaxonomyIndexArraysBasic verifies the children/siblings derivation from
// a known parent array. Mirrors TestTaxonomyIndexArrays in the Java reference.
func TestTaxonomyIndexArraysBasic(t *testing.T) {
	// Build a small taxonomy:
	//   0 (root)
	//   ├── 1
	//   │   ├── 3
	//   │   └── 4
	//   └── 2
	//       └── 5
	parents := []int{InvalidOrdinal, 0, 0, 1, 1, 2}
	ta := NewTaxonomyIndexArraysFromParents(parents)

	ch := ta.Children()
	sib := ta.Siblings()

	// Root's youngest child should be 2 (the last-added child of root).
	if ch[0] != 2 {
		t.Errorf("children[0] = %d; want 2", ch[0])
	}
	// 1 is the older sibling of 2 (since 1 was root's child before 2).
	if sib[2] != 1 {
		t.Errorf("siblings[2] = %d; want 1", sib[2])
	}
	// 1's youngest child is 4 (last-added child of 1).
	if ch[1] != 4 {
		t.Errorf("children[1] = %d; want 4", ch[1])
	}
	// 3 is the older sibling of 4.
	if sib[4] != 3 {
		t.Errorf("siblings[4] = %d; want 3", sib[4])
	}
	// 2's only child is 5.
	if ch[2] != 5 {
		t.Errorf("children[2] = %d; want 5", ch[2])
	}
	// Leaves have no children.
	for _, leaf := range []int{3, 4, 5} {
		if ch[leaf] != InvalidOrdinal {
			t.Errorf("children[%d] = %d; want InvalidOrdinal", leaf, ch[leaf])
		}
	}
}

// TestTaxonomyIndexArraysAdd verifies incremental growth.
func TestTaxonomyIndexArraysAdd(t *testing.T) {
	ta := NewTaxonomyIndexArraysFromParents([]int{InvalidOrdinal}) // just root
	ta.Add(1, 0)
	ta.Add(2, 0)

	ch := ta.Children()
	if ch[0] != 2 {
		t.Errorf("after Add: children[0] = %d; want 2", ch[0])
	}
}
