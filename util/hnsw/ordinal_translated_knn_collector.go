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

// OrdinalTranslatedKnnCollector wraps a KnnCollector and translates
// the vectorId ordinal to a documentId before delegating to the
// underlying collector. Port of
// org.apache.lucene.util.hnsw.OrdinalTranslatedKnnCollector
// (Lucene 10.4.0).
//
// Lucene extends KnnCollector.Decorator (an inner class of the
// canonical search.KnnCollector). The local KnnCollector stub in
// knn_collector.go does not expose a Decorator helper, so this
// implementation delegates by composition: it embeds the wrapped
// KnnCollector and overrides Collect and TopDocs.
type OrdinalTranslatedKnnCollector struct {
	KnnCollector
	vectorOrdinalToDocID IntToIntFunction
}

// NewOrdinalTranslatedKnnCollector wraps collector so that every
// Collect call translates vectorId via vectorOrdinalToDocID before
// passing through.
func NewOrdinalTranslatedKnnCollector(collector KnnCollector, vectorOrdinalToDocID IntToIntFunction) *OrdinalTranslatedKnnCollector {
	return &OrdinalTranslatedKnnCollector{
		KnnCollector:         collector,
		vectorOrdinalToDocID: vectorOrdinalToDocID,
	}
}

// Collect translates vectorID -> docID before delegating to the
// wrapped collector.
func (c *OrdinalTranslatedKnnCollector) Collect(vectorID int, similarity float32) bool {
	return c.KnnCollector.Collect(c.vectorOrdinalToDocID.Apply(vectorID), similarity)
}

// TopDocs returns the wrapped TopDocs with a fresh TotalHits that
// reports the current visitedCount and the early-termination
// relation. Mirrors Lucene's override exactly.
func (c *OrdinalTranslatedKnnCollector) TopDocs() *TopDocs {
	td := c.KnnCollector.TopDocs()
	rel := EqualTo
	if c.KnnCollector.EarlyTerminated() {
		rel = GreaterThanOrEqualTo
	}
	return NewTopDocs(NewTotalHits(c.KnnCollector.VisitedCount(), rel), td.ScoreDocs)
}
