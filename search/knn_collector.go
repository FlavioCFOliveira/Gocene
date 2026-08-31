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
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/search/knn"
)

// KnnCollector is a knn collector used for gathering kNN results and providing topDocs from the
// gathered neighbors.
//
// It is the Go port of org.apache.lucene.search.KnnCollector (Lucene 10.4.0).
type KnnCollector interface {
	// EarlyTerminated reports if search visits too many documents, the results collector will terminate early.
	// Usually, this is due to some restricted filter on the document set.
	//
	// When collection is earlyTerminated, the results are not a correct representation of k
	// nearest neighbors.
	EarlyTerminated() bool

	// IncVisitedCount increments the visited vector count. count must be greater than 0.
	IncVisitedCount(count int)

	// VisitedCount returns the current visited vector count.
	VisitedCount() int64

	// VisitLimit returns the visited vector limit.
	VisitLimit() int64

	// K returns the expected number of collected results.
	K() int

	// Collect collects the provided docID and include in the result set.
	// Returns true if the vector is collected.
	Collect(docID int, similarity float32) bool

	// MinCompetitiveSimilarity is utilized during search to ensure only competitive results are explored.
	//
	// Consequently, if this results collector wants to collect `k` results, this should return
	// float32(math.Inf(-1)) when not full.
	//
	// When full, the minimum score should be returned.
	MinCompetitiveSimilarity() float32

	// TopDocs drains the collected nearest kNN results and returns them in a new TopDocs
	// collection, ordered by score descending. NOTE: This is generally a destructive action and the
	// collector should not be used after TopDocs() is called.
	TopDocs() *TopDocs

	// GetSearchStrategy returns the search strategy used by this collector, can be nil.
	GetSearchStrategy() knn.KnnSearchStrategy
}

// KnnCollectorDecorator is the base class for decorators of KnnCollector objects, which extend
// the object with new behaviors.
//
// It is the Go port of KnnCollector.Decorator (Lucene 10.4.0).
type KnnCollectorDecorator struct {
	collector KnnCollector
}

// NewKnnCollectorDecorator constructs a new KnnCollectorDecorator.
func NewKnnCollectorDecorator(collector KnnCollector) *KnnCollectorDecorator {
	return &KnnCollectorDecorator{
		collector: collector,
	}
}

func (d *KnnCollectorDecorator) EarlyTerminated() bool {
	return d.collector.EarlyTerminated()
}

func (d *KnnCollectorDecorator) IncVisitedCount(count int) {
	d.collector.IncVisitedCount(count)
}

func (d *KnnCollectorDecorator) VisitedCount() int64 {
	return d.collector.VisitedCount()
}

func (d *KnnCollectorDecorator) VisitLimit() int64 {
	return d.collector.VisitLimit()
}

func (d *KnnCollectorDecorator) K() int {
	return d.collector.K()
}

func (d *KnnCollectorDecorator) Collect(docID int, similarity float32) bool {
	return d.collector.Collect(docID, similarity)
}

func (d *KnnCollectorDecorator) MinCompetitiveSimilarity() float32 {
	return d.collector.MinCompetitiveSimilarity()
}

func (d *KnnCollectorDecorator) TopDocs() *TopDocs {
	return d.collector.TopDocs()
}

func (d *KnnCollectorDecorator) GetSearchStrategy() knn.KnnSearchStrategy {
	return d.collector.GetSearchStrategy()
}
