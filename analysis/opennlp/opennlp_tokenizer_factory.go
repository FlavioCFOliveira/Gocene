// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package opennlp

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/opennlp/tools"
)

// SPINameOpenNLP is the SPI name for OpenNLPTokenizerFactory.
const SPINameOpenNLP = "openNlp"

// OpenNLPTokenizerFactory creates OpenNLPTokenizer instances.
//
// Go port of org.apache.lucene.analysis.opennlp.OpenNLPTokenizerFactory
// (Apache Lucene 10.4.0).
//
// Deviation: The Java factory implements ResourceLoaderAware and loads both
// models from files at inform() time. In Go, models must be registered in the
// tools.OpenNLPOpsFactory cache before use.
//
// Both SentenceModelName and TokenizerModelName are required.
type OpenNLPTokenizerFactory struct {
	SentenceModelName  string
	TokenizerModelName string
}

// NewOpenNLPTokenizerFactory creates a factory. Both sentenceModelName and
// tokenizerModelName must be non-empty.
func NewOpenNLPTokenizerFactory(sentenceModelName, tokenizerModelName string) *OpenNLPTokenizerFactory {
	if sentenceModelName == "" || tokenizerModelName == "" {
		panic("OpenNLPTokenizerFactory: both sentenceModelName and tokenizerModelName are required")
	}
	return &OpenNLPTokenizerFactory{
		SentenceModelName:  sentenceModelName,
		TokenizerModelName: tokenizerModelName,
	}
}

// Create creates a new OpenNLPTokenizer.
func (f *OpenNLPTokenizerFactory) Create() analysis.Tokenizer {
	sentenceOp := tools.GetSentenceDetector(f.SentenceModelName)
	tokenizerOp := tools.GetTokenizer(f.TokenizerModelName)
	tok, err := NewOpenNLPTokenizer(sentenceOp, tokenizerOp)
	if err != nil {
		panic("OpenNLPTokenizerFactory.Create: " + err.Error())
	}
	return tok
}

// Ensure OpenNLPTokenizerFactory implements analysis.TokenizerFactory.
var _ analysis.TokenizerFactory = (*OpenNLPTokenizerFactory)(nil)
