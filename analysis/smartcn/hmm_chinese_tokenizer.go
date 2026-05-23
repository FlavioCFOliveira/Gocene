// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package smartcn

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/smartcn/hhmm"
	analysisutil "github.com/FlavioCFOliveira/Gocene/analysis/util"
)

// HMMChineseTokenizer tokenises Chinese or mixed Chinese-English text.
//
// The analyser uses probabilistic knowledge to find the optimal word
// segmentation for Simplified Chinese text. The text is first broken into
// sentences via SentenceBreakIterator, then each sentence is segmented into
// words by WordSegmenter.
//
// Go port of org.apache.lucene.analysis.cn.smart.HMMChineseTokenizer
// (Apache Lucene 10.4.0).
//
// Deviation: Java extends SegmentingTokenizerBase; Go embeds
// *analysisutil.SegmentingTokenizerBase and satisfies analysis.Tokenizer via
// method delegation (same pattern as OpenNLPTokenizer).
//
// Deviation: The Java BreakIterator is java.text.BreakIterator.getSentenceInstance(Locale.ROOT).
// Go uses SentenceBreakIterator, a lightweight equivalent.
type HMMChineseTokenizer struct {
	analysis.BaseTokenStream

	base          *analysisutil.SegmentingTokenizerBase
	wordSegmenter *WordSegmenter
	tokens        []*hhmm.SegToken
	tokenIdx      int
	sentenceStart int

	termAtt   analysis.CharTermAttribute
	offsetAtt analysis.OffsetAttribute
	typeAtt   analysis.TypeAttribute
}

// NewHMMChineseTokenizer creates a new HMMChineseTokenizer.
// Returns an error if the underlying WordDictionary cannot be loaded.
func NewHMMChineseTokenizer() (*HMMChineseTokenizer, error) {
	ws, err := NewWordSegmenter()
	if err != nil {
		return nil, err
	}

	t := &HMMChineseTokenizer{
		BaseTokenStream: *analysis.NewBaseTokenStream(),
		wordSegmenter:   ws,
	}

	t.termAtt = analysis.NewCharTermAttribute()
	t.offsetAtt = analysis.NewOffsetAttribute()
	t.typeAtt = analysis.NewTypeAttribute()
	t.AddAttribute(t.termAtt)
	t.AddAttribute(t.offsetAtt)
	t.AddAttribute(t.typeAtt)

	iter := NewSentenceBreakIterator()
	t.base = analysisutil.NewSegmentingTokenizerBase(iter)
	t.base.SetNextSentenceFn = t.setNextSentence
	t.base.IncrementWordFn = t.incrementWord

	return t, nil
}

// SetReader sets the input reader for this tokenizer.
func (t *HMMChineseTokenizer) SetReader(r io.Reader) error {
	t.base.SetReader(r)
	return nil
}

// IncrementToken advances to the next token.
func (t *HMMChineseTokenizer) IncrementToken() (bool, error) {
	t.ClearAttributes()
	return t.base.IncrementToken()
}

// setNextSentence is called by SegmentingTokenizerBase for each sentence.
func (t *HMMChineseTokenizer) setNextSentence(sentenceStart, sentenceEnd int) {
	t.sentenceStart = sentenceStart
	sentence := string(t.base.Buffer[sentenceStart:sentenceEnd])
	tokens, err := t.wordSegmenter.SegmentSentence(sentence, t.base.Offset+sentenceStart)
	if err != nil || len(tokens) == 0 {
		t.tokens = nil
		t.tokenIdx = 0
		return
	}
	t.tokens = tokens
	t.tokenIdx = 0
}

// incrementWord is called by SegmentingTokenizerBase to emit the next word.
func (t *HMMChineseTokenizer) incrementWord() bool {
	if t.tokens == nil || t.tokenIdx >= len(t.tokens) {
		return false
	}
	t.ClearAttributes()
	token := t.tokens[t.tokenIdx]
	t.tokenIdx++

	t.termAtt.SetEmpty()
	t.termAtt.Append([]byte(string(token.CharArray)))
	t.offsetAtt.SetOffset(token.StartOffset, token.EndOffset)
	t.typeAtt.SetType("word")
	return true
}

// Reset resets the tokenizer state.
func (t *HMMChineseTokenizer) Reset() error {
	t.base.Reset()
	t.tokens = nil
	t.tokenIdx = 0
	t.sentenceStart = 0
	return nil
}

// Close releases resources.
func (t *HMMChineseTokenizer) Close() error {
	t.tokens = nil
	t.tokenIdx = 0
	return nil
}

// Ensure HMMChineseTokenizer implements analysis.Tokenizer.
var _ analysis.Tokenizer = (*HMMChineseTokenizer)(nil)
