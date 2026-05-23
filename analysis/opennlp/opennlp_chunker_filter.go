// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package opennlp

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/opennlp/tools"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// OpenNLPChunkerFilter runs an OpenNLP chunker over the token stream,
// replacing the POS tags (stored in TypeAttribute) with chunk tags.
//
// Prerequisite: OpenNLPTokenizer and OpenNLPPOSFilter must precede this
// filter.
//
// Go port of org.apache.lucene.analysis.opennlp.OpenNLPChunkerFilter
// (Apache Lucene 10.4.0).
type OpenNLPChunkerFilter struct {
	*analysis.BaseTokenFilter

	tokenNum                   int
	chunkerOp                  *tools.NLPChunkerOp
	sentenceAttributeExtractor *SentenceAttributeExtractor
	typeAtt                    *analysis.TypeAttributeImpl
}

// NewOpenNLPChunkerFilter constructs a filter that applies chunkerOp to the
// token stream.
func NewOpenNLPChunkerFilter(input analysis.TokenStream, chunkerOp *tools.NLPChunkerOp) *OpenNLPChunkerFilter {
	base := analysis.NewBaseTokenFilter(input)
	sentImpl := analysis.NewSentenceAttributeImpl()
	typeImpl := analysis.NewTypeAttributeImpl()
	base.AddAttribute(sentImpl)
	base.AddAttribute(typeImpl)

	f := &OpenNLPChunkerFilter{
		BaseTokenFilter:            base,
		chunkerOp:                  chunkerOp,
		sentenceAttributeExtractor: NewSentenceAttributeExtractor(input, sentImpl),
		typeAtt:                    typeImpl,
	}
	return f
}

// IncrementToken returns the next chunked token.
func (f *OpenNLPChunkerFilter) IncrementToken() (bool, error) {
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

// nextSentence advances to the next sentence, runs the chunker, and assigns
// chunk tags to the cached AttributeSources.
func (f *OpenNLPChunkerFilter) nextSentence() ([]*util.AttributeSource, error) {
	f.tokenNum = 0
	sentenceAttrs, err := f.sentenceAttributeExtractor.ExtractSentenceAttributes()
	if err != nil {
		return nil, err
	}

	var termList []string
	var posTagList []string
	for _, src := range sentenceAttrs {
		termImpl := src.GetAttribute(analysis.CharTermAttributeType)
		typeImpl := src.GetAttribute(analysis.TypeAttributeType)
		if termImpl != nil {
			termList = append(termList, termImpl.(*analysis.CharTermAttributeImpl).String())
		}
		if typeImpl != nil {
			posTagList = append(posTagList, typeImpl.(*analysis.TypeAttributeImpl).GetType())
		}
	}

	if f.chunkerOp != nil && len(termList) > 0 {
		chunks := f.chunkerOp.GetChunks(termList, posTagList, nil)
		f.assignTokenTypes(chunks)
	}
	return f.sentenceAttributeExtractor.GetSentenceAttributes(), nil
}

// assignTokenTypes writes chunk tags into the cached AttributeSources'
// TypeAttribute entries.
func (f *OpenNLPChunkerFilter) assignTokenTypes(tags []string) {
	attrs := f.sentenceAttributeExtractor.GetSentenceAttributes()
	for i := 0; i < len(tags) && i < len(attrs); i++ {
		typeImpl := attrs[i].GetAttribute(analysis.TypeAttributeType)
		if typeImpl != nil {
			typeImpl.(*analysis.TypeAttributeImpl).SetType(tags[i])
		}
	}
}

// Reset resets the filter state.
func (f *OpenNLPChunkerFilter) Reset() error {
	f.sentenceAttributeExtractor.Reset()
	f.tokenNum = 0
	return nil
}

// Ensure OpenNLPChunkerFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*OpenNLPChunkerFilter)(nil)
