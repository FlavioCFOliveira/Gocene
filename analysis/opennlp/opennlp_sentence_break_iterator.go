// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package opennlp

import (
	"github.com/FlavioCFOliveira/Gocene/analysis/opennlp/tools"
	analysisutil "github.com/FlavioCFOliveira/Gocene/analysis/util"
)

// OpenNLPSentenceBreakIterator is a BreakIterator that splits sentences using
// an OpenNLP sentence detection model.
//
// Go port of
// org.apache.lucene.analysis.opennlp.OpenNLPSentenceBreakIterator
// (Apache Lucene 10.4.0).
//
// Deviation: The Java class extends java.text.BreakIterator and works with
// java.text.CharacterIterator. In Go, BreakIterator is the interface defined
// in analysis/util and operates on []rune slices.
type OpenNLPSentenceBreakIterator struct {
	sentenceOp      *tools.NLPSentenceDetectorOp
	text            []rune
	length          int
	sentenceStarts  []int
	currentSentence int
}

// NewOpenNLPSentenceBreakIterator constructs a break iterator backed by
// sentenceOp.
func NewOpenNLPSentenceBreakIterator(sentenceOp *tools.NLPSentenceDetectorOp) *OpenNLPSentenceBreakIterator {
	return &OpenNLPSentenceBreakIterator{
		sentenceOp: sentenceOp,
	}
}

// SetText configures the iterator to operate on buf[0:length] (rune slice).
// Sentence boundaries are computed immediately.
func (it *OpenNLPSentenceBreakIterator) SetText(buf []rune, length int) {
	it.text = buf
	it.length = length
	it.currentSentence = 0

	// Convert the rune slice to a string for the NLP model.
	text := string(buf[:length])
	spans := it.sentenceOp.SplitSentences(text)

	it.sentenceStarts = make([]int, len(spans))
	for i, sp := range spans {
		// Convert byte offset to rune offset.
		it.sentenceStarts[i] = sp.Start
	}
}

// Current returns the current sentence boundary index.
func (it *OpenNLPSentenceBreakIterator) Current() int {
	if it.currentSentence >= len(it.sentenceStarts) {
		return analysisutil.BreakDone
	}
	return it.sentenceStarts[it.currentSentence]
}

// Next advances to the next sentence boundary and returns the end of the
// current sentence (start of next, or length of text if last sentence).
// Returns BreakDone when all sentences have been traversed.
func (it *OpenNLPSentenceBreakIterator) Next() int {
	if it.currentSentence >= len(it.sentenceStarts) {
		return analysisutil.BreakDone
	}
	it.currentSentence++
	if it.currentSentence >= len(it.sentenceStarts) {
		return it.length
	}
	return it.sentenceStarts[it.currentSentence]
}

// Ensure OpenNLPSentenceBreakIterator implements analysisutil.BreakIterator.
var _ analysisutil.BreakIterator = (*OpenNLPSentenceBreakIterator)(nil)
