// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tools

import "sync"

// NLPPOSTaggerOp wraps a POSModel to tag words with parts of speech.
//
// Go port of org.apache.lucene.analysis.opennlp.tools.NLPPOSTaggerOp
// (Apache Lucene 10.4.0).
//
// Deviation: The Java class uses opennlp.tools.postag.POSTaggerME and
// POSModel. In Go, POSModel is a local interface. Access is synchronised
// to match the Java synchronized method contract.
type NLPPOSTaggerOp struct {
	mu    sync.Mutex
	model POSModel
}

// NewNLPPOSTaggerOp constructs an op backed by the given model.
func NewNLPPOSTaggerOp(model POSModel) *NLPPOSTaggerOp {
	return &NLPPOSTaggerOp{model: model}
}

// GetPOSTags returns POS tags for the given words.
func (op *NLPPOSTaggerOp) GetPOSTags(words []string) []string {
	op.mu.Lock()
	defer op.mu.Unlock()
	return op.model.Tag(words)
}
