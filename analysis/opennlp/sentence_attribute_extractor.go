// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package opennlp provides OpenNLP-based analysis components.
//
// Go port of org.apache.lucene.analysis.opennlp (Apache Lucene 10.4.0).
//
// Deviation: The Java implementation depends on the OpenNLP library
// (opennlp.tools.*). Go has no CGO-free equivalent of the OpenNLP Java
// library. All NLP model types are modelled as Go interfaces in the
// analysis/opennlp/tools sub-package; callers must supply concrete
// implementations backed by an actual NLP engine.
package opennlp

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SentenceAttributeExtractor iterates through sentence tokens and caches
// their attributes.
//
// Go port of org.apache.lucene.analysis.opennlp.SentenceAttributeExtractor
// (Apache Lucene 10.4.0).
type SentenceAttributeExtractor struct {
	input               analysis.TokenStream
	sentenceAtt         analysis.SentenceAttribute
	sentenceTokenAttrs  []*util.AttributeSource
	prevAttributeSource *util.AttributeSource
	currSentence        int
	hasNextToken        bool
}

// NewSentenceAttributeExtractor constructs a SentenceAttributeExtractor that
// reads tokens from input and tracks sentence boundaries via sentenceAtt.
func NewSentenceAttributeExtractor(input analysis.TokenStream, sentenceAtt analysis.SentenceAttribute) *SentenceAttributeExtractor {
	return &SentenceAttributeExtractor{
		input:        input,
		sentenceAtt:  sentenceAtt,
		hasNextToken: true,
	}
}

// ExtractSentenceAttributes advances through the input stream, collecting all
// tokens that belong to the current sentence into the internal cache.
// Returns the cached []*util.AttributeSource for the sentence just completed.
//
// The method mirrors Lucene's do-while loop: it reads one token past the
// current sentence boundary so the extractor knows where the next sentence
// begins.
func (e *SentenceAttributeExtractor) ExtractSentenceAttributes() ([]*util.AttributeSource, error) {
	e.sentenceTokenAttrs = e.sentenceTokenAttrs[:0]
	var hasNext bool
	for {
		var err error
		e.hasNextToken, err = e.input.IncrementToken()
		if err != nil {
			return nil, err
		}
		currSentenceTmp := e.sentenceAtt.GetSentenceIndex()
		hasNext = e.currSentence == currSentenceTmp && e.hasNextToken
		e.currSentence = currSentenceTmp
		if e.prevAttributeSource != nil {
			e.sentenceTokenAttrs = append(e.sentenceTokenAttrs, e.prevAttributeSource)
		}
		src, ok := e.input.(interface{ GetAttributeSource() *util.AttributeSource })
		if !ok {
			break
		}
		e.prevAttributeSource = src.GetAttributeSource().CloneAttributes()
		if !hasNext {
			break
		}
	}
	return e.sentenceTokenAttrs, nil
}

// GetSentenceAttributes returns the cached attribute sources for the most
// recently extracted sentence. The slice is valid until the next call to
// ExtractSentenceAttributes.
func (e *SentenceAttributeExtractor) GetSentenceAttributes() []*util.AttributeSource {
	return e.sentenceTokenAttrs
}

// AllSentencesProcessed reports whether the input stream is exhausted.
func (e *SentenceAttributeExtractor) AllSentencesProcessed() bool {
	return !e.hasNextToken
}

// Reset resets the extractor state so it can process a new stream.
func (e *SentenceAttributeExtractor) Reset() {
	e.hasNextToken = true
	e.sentenceTokenAttrs = e.sentenceTokenAttrs[:0]
	e.currSentence = 0
	e.prevAttributeSource = nil
}
