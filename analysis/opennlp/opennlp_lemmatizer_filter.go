// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package opennlp

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/opennlp/tools"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// OpenNLPLemmatizerFilter runs dictionary-based and/or MaxEnt lemmatizers
// on token streams that have been tokenised and POS-tagged by the OpenNLP
// pipeline.
//
// Go port of org.apache.lucene.analysis.opennlp.OpenNLPLemmatizerFilter
// (Apache Lucene 10.4.0).
type OpenNLPLemmatizerFilter struct {
	*analysis.BaseTokenFilter

	lemmatizerOp               *tools.NLPLemmatizerOp
	termAtt                    *analysis.CharTermAttributeImpl
	keywordAtt                 *analysis.KeywordAttributeImpl
	sentenceTokenAttrs         []*util.AttributeSource
	sentenceTokenAttrsIterPos  int
	sentenceAttributeExtractor *SentenceAttributeExtractor
	lemmas                     []string
	lemmaNum                   int
}

// NewOpenNLPLemmatizerFilter constructs a filter that lemmatises tokens from
// input using lemmatizerOp.
func NewOpenNLPLemmatizerFilter(input analysis.TokenStream, lemmatizerOp *tools.NLPLemmatizerOp) *OpenNLPLemmatizerFilter {
	base := analysis.NewBaseTokenFilter(input)
	f := &OpenNLPLemmatizerFilter{
		BaseTokenFilter: base,
		lemmatizerOp:    lemmatizerOp,
		lemmas:          []string{},
	}

	// Register and retrieve shared attributes.
	termImpl := analysis.NewCharTermAttributeImpl()
	keywordImpl := analysis.NewKeywordAttributeImpl()
	sentImpl := analysis.NewSentenceAttributeImpl()
	base.AddAttribute(termImpl)
	base.AddAttribute(keywordImpl)
	base.AddAttribute(sentImpl)

	f.termAtt = termImpl
	f.keywordAtt = keywordImpl
	f.sentenceAttributeExtractor = NewSentenceAttributeExtractor(input, sentImpl)

	return f
}

// IncrementToken returns the next lemmatised token.
func (f *OpenNLPLemmatizerFilter) IncrementToken() (bool, error) {
	isEndOfCurrentSentence := f.lemmaNum >= len(f.lemmas)
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
	}
	f.ClearAttributes()
	src := f.sentenceTokenAttrs[f.sentenceTokenAttrsIterPos]
	f.sentenceTokenAttrsIterPos++
	src.CopyTo(f.GetAttributeSource())
	if !f.keywordAtt.IsKeywordToken() {
		f.termAtt.SetEmpty()
		f.termAtt.AppendString(f.lemmas[f.lemmaNum])
		f.lemmaNum++
	}
	return true, nil
}

// nextSentence advances to the next sentence and populates lemmas.
func (f *OpenNLPLemmatizerFilter) nextSentence() ([]*util.AttributeSource, error) {
	f.lemmaNum = 0
	var tokenList []string
	var typeList []string

	sentenceAttrs, err := f.sentenceAttributeExtractor.ExtractSentenceAttributes()
	if err != nil {
		return nil, err
	}

	for _, src := range sentenceAttrs {
		kwImpl := src.GetAttribute(analysis.KeywordAttributeType)
		if kw, ok := kwImpl.(*analysis.KeywordAttributeImpl); ok && kw.IsKeywordToken() {
			continue
		}
		termImpl := src.GetAttribute(analysis.CharTermAttributeType)
		typeImpl := src.GetAttribute(analysis.TypeAttributeType)
		if termImpl != nil {
			tokenList = append(tokenList, termImpl.(*analysis.CharTermAttributeImpl).String())
		}
		if typeImpl != nil {
			typeList = append(typeList, typeImpl.(*analysis.TypeAttributeImpl).GetType())
		}
	}

	f.lemmas = f.lemmatizerOp.Lemmatize(tokenList, typeList)
	f.sentenceTokenAttrs = sentenceAttrs
	f.sentenceTokenAttrsIterPos = 0
	return f.sentenceAttributeExtractor.GetSentenceAttributes(), nil
}

// Reset resets the filter state. The caller is responsible for resetting the
// upstream TokenStream before calling this method.
func (f *OpenNLPLemmatizerFilter) Reset() error {
	f.sentenceAttributeExtractor.Reset()
	f.sentenceTokenAttrs = nil
	f.sentenceTokenAttrsIterPos = 0
	f.lemmas = f.lemmas[:0]
	f.lemmaNum = 0
	return nil
}

// Ensure OpenNLPLemmatizerFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*OpenNLPLemmatizerFilter)(nil)
