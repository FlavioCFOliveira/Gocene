// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package opennlp

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/opennlp/tools"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// OpenNLPPOSFilter runs an OpenNLP POS tagger over the token stream, storing
// POS tags in the TypeAttribute.
//
// Go port of org.apache.lucene.analysis.opennlp.OpenNLPPOSFilter
// (Apache Lucene 10.4.0).
type OpenNLPPOSFilter struct {
	*analysis.BaseTokenFilter

	tokenNum                   int
	posTaggerOp                *tools.NLPPOSTaggerOp
	sentenceAttributeExtractor *SentenceAttributeExtractor
}

// NewOpenNLPPOSFilter constructs a filter that applies posTaggerOp to the
// token stream.
func NewOpenNLPPOSFilter(input analysis.TokenStream, posTaggerOp *tools.NLPPOSTaggerOp) *OpenNLPPOSFilter {
	base := analysis.NewBaseTokenFilter(input)
	sentImpl := analysis.NewSentenceAttributeImpl()
	typeImpl := analysis.NewTypeAttributeImpl()
	base.AddAttribute(sentImpl)
	base.AddAttribute(typeImpl)

	f := &OpenNLPPOSFilter{
		BaseTokenFilter:            base,
		posTaggerOp:                posTaggerOp,
		sentenceAttributeExtractor: NewSentenceAttributeExtractor(input, sentImpl),
	}
	return f
}

// IncrementToken returns the next POS-tagged token.
func (f *OpenNLPPOSFilter) IncrementToken() (bool, error) {
	sentenceTokenAttrs := f.sentenceAttributeExtractor.GetSentenceAttributes()
	isEndOfCurrentSentence := f.tokenNum >= len(sentenceTokenAttrs)
	if isEndOfCurrentSentence {
		if f.sentenceAttributeExtractor.AllSentencesProcessed() {
			return false, nil
		}
		next, err := f.nextSentence()
		if err != nil {
			return false, err
		}
		if len(next) == 0 {
			return false, nil
		}
		sentenceTokenAttrs = f.sentenceAttributeExtractor.GetSentenceAttributes()
	}
	f.ClearAttributes()
	sentenceTokenAttrs[f.tokenNum].CopyTo(f.GetAttributeSource())
	f.tokenNum++
	return true, nil
}

// nextSentence advances to the next sentence, runs the POS tagger, and
// assigns POS tags to the cached AttributeSources.
func (f *OpenNLPPOSFilter) nextSentence() ([]*util.AttributeSource, error) {
	f.tokenNum = 0
	sentenceAttrs, err := f.sentenceAttributeExtractor.ExtractSentenceAttributes()
	if err != nil {
		return nil, err
	}

	var termList []string
	for _, src := range sentenceAttrs {
		termImpl := src.GetAttribute(analysis.CharTermAttributeType)
		if termImpl != nil {
			termList = append(termList, termImpl.(*analysis.CharTermAttributeImpl).String())
		}
	}

	if len(termList) > 0 {
		tags := f.posTaggerOp.GetPOSTags(termList)
		f.assignTokenTypes(tags)
	}
	return f.sentenceAttributeExtractor.GetSentenceAttributes(), nil
}

// assignTokenTypes writes POS tags into the cached AttributeSources'
// TypeAttribute entries.
func (f *OpenNLPPOSFilter) assignTokenTypes(tags []string) {
	attrs := f.sentenceAttributeExtractor.GetSentenceAttributes()
	for i := 0; i < len(tags) && i < len(attrs); i++ {
		typeImpl := attrs[i].GetAttribute(analysis.TypeAttributeType)
		if typeImpl != nil {
			typeImpl.(*analysis.TypeAttributeImpl).SetType(tags[i])
		}
	}
}

// Reset resets the filter state.
func (f *OpenNLPPOSFilter) Reset() error {
	f.sentenceAttributeExtractor.Reset()
	f.tokenNum = 0
	return nil
}

// Ensure OpenNLPPOSFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*OpenNLPPOSFilter)(nil)
