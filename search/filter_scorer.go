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
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FilterScorer contains another Scorer, which it uses as its basic source of
// data, possibly transforming the data along the way or providing additional functionality.
//
// Mirrors org.apache.lucene.search.FilterScorer (Lucene 10.5.0).
type FilterScorer struct {
	in Scorer
}

// NewFilterScorer creates a new FilterScorer with a specific weight.
func NewFilterScorer(in Scorer) *FilterScorer {
	if in == nil {
		panic("wrapped Scorer must not be nil")
	}
	return &FilterScorer{in: in}
}

// Score returns the score of the current document.
func (fs *FilterScorer) Score() (float32, error) {
	return fs.in.Score()
}

// DocID returns the doc ID that is currently being scored.
func (fs *FilterScorer) DocID() int {
	return fs.in.DocID()
}

// Iterator returns a DocIdSetIterator over matching documents.
func (fs *FilterScorer) Iterator() util.DocIdSetIterator {
	return fs.in.Iterator()
}

// TwoPhaseIterator returns a TwoPhaseIterator view of this Scorer.
func (fs *FilterScorer) TwoPhaseIterator() *TwoPhaseIterator {
	return fs.in.TwoPhaseIterator()
}

// AdvanceShallow advances to the block of documents that contains target.
func (fs *FilterScorer) AdvanceShallow(target int) (int, error) {
	return fs.in.AdvanceShallow(target)
}

// GetMaxScore returns the maximum score that documents between the last target
// and upTo included.
func (fs *FilterScorer) GetMaxScore(upTo int) (float32, error) {
	return fs.in.GetMaxScore(upTo)
}

// NextDocsAndScores returns a new batch of doc IDs and scores.
func (fs *FilterScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	return fs.in.NextDocsAndScores(upTo, liveDocs, buffer)
}

// Unwrap returns the wrapped Scorer.
func (fs *FilterScorer) Unwrap() Scorer {
	return fs.in
}

// SmoothingScore returns the smoothing score of the current document.
func (fs *FilterScorer) SmoothingScore(docID int) (float32, error) {
	return fs.in.SmoothingScore(docID)
}

// SetMinCompetitiveScore tells the scorer that its iterator may safely ignore
// all documents whose score is less than the given minScore.
func (fs *FilterScorer) SetMinCompetitiveScore(minScore float32) error {
	return fs.in.SetMinCompetitiveScore(minScore)
}

// GetChildren returns child sub-scorers positioned on the current document.
func (fs *FilterScorer) GetChildren() ([]ChildScorable, error) {
	return fs.in.GetChildren()
}
