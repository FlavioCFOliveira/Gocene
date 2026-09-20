// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/DisiWrapper.java
//   lucene/core/src/java/org/apache/lucene/search/DisiPriorityQueue.java

// DisiWrapper wraps a Scorer for use in disjunction priority queues.
// It caches the iterator, cost, current docID, and optional two-phase
// match cost for ordering.
//
// Mirrors org.apache.lucene.search.DisiWrapper (Lucene 10.4.0).
//
// Deviations from Java:
//   - scorable is the Scorer directly (Gocene's Scorer is also Scorable).
//   - twoPhaseView and matchCost are populated only when the scorer
//     satisfies the scorerTwoPhaseProvider interface; otherwise both
//     are left at their zero values (nil / 0).
//   - iterator exposes the raw DISI (approximation when two-phase, else scorer)
//     so that WANDScorer can advance it directly as Java does.
//   - scaledMaxScore is the max score of this clause scaled as a long integer,
//     used by WANDScorer for block-max pruning.
type DisiWrapper struct {
	// scorer is the original Scorer this wrapper was built from.
	scorer Scorer
	// scorable is used for scoring; in Gocene Scorer satisfies Scorable.
	scorable Scorer
	// iterator is the raw DocIdSetIterator used for direct advancement (same
	// as approximation but named to mirror the Java field).
	iterator DocIdSetIterator
	// The DISI to use for approximation-based iteration.
	approximation DocIdSetIterator
	// cost is the estimated iteration cost (number of matching docs).
	cost int64
	// doc is the current document ID.
	doc int
	// next links wrappers sharing the same doc in a topList linked list.
	next *DisiWrapper
	// twoPhaseView is the optional TwoPhaseIterator, or nil.
	twoPhaseView *TwoPhaseIterator
	// matchCost is twoPhaseView.MatchCost() when twoPhaseView != nil.
	matchCost float32
	// scaledMaxScore is the max score of this clause scaled as a long, used
	// by WANDScorer for block-max pruning.  Zero when not in TOP_SCORES mode.
	scaledMaxScore int64
	// maxWindowScore is the maximum score this clause can contribute in the
	// current scoring window.  Used by MaxScoreBulkScorer for partitioning.
	maxWindowScore float32
	// postingsEnum is the same object as iterator when iterator is a
	// PostingsEnum, and nil otherwise.  Mirrors the Java field of the same
	// name.
	postingsEnum index.PostingsEnum
	// weight is the per-field BM25F weight used by CombinedFieldQuery.
	// Mirrors the Java field of the same name; it is 1 for every other caller.
	weight float32
}

// Doc returns the current document ID cached in this wrapper.
func (w *DisiWrapper) Doc() int { return w.doc }

// SetDoc sets the current document ID cached in this wrapper.
// Callers must call DisiPriorityQueue.UpdateTop after modifying the top.
func (w *DisiWrapper) SetDoc(doc int) { w.doc = doc }

// Next returns the next DisiWrapper in a topList chain, or nil.
// The field is set by TopList / topListAt; callers should treat the
// chain as read-only after retrieval.
func (w *DisiWrapper) Next() *DisiWrapper { return w.next }

// Advance advances the wrapper's underlying iterator to the first document at
// or beyond target, updates the cached Doc, and returns the new doc ID.
// This is equivalent to the Java field access top.iterator.advance(target)
// followed by top.doc = result.
func (w *DisiWrapper) Advance(target int) (int, error) {
	doc, err := w.iterator.Advance(target)
	if err != nil {
		return doc, err
	}
	w.doc = doc
	return doc, nil
}

// Scorer returns the underlying Scorer this wrapper was built from.
func (w *DisiWrapper) Scorer() Scorer { return w.scorer }

// Scorable returns the Scorable (same as Scorer in Gocene) for scoring.
func (w *DisiWrapper) Scorable() Scorer { return w.scorable }

// NewDisiWrapper constructs a DisiWrapper for scorer.
// If scorer exposes a TwoPhaseIterator via the scorerTwoPhaseProvider
// interface, the wrapper captures it and sets matchCost accordingly.
// The impacts parameter is ignored in this port (no ImpactsDISI yet).
func NewDisiWrapper(scorer Scorer, impacts bool) *DisiWrapper {
	return newDisiWrapper(scorer, impacts, 1)
}

// newDisiWrapper constructs a DisiWrapper for scorer with an explicit BM25F
// weight.
//
// Mirrors the package-private constructor
// DisiWrapper(Scorer, boolean, float), which the public two-argument
// constructor delegates to with a weight of 1f.
func newDisiWrapper(scorer Scorer, _ bool, weight float32) *DisiWrapper {
	w := &DisiWrapper{
		scorer:   scorer,
		scorable: scorer,
		cost:     scorer.Iterator().Cost(),
		doc:      -1,
		weight:   weight,
	}
	if sp, ok := scorer.(scorerTwoPhaseProvider); ok {
		w.twoPhaseView = sp.TwoPhaseIterator()
	}
	if w.twoPhaseView != nil {
		w.approximation = w.twoPhaseView.Approximation()
		w.matchCost = w.twoPhaseView.MatchCost()
	} else {
		w.approximation = scorer.Iterator()
	}
	w.iterator = w.approximation
	if pe, ok := w.iterator.(index.PostingsEnum); ok {
		w.postingsEnum = pe
	}
	return w
}

// ─── DisiPriorityQueue ───────────────────────────────────────────────────────
//
// DisiPriorityQueue is not declared here. In Lucene 10.5.0 it is its own class,
// an abstract sealed DisiPriorityQueue with the static factory ofMaxSize and the
// two subclasses DisiPriorityQueue2 and DisiPriorityQueueN. Gocene renders that
// as the DisiPriorityQueue interface plus OfMaxSize in disi.go, implemented by
// disi_priority_queue2.go and disi_priority_queue_n.go. The monolithic struct
// that used to sit here had no Lucene counterpart.

// ─── 0-indexed heap helpers (DisiPriorityQueueN) ────────────────────────────

// disiLeftNode returns the left child index in a 0-indexed binary heap.
// Mirrors DisiPriorityQueueN.leftNode(int).
func disiLeftNode(node int) int { return ((node + 1) << 1) - 1 }

// disiRightNode returns the right child index given the left child index.
// Mirrors DisiPriorityQueueN.rightNode(int).
func disiRightNode(leftNode int) int { return leftNode + 1 }

// disiParentNode returns the parent index in a 0-indexed binary heap.
// Mirrors DisiPriorityQueueN.parentNode(int).
func disiParentNode(node int) int { return ((node + 1) >> 1) - 1 }
