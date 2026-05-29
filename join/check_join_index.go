// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// liveDocsProvider is implemented by leaf readers that can report their
// live-docs bitmap. SegmentReader satisfies it; the interface keeps Check
// decoupled from the concrete reader type.
type liveDocsProvider interface {
	GetLiveDocs() util.Bits
}

// Check verifies that the given index is well-formed for block joins: each
// segment must contain at least one parent, the last document of every segment
// must be a parent, and—when a segment has deletions—a parent and all of its
// children must share the same liveness (a block is deleted as a unit).
//
// This is the Go port of org.apache.lucene.search.join.CheckJoinIndex.check.
// It returns a descriptive error instead of throwing IllegalStateException.
func Check(reader index.IndexReaderInterface, parentsFilter BitSetProducer) error {
	leaves, err := reader.Leaves()
	if err != nil {
		return err
	}
	for _, context := range leaves {
		leaf := context.LeafReader()
		if leaf == nil {
			continue
		}
		maxDoc := leaf.MaxDoc()
		if maxDoc == 0 {
			continue
		}

		parents, err := parentsFilter.GetBitSet(context)
		if err != nil {
			return err
		}
		if parents == nil || parents.Cardinality() == 0 {
			return fmt.Errorf("join: every segment should have at least one parent, but segment ord %d does not have any", context.Ord())
		}
		if !parents.Get(maxDoc - 1) {
			return fmt.Errorf("join: the last document of a segment must always be a parent, but segment ord %d has a child as a last doc", context.Ord())
		}

		var liveDocs util.Bits
		if ldp, ok := leaf.(liveDocsProvider); ok {
			liveDocs = ldp.GetLiveDocs()
		}
		if liveDocs == nil {
			continue
		}

		prevParentDoc := -1
		for parentDoc := parents.NextSetBit(0); parentDoc >= 0; parentDoc = parents.NextSetBit(parentDoc + 1) {
			parentIsLive := liveDocs.Get(parentDoc)
			for child := prevParentDoc + 1; child != parentDoc; child++ {
				childIsLive := liveDocs.Get(child)
				if parentIsLive != childIsLive {
					if parentIsLive {
						return fmt.Errorf("join: parent doc %d of segment ord %d is live but has a deleted child document %d", parentDoc, context.Ord(), child)
					}
					return fmt.Errorf("join: parent doc %d of segment ord %d is deleted but has a live child document %d", parentDoc, context.Ord(), child)
				}
			}
			prevParentDoc = parentDoc
		}
	}
	return nil
}

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
