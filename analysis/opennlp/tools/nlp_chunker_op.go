// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tools

import "sync"

// NLPChunkerOp wraps a ChunkerModel to assign chunk tags to words.
//
// Go port of org.apache.lucene.analysis.opennlp.tools.NLPChunkerOp
// (Apache Lucene 10.4.0).
//
// Deviation: The Java class uses opennlp.tools.chunker.ChunkerME and
// ChunkerModel. In Go, ChunkerModel is a local interface. Access is
// synchronised to match the Java synchronized method contract.
type NLPChunkerOp struct {
	mu    sync.Mutex
	model ChunkerModel
}

// NewNLPChunkerOp constructs an op backed by the given model.
func NewNLPChunkerOp(model ChunkerModel) *NLPChunkerOp {
	return &NLPChunkerOp{model: model}
}

// GetChunks returns chunk tags for the given words and POS tags.
// If probs is non-nil, it is populated with the probability of each chunk.
func (op *NLPChunkerOp) GetChunks(words, tags []string, probs []float64) []string {
	op.mu.Lock()
	defer op.mu.Unlock()
	chunks := op.model.Chunk(words, tags)
	if probs != nil {
		op.model.Probs(probs)
	}
	return chunks
}
