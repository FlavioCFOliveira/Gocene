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

package fst

import (
	"errors"
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Util.java port — Top-N shortest-paths search.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/fst/Util.java

// OutputComparator compares two output values. Mirrors
// {@code java.util.Comparator<T>} as used by Util.TopNSearcher.
type OutputComparator[T any] func(a, b T) int

// PathComparator compares two FSTPath instances. Mirrors
// {@code java.util.Comparator<FSTPath<T>>}.
type PathComparator[T any] func(a, b *FSTPath[T]) int

// FSTPath represents a partial path under exploration by
// [TopNSearcher]. Mirrors {@code Util.FSTPath<T>}.
type FSTPath[T any] struct {
	// Arc is the last arc appended to this path.
	Arc *Arc[T]
	// Output holds the accumulated path output plus any consumer-specific
	// addend (boost, payload weighting, …).
	Output T
	// Input is the consumed label sequence so far.
	Input *util.IntsRefBuilder
	// Boost is a free-form factor reserved for the NRT suggester; the
	// FST machinery itself does not consult it. Mirrors the Java field.
	Boost float32
	// Context is a free-form annotation, mirroring the Java field.
	Context string
	// Payload is a consumer-defined integer payload mirroring the Java
	// field; the NRT suggester uses it to record whether this path has
	// already enumerated a surface form.
	Payload int
}

// NewFSTPath builds an FSTPath whose Arc is a deep-copy of the supplied
// arc. Mirrors the package-private Java constructor.
func NewFSTPath[T any](
	output T,
	arc *Arc[T],
	input *util.IntsRefBuilder,
	boost float32,
	context string,
	payload int,
) *FSTPath[T] {
	p := &FSTPath[T]{
		Arc:     (&Arc[T]{}).CopyFrom(arc),
		Output:  output,
		Input:   input,
		Boost:   boost,
		Context: context,
		Payload: payload,
	}
	return p
}

// newPath constructs a sibling path that shares boost/context/payload
// with the receiver but supplies a fresh output + input. Mirrors
// {@code FSTPath.newPath}.
func (p *FSTPath[T]) newPath(output T, input *util.IntsRefBuilder) *FSTPath[T] {
	return NewFSTPath(output, p.Arc, input, p.Boost, p.Context, p.Payload)
}

// String returns the Lucene-format debug rendering.
func (p *FSTPath[T]) String() string {
	return fmt.Sprintf("input=%s output=%v context=%s boost=%v payload=%d",
		p.Input.Get().HexString(), p.Output, p.Context, p.Boost, p.Payload)
}

// TieBreakByInputComparator returns a path comparator that first uses
// cmp and then breaks ties by comparing path.Input lexicographically.
// Mirrors {@code Util.TieBreakByInputComparator}.
func TieBreakByInputComparator[T any](cmp OutputComparator[T]) PathComparator[T] {
	return func(a, b *FSTPath[T]) int {
		c := cmp(a.Output, b.Output)
		if c != 0 {
			return c
		}
		return util.IntsRefCompare(a.Input.Get(), b.Input.Get())
	}
}

// Result is one (input, output) hit returned by [ShortestPaths] /
// [TopNSearcher.Search]. Mirrors {@code Util.Result<T>}.
type Result[T any] struct {
	Input  *util.IntsRef
	Output T
}

// TopResults is the bundle returned by [TopNSearcher.Search].
//
// IsComplete is true iff the configured queue size was large enough to
// accommodate every candidate path; when false the caller should treat
// TopN as a best-effort subset. Mirrors {@code Util.TopResults<T>}.
type TopResults[T any] struct {
	IsComplete bool
	TopN       []*Result[T]
}

// TopNSearcher finds the top-N shortest (lowest-cost) completions
// starting from one or more start arcs. Mirrors
// {@code Util.TopNSearcher<T>}.
type TopNSearcher[T any] struct {
	fst            *FST[T]
	bytesReader    BytesReader
	topN           int
	maxQueueDepth  int
	scratchArc     *Arc[T]
	comparator     OutputComparator[T]
	pathComparator PathComparator[T]

	// queue is the sorted-set of candidate paths ordered by
	// pathComparator. Java uses TreeSet; we use a sorted slice because
	// the queue is bounded by maxQueueDepth (typically small) and the
	// operations we need — pollFirst, pollLast, last, add — map to
	// O(log n) binary-search insertion plus O(n) shift on insert. For
	// the typical maxQueueDepth this beats a heap which would not
	// efficiently support pollLast / last.
	//
	// queueDisabled mirrors Java's "queue = null" sentinel: when true
	// the searcher has irrevocably committed to the current path and
	// will not consider further alternatives.
	queue         []*FSTPath[T]
	queueDisabled bool

	// acceptPartialPath / acceptResult are user-overridable hooks
	// mirroring the protected Java methods. nil means "always accept".
	AcceptPartialPath func(path *FSTPath[T]) bool
	AcceptResult      func(input *util.IntsRef, output T) bool
}

// NewTopNSearcher builds a TopNSearcher with the default pathComparator
// (tie-break by input). Mirrors the simpler Java constructor.
func NewTopNSearcher[T any](
	fst *FST[T],
	topN, maxQueueDepth int,
	comparator OutputComparator[T],
) *TopNSearcher[T] {
	return NewTopNSearcherWithPathComparator(
		fst, topN, maxQueueDepth, comparator, TieBreakByInputComparator(comparator),
	)
}

// NewTopNSearcherWithPathComparator is the full-fidelity constructor
// that lets the caller supply an explicit path comparator.
func NewTopNSearcherWithPathComparator[T any](
	fst *FST[T],
	topN, maxQueueDepth int,
	comparator OutputComparator[T],
	pathComparator PathComparator[T],
) *TopNSearcher[T] {
	return &TopNSearcher[T]{
		fst:            fst,
		bytesReader:    fst.GetBytesReader(),
		topN:           topN,
		maxQueueDepth:  maxQueueDepth,
		scratchArc:     &Arc[T]{},
		comparator:     comparator,
		pathComparator: pathComparator,
	}
}

// addIfCompetitive inserts a new candidate path derived from path into
// the queue if it would compete with the current bottom. Mirrors
// {@code TopNSearcher.addIfCompetitive}.
func (s *TopNSearcher[T]) addIfCompetitive(path *FSTPath[T]) {
	if s.queueDisabled {
		return
	}
	output := s.fst.outputs.Add(path.Output, path.Arc.Output())
	if len(s.queue) == s.maxQueueDepth {
		bottom := s.queue[len(s.queue)-1]
		comp := s.pathComparator(path, bottom)
		if comp > 0 {
			return
		}
		if comp == 0 {
			// Tie break by alpha sort on the input. Java appends the
			// arc label, compares, then truncates. We do the same.
			path.Input.Append(path.Arc.Label())
			cmp := util.IntsRefCompare(bottom.Input.Get(), path.Input.Get())
			path.Input.SetLength(path.Input.Length() - 1)
			if cmp == 0 {
				// Duplicate detection — the Java assertion fires here.
				panic("fst.TopNSearcher.addIfCompetitive: unexpected duplicate path")
			}
			if cmp < 0 {
				return
			}
		}
	}
	// Copy current input and append the arc label.
	newInput := util.NewIntsRefBuilder()
	newInput.CopyIntsRef(path.Input.Get())
	newInput.Append(path.Arc.Label())
	newPath := path.newPath(output, newInput)
	if s.AcceptPartialPath != nil && !s.AcceptPartialPath(newPath) {
		return
	}
	s.queueInsert(newPath)
	if len(s.queue) == s.maxQueueDepth+1 {
		// pollLast: drop the largest entry.
		s.queue = s.queue[:len(s.queue)-1]
	}
}

// queueInsert inserts p into the queue maintaining sorted order.
func (s *TopNSearcher[T]) queueInsert(p *FSTPath[T]) {
	idx := sort.Search(len(s.queue), func(i int) bool {
		return s.pathComparator(s.queue[i], p) >= 0
	})
	s.queue = append(s.queue, nil)
	copy(s.queue[idx+1:], s.queue[idx:])
	s.queue[idx] = p
}

// queuePollFirst removes and returns the smallest entry, or nil.
func (s *TopNSearcher[T]) queuePollFirst() *FSTPath[T] {
	if len(s.queue) == 0 {
		return nil
	}
	p := s.queue[0]
	s.queue = s.queue[1:]
	return p
}

// AddStartPaths is the convenience overload corresponding to the Java
// signature {@code addStartPaths(Arc, T, boolean, IntsRefBuilder)}.
func (s *TopNSearcher[T]) AddStartPaths(
	node *Arc[T],
	startOutput T,
	allowEmptyString bool,
	input *util.IntsRefBuilder,
) error {
	return s.AddStartPathsFull(node, startOutput, allowEmptyString, input, 0, "", -1)
}

// AddStartPathsFull adds all leaving arcs of node — including the
// synthesised final arc when node is final — to the queue. Mirrors the
// full {@code addStartPaths(Arc, T, boolean, IntsRefBuilder, float, CharSequence, int)}.
func (s *TopNSearcher[T]) AddStartPathsFull(
	node *Arc[T],
	startOutput T,
	allowEmptyString bool,
	input *util.IntsRefBuilder,
	boost float32,
	context string,
	payload int,
) error {
	noOutput := s.fst.outputs.GetNoOutput()
	// De-dup NO_OUTPUT to the singleton, mirroring "equals" in Java.
	if isNoOutput[T](s.fst.outputs, startOutput, noOutput) {
		startOutput = noOutput
	}
	path := NewFSTPath(startOutput, node, input, boost, context, payload)
	if _, err := s.fst.ReadFirstTargetArc(node, path.Arc, s.bytesReader); err != nil {
		return err
	}
	// Bootstrap: enumerate the leaving arcs.
	for {
		if allowEmptyString || path.Arc.Label() != END_LABEL {
			s.addIfCompetitive(path)
		}
		if path.Arc.IsLast() {
			break
		}
		if _, err := s.fst.ReadNextArc(path.Arc, s.bytesReader); err != nil {
			return err
		}
	}
	return nil
}

// Search runs the search and returns the top-N results. Mirrors
// {@code TopNSearcher.search}.
func (s *TopNSearcher[T]) Search() (*TopResults[T], error) {
	results := make([]*Result[T], 0, s.topN)
	fstReader := s.fst.GetBytesReader()
	noOutput := s.fst.outputs.GetNoOutput()

	rejectCount := 0

	for len(results) < s.topN {
		// Java's `if (queue == null) break;` corresponds to queueDisabled
		// here. When disabled we exit the outer loop, leaving any
		// partial results in `results`.
		if s.queueDisabled {
			break
		}
		path := s.queuePollFirst()
		if path == nil {
			// There were fewer than topN paths available.
			break
		}
		if s.AcceptPartialPath != nil && !s.AcceptPartialPath(path) {
			continue
		}
		if path.Arc.Label() == END_LABEL {
			// Empty string acceptance.
			path.Input.SetLength(path.Input.Length() - 1)
			results = append(results, &Result[T]{Input: util.DeepCopyOfIntsRef(path.Input.Get()), Output: path.Output})
			continue
		}
		if len(results) == s.topN-1 && s.maxQueueDepth == s.topN {
			// Last path: no need to keep the queue any longer.
			s.queueDisabled = true
			s.queue = nil
		}

		// Walk the "0-output completion" — keep traversing the first
		// NO_OUTPUT arc encountered. Anything else competing is added
		// to the queue.
		for {
			if _, err := s.fst.ReadFirstTargetArc(path.Arc, path.Arc, fstReader); err != nil {
				return nil, err
			}
			foundZero := false
			arcCopyIsPending := false
			for {
				if s.comparator(noOutput, path.Arc.Output()) == 0 {
					if s.queueDisabled {
						foundZero = true
						break
					}
					if !foundZero {
						arcCopyIsPending = true
						foundZero = true
					} else {
						s.addIfCompetitive(path)
					}
				} else if !s.queueDisabled {
					s.addIfCompetitive(path)
				}
				if path.Arc.IsLast() {
					break
				}
				if arcCopyIsPending {
					s.scratchArc.CopyFrom(path.Arc)
					arcCopyIsPending = false
				}
				if _, err := s.fst.ReadNextArc(path.Arc, fstReader); err != nil {
					return nil, err
				}
			}
			if !foundZero {
				return nil, errors.New("fst.TopNSearcher.Search: invariant: foundZero must be true")
			}
			if !s.queueDisabled && !arcCopyIsPending {
				path.Arc.CopyFrom(s.scratchArc)
			}
			if path.Arc.Label() == END_LABEL {
				path.Output = s.fst.outputs.Add(path.Output, path.Arc.Output())
				if s.acceptResult(path.Input.Get(), path.Output) {
					results = append(results, &Result[T]{Input: util.DeepCopyOfIntsRef(path.Input.Get()), Output: path.Output})
				} else {
					rejectCount++
				}
				break
			}
			path.Input.Append(path.Arc.Label())
			path.Output = s.fst.outputs.Add(path.Output, path.Arc.Output())
			if s.AcceptPartialPath != nil && !s.AcceptPartialPath(path) {
				break
			}
		}
	}

	complete := rejectCount+s.topN <= s.maxQueueDepth
	return &TopResults[T]{IsComplete: complete, TopN: results}, nil
}

// acceptResult is the internal entry point that mirrors the protected
// Java overload {@code acceptResult(IntsRef, T)}.
func (s *TopNSearcher[T]) acceptResult(input *util.IntsRef, output T) bool {
	if s.AcceptResult == nil {
		return true
	}
	return s.AcceptResult(input, output)
}

// ShortestPaths is the static helper that builds a TopNSearcher with
// maxQueueDepth == topN, seeds it with the supplied start arc, and
// returns the search results. Mirrors
// {@code Util.shortestPaths(FST<T>, FST.Arc<T>, T, Comparator<T>, int, boolean)}.
func ShortestPaths[T any](
	fst *FST[T],
	fromNode *Arc[T],
	startOutput T,
	comparator OutputComparator[T],
	topN int,
	allowEmptyString bool,
) (*TopResults[T], error) {
	searcher := NewTopNSearcher(fst, topN, topN, comparator)
	if err := searcher.AddStartPaths(fromNode, startOutput, allowEmptyString, util.NewIntsRefBuilder()); err != nil {
		return nil, err
	}
	return searcher.Search()
}
