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

// Package analyzing implements
// org.apache.lucene.search.suggest.analyzing: the AnalyzingSuggester family
// and the FST-based helpers it relies on.
package analyzing

// This file ports org.apache.lucene.search.suggest.analyzing.FSTUtil from
// Apache Lucene 10.4.0.

import (
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
	fstp "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// FSTPath holds a (automaton-state, fst-arc, output, input) quadruple
// accumulated while intersecting an Automaton with an FST. It is the Go
// counterpart of org.apache.lucene.search.suggest.analyzing.FSTUtil.Path<T>.
type FSTPath[T any] struct {
	// State is the automaton node where this path ends.
	State int
	// FSTNode is the last FST arc of the path.
	FSTNode *fstp.Arc[T]
	// Output is the accumulated FST output along the path.
	Output T
	// Input is the accumulated integer labels.
	Input *util.IntsRefBuilder
}

// IntersectPrefixPaths enumerates all minimal prefix paths in automaton a
// that also exist in the FST fst, accumulating the FST arc and output for
// each accepting prefix. It is a faithful port of
// org.apache.lucene.search.suggest.analyzing.FSTUtil.intersectPrefixPaths.
//
// The automaton must be deterministic (a.IsDeterministic() == true).
func IntersectPrefixPaths[T any](a *automaton.Automaton, fst *fstp.FST[T]) ([]*FSTPath[T], error) {
	var queue []*FSTPath[T]
	var endNodes []*FSTPath[T]

	if a.NumStates() == 0 {
		return endNodes, nil
	}

	firstArc := new(fstp.Arc[T])
	fst.GetFirstArc(firstArc)

	queue = append(queue, &FSTPath[T]{
		State:   0,
		FSTNode: firstArc,
		Output:  fst.Outputs().GetNoOutput(),
		Input:   util.NewIntsRefBuilder(),
	})

	scratchArc := new(fstp.Arc[T])
	fstReader := fst.GetBytesReader()
	var t automaton.Transition

	for len(queue) > 0 {
		// Pop last (DFS order).
		n := len(queue) - 1
		path := queue[n]
		queue = queue[:n]

		if a.IsAccept(path.State) {
			endNodes = append(endNodes, path)
			// Any extension is also accepted; stop here.
			continue
		}

		currentInput := path.Input
		count := a.InitTransition(path.State, &t)
		for i := 0; i < count; i++ {
			a.GetNextTransition(&t)
			if t.Min == t.Max {
				// Single-label transition.
				nextArc, err := fst.FindTargetArc(t.Min, path.FSTNode, scratchArc, fstReader)
				if err != nil {
					return nil, err
				}
				if nextArc != nil {
					newInput := util.NewIntsRefBuilder()
					newInput.CopyIntsRef(currentInput.Get())
					newInput.Append(t.Min)
					copied := new(fstp.Arc[T])
					copied.CopyFrom(nextArc)
					queue = append(queue, &FSTPath[T]{
						State:   t.Dest,
						FSTNode: copied,
						Output:  fst.Outputs().Add(path.Output, nextArc.Output()),
						Input:   newInput,
					})
				}
			} else {
				// Range transition: walk FST arcs in [t.Min, t.Max].
				nextArc, err := fstp.ReadCeilArc(t.Min, fst, path.FSTNode, scratchArc, fstReader)
				if err != nil {
					return nil, err
				}
				for nextArc != nil && nextArc.Label() <= t.Max {
					newInput := util.NewIntsRefBuilder()
					newInput.CopyIntsRef(currentInput.Get())
					newInput.Append(nextArc.Label())
					copied := new(fstp.Arc[T])
					copied.CopyFrom(nextArc)
					queue = append(queue, &FSTPath[T]{
						State:   t.Dest,
						FSTNode: copied,
						Output:  fst.Outputs().Add(path.Output, nextArc.Output()),
						Input:   newInput,
					})
					prevLabel := nextArc.Label()
					if nextArc.IsLast() {
						nextArc = nil
					} else {
						nextArc, err = fst.ReadNextRealArc(nextArc, fstReader)
						if err != nil {
							return nil, err
						}
						_ = prevLabel
					}
				}
			}
		}
	}
	return endNodes, nil
}
