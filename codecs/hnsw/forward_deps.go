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

// This file consolidates the minimal placeholder types the codecs/hnsw
// ports reference but whose canonical implementations live in Lucene
// packages that have not yet been ported to Gocene. Each placeholder
// documents the upstream Java type and the roadmap sprint that will
// supply the real port. Keeping all forward declarations in a single
// file makes the eventual cut-over a single search-and-replace per
// type.

// MergeState is a placeholder for org.apache.lucene.index.MergeState
// (Lucene 10.4.0). The Lucene reference carries per-segment readers,
// doc-id maps, deletes, FieldInfos, and merge progress tracking; the
// Gocene equivalent will be supplied by a later index-merge sprint.
//
// Sprint 19 only needs the type to satisfy [FlatVectorsWriter.MergeOneFieldToIndex]
// signatures byte-for-byte with the Java original. Codec callers in
// this sprint do not invoke MergeOneFieldToIndex; the concrete
// Lucene99HnswVectorsFormat / Lucene104ScalarQuantizedVectorsFormat
// writers wired in later sprints will use the real MergeState once the
// merge subsystem is ported.
//
// TODO(rmp): replace with the canonical index.MergeState type when the
// merge-state sprint (tracked separately in the roadmap) lands.
type MergeState struct{}

// DocsWithFieldSet is a placeholder for
// org.apache.lucene.index.DocsWithFieldSet (Lucene 10.4.0). The Lucene
// reference is a growing bitset of document ids that have a value for
// a given field; consumers use it to skip empty docs at flush/merge
// time.
//
// Sprint 19 only requires the type as the return shape of
// [FlatFieldVectorsWriter.GetDocsWithFieldSet] so the abstract base
// matches the Java surface. Concrete subclasses produced by later
// sprints will return a real backing bitset. The placeholder exposes
// only a Cardinality accessor so test peers in this sprint can assert
// "no docs" semantics without depending on a richer API.
//
// TODO(rmp): replace with the canonical index.DocsWithFieldSet type
// when the docs-with-field-set sprint lands.
type DocsWithFieldSet struct {
	cardinality int
}

// NewDocsWithFieldSet constructs an empty DocsWithFieldSet placeholder.
func NewDocsWithFieldSet() *DocsWithFieldSet {
	return &DocsWithFieldSet{}
}

// Cardinality returns the number of doc ids in the set. The placeholder
// implementation only tracks a counter; concrete subclasses ported in
// later sprints will store the full bitset.
func (s *DocsWithFieldSet) Cardinality() int {
	if s == nil {
		return 0
	}
	return s.cardinality
}

// Add records that the given doc id is present. The placeholder only
// updates the counter; the real type will maintain the underlying
// bitset.
func (s *DocsWithFieldSet) Add(docID int) {
	_ = docID
	s.cardinality++
}
