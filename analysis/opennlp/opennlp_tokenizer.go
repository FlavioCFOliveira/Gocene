// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package opennlp

import (
	"errors"
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/opennlp/tools"
	analysisutil "github.com/FlavioCFOliveira/Gocene/analysis/util"
)

// OpenNLPTokenizer runs the OpenNLP sentence detector and tokenizer,
// storing the sentence index in SentenceAttribute.
//
// Go port of org.apache.lucene.analysis.opennlp.OpenNLPTokenizer
// (Apache Lucene 10.4.0).
//
// Deviation: The Java class uses AttributeFactory to customise attribute
// creation. Go does not use AttributeFactory in the same way; the tokenizer
// uses the default AttributeSource configuration.
type OpenNLPTokenizer struct {
	analysis.BaseTokenStream

	base          *analysisutil.SegmentingTokenizerBase
	tokenizerOp   *tools.NLPTokenizerOp
	sentenceOp    *tools.NLPSentenceDetectorOp
	termAtt       *analysis.CharTermAttributeImpl
	offsetAtt     *analysis.OffsetAttributeImpl
	sentenceAtt   *analysis.SentenceAttributeImpl
	termSpans     []tools.Span
	termNum       int
	sentenceStart int
	sentenceIndex int
}

// NewOpenNLPTokenizer constructs a tokenizer using sentenceOp to detect
// sentence boundaries and tokenizerOp to tokenise within each sentence.
// Both ops are required and must not be nil.
func NewOpenNLPTokenizer(sentenceOp *tools.NLPSentenceDetectorOp, tokenizerOp *tools.NLPTokenizerOp) (*OpenNLPTokenizer, error) {
	if sentenceOp == nil || tokenizerOp == nil {
		return nil, errors.New("OpenNLPTokenizer: both a sentence detector and a tokenizer are required")
	}

	t := &OpenNLPTokenizer{
		BaseTokenStream: *analysis.NewBaseTokenStream(),
		tokenizerOp:     tokenizerOp,
		sentenceOp:      sentenceOp,
		sentenceIndex:   -1,
	}

	t.termAtt = analysis.NewCharTermAttributeImpl()
	t.offsetAtt = analysis.NewOffsetAttributeImpl()
	t.sentenceAtt = analysis.NewSentenceAttributeImpl()
	t.AddAttribute(t.termAtt)
	t.AddAttribute(t.offsetAtt)
	t.AddAttribute(t.sentenceAtt)

	breakIter := NewOpenNLPSentenceBreakIterator(sentenceOp)
	t.base = analysisutil.NewSegmentingTokenizerBase(breakIter)
	t.base.SetNextSentenceFn = t.setNextSentence
	t.base.IncrementWordFn = t.incrementWord

	return t, nil
}

// SetReader sets the input reader for this tokenizer.
func (t *OpenNLPTokenizer) SetReader(r io.Reader) error {
	t.base.SetReader(r)
	return nil
}

// IncrementToken advances to the next token.
func (t *OpenNLPTokenizer) IncrementToken() (bool, error) {
	t.ClearAttributes()
	return t.base.IncrementToken()
}

// setNextSentence is called by SegmentingTokenizerBase for each sentence.
func (t *OpenNLPTokenizer) setNextSentence(sentenceStart, sentenceEnd int) {
	t.sentenceStart = sentenceStart
	sentence := string(t.base.Buffer[sentenceStart:sentenceEnd])
	t.termSpans = t.tokenizerOp.GetTerms(sentence)
	t.termNum = 0
	t.sentenceIndex++
}

// incrementWord is called by SegmentingTokenizerBase to emit the next word.
func (t *OpenNLPTokenizer) incrementWord() bool {
	if t.termSpans == nil || t.termNum >= len(t.termSpans) {
		return false
	}
	t.ClearAttributes()
	term := t.termSpans[t.termNum]
	t.termNum++

	start := t.sentenceStart + term.Start
	end := t.sentenceStart + term.End
	t.termAtt.SetEmpty()
	t.termAtt.Append([]byte(string(t.base.Buffer[start:end])))
	t.offsetAtt.SetOffset(
		t.base.Offset+start,
		t.base.Offset+end,
	)
	t.sentenceAtt.SetSentenceIndex(t.sentenceIndex)
	return true
}

// Reset resets the tokenizer state.
func (t *OpenNLPTokenizer) Reset() error {
	t.base.Reset()
	t.termSpans = nil
	t.termNum = 0
	t.sentenceStart = 0
	t.sentenceIndex = -1
	return nil
}

// Close releases resources.
func (t *OpenNLPTokenizer) Close() error {
	t.termSpans = nil
	t.termNum = 0
	t.sentenceStart = 0
	return nil
}

// Ensure OpenNLPTokenizer implements analysis.Tokenizer.
var _ analysis.Tokenizer = (*OpenNLPTokenizer)(nil)
