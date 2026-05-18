// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import "errors"

// ErrChildBeforeParent is returned by CheckJoinIndex when child documents
// appear after their parent or when a parent has no children.
var ErrChildBeforeParent = errors.New("join: each parent must follow its children in the block")

// CheckJoinIndex verifies that an iteration of (docID, isParent) tuples
// follows the block-join layout used by ToParentBlockJoinQuery: every parent
// document is immediately preceded by its block of children, with no parent
// or child interleaving from another block. Mirrors
// org.apache.lucene.search.join.CheckJoinIndex.
//
// The function returns nil when the iteration is well-formed and a
// descriptive error otherwise.
func CheckJoinIndex(docs []int, isParent []bool) error {
	if len(docs) != len(isParent) {
		return errors.New("join: docs and isParent must have the same length")
	}
	prevParent := -1
	for i := 0; i < len(docs); i++ {
		if isParent[i] {
			if i == 0 {
				continue
			}
			if prevParent == i-1 {
				// Parent immediately after another parent — empty block; ok.
				prevParent = i
				continue
			}
			prevParent = i
		} else {
			// child: must precede a parent
			if i+1 >= len(docs) {
				return ErrChildBeforeParent
			}
		}
	}
	if len(docs) > 0 && !isParent[len(docs)-1] {
		return ErrChildBeforeParent
	}
	return nil
}
