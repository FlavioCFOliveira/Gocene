// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/java/org/apache/lucene/search/PhraseQueue.java

import "github.com/FlavioCFOliveira/Gocene/util"

// PhraseQueue is the priority queue over PhrasePositions used to advance the
// least PhrasePosition.
//
// Mirrors org.apache.lucene.search.PhraseQueue, a package-private final class
// extending org.apache.lucene.util.PriorityQueue<PhrasePositions>.
type PhraseQueue struct {
	*util.PriorityQueue[*PhrasePositions]
}

// NewPhraseQueue creates a PhraseQueue holding at most size elements.
//
// Mirrors PhraseQueue(int size), which delegates to PriorityQueue(int) and
// throws IllegalArgumentException for a negative size.
func NewPhraseQueue(size int) *PhraseQueue {
	pq, err := util.NewPriorityQueue(size, phraseQueueLessThan)
	if err != nil {
		panic(err)
	}
	return &PhraseQueue{PriorityQueue: pq}
}

// phraseQueueLessThan mirrors PhraseQueue.lessThan(PhrasePositions,
// PhrasePositions).
func phraseQueueLessThan(pp1, pp2 *PhrasePositions) bool {
	if pp1.Position == pp2.Position {
		// same doc and pp.position, so decide by actual term positions.
		// rely on: pp.position == tp.position - offset.
		if pp1.Offset == pp2.Offset {
			return pp1.Ord < pp2.Ord
		}
		return pp1.Offset < pp2.Offset
	}
	return pp1.Position < pp2.Position
}
