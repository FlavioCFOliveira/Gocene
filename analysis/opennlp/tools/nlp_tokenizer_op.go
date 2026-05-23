// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tools

// NLPTokenizerOp wraps a TokenizerModel to tokenise sentences into terms.
//
// Go port of org.apache.lucene.analysis.opennlp.tools.NLPTokenizerOp
// (Apache Lucene 10.4.0).
//
// Deviation: The Java class uses opennlp.tools.tokenize.TokenizerME and
// TokenizerModel. In Go, TokenizerModel is a local interface. When no model
// is provided, the entire sentence is returned as a single span.
type NLPTokenizerOp struct {
	model TokenizerModel
}

// NewNLPTokenizerOpWithModel constructs an op backed by the given model.
func NewNLPTokenizerOpWithModel(model TokenizerModel) *NLPTokenizerOp {
	return &NLPTokenizerOp{model: model}
}

// NewNLPTokenizerOp constructs an op with no model; each sentence is a single
// token.
func NewNLPTokenizerOp() *NLPTokenizerOp {
	return &NLPTokenizerOp{}
}

// GetTerms returns the token spans within sentence. If no model was provided,
// returns a single span covering the full sentence.
func (op *NLPTokenizerOp) GetTerms(sentence string) []Span {
	if op.model == nil {
		return []Span{{Start: 0, End: len(sentence)}}
	}
	return op.model.TokenizePos(sentence)
}
