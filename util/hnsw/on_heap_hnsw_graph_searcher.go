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

package hnsw

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// onHeapCursor is the local seek state used by the thread-safe
// OnHeapHnswGraph searcher. The Java reference holds these fields in
// the private inner class HnswGraphSearcher.OnHeapHnswGraphSearcher;
// the Go port stores them on the same struct as the base searcher and
// installs custom seek/next policy functions instead.
type onHeapCursor struct {
	cur  *NeighborArray
	upto int
}

// onHeapSeek replaces the default seek policy with a thread-local
// cursor over the OnHeapHnswGraph neighbour array. This avoids
// touching the graph's intrinsic cursor and is the reason the Java
// HnswGraphSearcher.search(OnHeapHnswGraph) overload is documented
// as thread-safe.
func onHeapSeek(s *HnswGraphSearcher, graph HnswGraph, level, target int) error {
	oh, ok := graph.(*OnHeapHnswGraph)
	if !ok {
		return fmt.Errorf("hnsw: onHeapSeek requires *OnHeapHnswGraph, got %T", graph)
	}
	cur := oh.GetNeighbors(level, target)
	c, _ := s.userData.(*onHeapCursor)
	if c == nil {
		c = &onHeapCursor{}
		s.userData = c
	}
	c.cur = cur
	c.upto = -1
	return nil
}

// onHeapNext advances the thread-local cursor by one step.
func onHeapNext(s *HnswGraphSearcher, _ HnswGraph) (int, error) {
	c, _ := s.userData.(*onHeapCursor)
	if c == nil || c.cur == nil {
		return util.NO_MORE_DOCS, nil
	}
	c.upto++
	if c.upto < c.cur.Size() {
		return c.cur.Nodes()[c.upto], nil
	}
	return util.NO_MORE_DOCS, nil
}

// NewOnHeapHnswGraphSearcher constructs a searcher that walks an
// OnHeapHnswGraph via a thread-local cursor, leaving the graph's
// intrinsic seek state untouched. The returned searcher is the
// thread-safe analogue used by [SearchWithOnHeapGraph].
//
// candidates and visited are owned by the searcher just like in the
// non-concurrent variant; callers must allocate one searcher per
// goroutine.
func NewOnHeapHnswGraphSearcher(candidates *NeighborQueue, visited util.BitSet) *HnswGraphSearcher {
	s := NewHnswGraphSearcher(candidates, visited)
	s.seek = onHeapSeek
	s.next = onHeapNext
	s.userData = &onHeapCursor{}
	return s
}
