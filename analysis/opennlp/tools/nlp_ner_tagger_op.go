// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tools

import "sync"

// NLPNERTaggerOp wraps a TokenNameFinderModel to perform named entity
// recognition.
//
// Go port of org.apache.lucene.analysis.opennlp.tools.NLPNERTaggerOp
// (Apache Lucene 10.4.0).
//
// Deviation: The Java class uses opennlp.tools.namefind.NameFinderME and
// TokenNameFinderModel. In Go, TokenNameFinderModel is a local interface.
// Reset is synchronised to match the Java synchronized method contract.
type NLPNERTaggerOp struct {
	mu    sync.Mutex
	model TokenNameFinderModel
}

// NewNLPNERTaggerOp constructs an op backed by the given model.
func NewNLPNERTaggerOp(model TokenNameFinderModel) *NLPNERTaggerOp {
	return &NLPNERTaggerOp{model: model}
}

// GetNames returns named-entity spans found in words.
func (op *NLPNERTaggerOp) GetNames(words []string) []Span {
	return op.model.Find(words)
}

// Reset clears the adaptive data in the model. Must be called after each
// document to avoid degraded detection rates across documents.
func (op *NLPNERTaggerOp) Reset() {
	op.mu.Lock()
	defer op.mu.Unlock()
	op.model.ClearAdaptiveData()
}
