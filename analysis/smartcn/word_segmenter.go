// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package smartcn

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis/smartcn/hhmm"
)

// WordSegmenter segments a Chinese sentence into words using HHMMSegmenter.
//
// Go port of org.apache.lucene.analysis.cn.smart.WordSegmenter.
type WordSegmenter struct {
	hhmmSegmenter *hhmm.HHMMSegmenter
	tokenFilter   *hhmm.SegTokenFilter
}

// NewWordSegmenter creates a new WordSegmenter.
func NewWordSegmenter() (*WordSegmenter, error) {
	seg, err := hhmm.NewHHMMSegmenter()
	if err != nil {
		return nil, fmt.Errorf("WordSegmenter: %w", err)
	}
	return &WordSegmenter{
		hhmmSegmenter: seg,
		tokenFilter:   hhmm.NewSegTokenFilter(),
	}, nil
}

// SegmentSentence segments sentence into words and adjusts offsets by
// startOffset. The returned slice excludes the SENTENCE_BEGIN and
// SENTENCE_END sentinels.
func (ws *WordSegmenter) SegmentSentence(sentence string, startOffset int) ([]*hhmm.SegToken, error) {
	segTokenList, err := ws.hhmmSegmenter.Process(sentence)
	if err != nil {
		return nil, err
	}

	// Exclude the two sentinel tokens (begin and end).
	var result []*hhmm.SegToken
	if len(segTokenList) > 2 {
		result = segTokenList[1 : len(segTokenList)-1]
	}

	for _, st := range result {
		ws.convertSegToken(st, sentence, startOffset)
	}
	return result, nil
}

// convertSegToken adjusts offsets and normalises a SegToken.
func (ws *WordSegmenter) convertSegToken(st *hhmm.SegToken, sentence string, sentenceStartOffset int) {
	runes := []rune(sentence)
	switch st.WordType {
	case WordTypeString, WordTypeNumber, WordTypeFullwidthNumber, WordTypeFullwidthString:
		// Extract the actual text from the sentence.
		start := st.StartOffset
		end := st.EndOffset
		if start >= 0 && end <= len(runes) && start < end {
			st.CharArray = runes[start:end]
		}
	}
	ws.tokenFilter.Filter(st)
	st.StartOffset += sentenceStartOffset
	st.EndOffset += sentenceStartOffset
}
