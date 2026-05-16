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

import "github.com/FlavioCFOliveira/Gocene/util"

// HnswBuilder is the interface for building an OnHeapHnswGraph.
// Port of org.apache.lucene.util.hnsw.HnswBuilder (Lucene 10.4.0).
//
// Java's IntHashSet entry-point parameter is replaced by a Go
// map[int]struct{}, mirroring the substitution used elsewhere in
// this package (see hnsw_util.go and update_graphs_utils.go).
//
// Methods returning error correspond to Java's IOException throwers.
type HnswBuilder interface {
	// Build adds all nodes to the graph up to maxOrd (exclusive).
	Build(maxOrd int) (*OnHeapHnswGraph, error)

	// AddGraphNode inserts a doc with a vector value to the graph.
	AddGraphNode(node int) error

	// AddGraphNodeWithEntryPoints inserts a doc with a vector value
	// to the graph, searching on level 0 with the provided entry
	// points.
	AddGraphNodeWithEntryPoints(node int, eps map[int]struct{}) error

	// SetInfoStream sets an info-stream sink for debugging output.
	SetInfoStream(infoStream util.InfoStream)

	// GetGraph returns the in-progress graph.
	GetGraph() *OnHeapHnswGraph

	// GetCompletedGraph returns the final graph. After it is called
	// no further updates are accepted -- subsequent AddGraphNode
	// calls must return an error (Java IllegalStateException). The
	// call may take some time because it triggers final
	// modifications (e.g. patching disconnected components,
	// re-ordering node ids for delta compression).
	GetCompletedGraph() (*OnHeapHnswGraph, error)
}
