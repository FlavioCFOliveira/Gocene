// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tools

// NLPSentenceDetectorOp wraps a SentenceModel to detect sentence boundaries.
//
// Go port of org.apache.lucene.analysis.opennlp.tools.NLPSentenceDetectorOp
// (Apache Lucene 10.4.0).
//
// Deviation: The Java class uses opennlp.tools.sentdetect.SentenceDetectorME
// and SentenceModel. In Go, SentenceModel is a local interface. When no model
// is provided, the entire input is treated as a single sentence.
type NLPSentenceDetectorOp struct {
	model SentenceModel
}

// NewNLPSentenceDetectorOpWithModel constructs an op backed by the given model.
// If model is nil, the entire input is treated as one sentence.
func NewNLPSentenceDetectorOpWithModel(model SentenceModel) *NLPSentenceDetectorOp {
	return &NLPSentenceDetectorOp{model: model}
}

// NewNLPSentenceDetectorOp constructs an op with no model; the entire input
// is treated as a single sentence.
func NewNLPSentenceDetectorOp() *NLPSentenceDetectorOp {
	return &NLPSentenceDetectorOp{}
}

// SplitSentences splits line into sentence spans. If no model was provided,
// returns a single span covering the full input.
func (op *NLPSentenceDetectorOp) SplitSentences(line string) []Span {
	if op.model != nil {
		return op.model.SplitSentences(line)
	}
	return []Span{{Start: 0, End: len(line)}}
}
