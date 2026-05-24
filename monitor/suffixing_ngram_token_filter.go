// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// SuffixingNGramTokenFilter generates position-aware NGrams from an input
// token stream, appending a fixed suffix to each.  Tokens longer than
// maxTokenLength are replaced by a wildcard token.
//
// Port of org.apache.lucene.monitor.SuffixingNGramTokenFilter.
//
// Deviation: Gocene's analysis package does not yet expose
// PositionIncrementAttribute / PositionLengthAttribute / KeywordAttribute.
// This implementation handles only the CharTermAttribute and OffsetAttribute
// contract; the position/keyword path is deferred to backlog #2693.
type SuffixingNGramTokenFilter struct {
	input         analysis.TokenStream
	suffix        string
	maxTokenLen   int
	wildcardToken string

	// per-token state
	curTermBuffer []rune
	curTermLen    int
	curGramSize   int
	curPos        int
	seenSuffixes  map[string]struct{}
	seenInfixes   map[string]struct{}

	// output attribute
	termAttr analysis.CharTermAttribute
}

// NewSuffixingNGramTokenFilter creates the filter.
//
//   - suffix: appended to every produced ngram
//   - wildcardToken: emitted when the input token is longer than maxTokenLen
//   - maxTokenLen: tokens longer than this are replaced by wildcardToken
func NewSuffixingNGramTokenFilter(
	input analysis.TokenStream,
	suffix string,
	wildcardToken string,
	maxTokenLen int,
) *SuffixingNGramTokenFilter {
	return &SuffixingNGramTokenFilter{
		input:         input,
		suffix:        suffix,
		wildcardToken: wildcardToken,
		maxTokenLen:   maxTokenLen,
		seenSuffixes:  make(map[string]struct{}),
		seenInfixes:   make(map[string]struct{}),
		termAttr:      analysis.NewCharTermAttribute(),
	}
}

// IncrementToken advances to the next ngram token.
func (f *SuffixingNGramTokenFilter) IncrementToken() (bool, error) {
	for {
		if f.curTermBuffer == nil {
			// Read the next input token.
			ok, err := f.input.IncrementToken()
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
			// TODO: respect KeywordAttribute once available.
			text := f.termAttr.String()
			f.curTermBuffer = []rune(text)
			f.curTermLen = len(f.curTermBuffer)
			f.curGramSize = f.curTermLen
			f.curPos = 0
			return true, nil // emit the original token first (Java does this)
		}

		// Oversized token → emit wildcard.
		if f.curTermLen > f.maxTokenLen {
			f.termAttr.SetEmpty()
			f.termAttr.AppendString(f.wildcardToken)
			f.curTermBuffer = nil
			return true, nil
		}

		// Produce ngrams.
		if f.curGramSize == 0 {
			f.curPos++
			f.curGramSize = f.curTermLen - f.curPos
		}
		if f.curGramSize >= 0 && (f.curPos+f.curGramSize) <= len(f.curTermBuffer) {
			gram := string(f.curTermBuffer[f.curPos:f.curPos+f.curGramSize]) + f.suffix
			isSuffix := f.curGramSize == f.curTermLen-f.curPos
			if isSuffix {
				if _, seen := f.seenSuffixes[gram]; seen {
					f.curTermBuffer = nil
					continue
				}
				f.seenSuffixes[gram] = struct{}{}
			}
			if _, seen := f.seenInfixes[gram]; seen {
				f.curGramSize--
				continue
			}
			f.seenInfixes[gram] = struct{}{}
			f.termAttr.SetEmpty()
			f.termAttr.AppendString(gram)
			f.curGramSize--
			return true, nil
		}
		f.curTermBuffer = nil
	}
}

// End performs end-of-stream operations on the wrapped stream.
func (f *SuffixingNGramTokenFilter) End() error { return f.input.End() }

// Reset resets per-token state and the wrapped stream if it supports Reset.
func (f *SuffixingNGramTokenFilter) Reset() error {
	f.curTermBuffer = nil
	f.seenSuffixes = make(map[string]struct{})
	f.seenInfixes = make(map[string]struct{})
	if r, ok := f.input.(interface{ Reset() error }); ok {
		return r.Reset()
	}
	return nil
}

// Close closes the wrapped stream.
func (f *SuffixingNGramTokenFilter) Close() error { return f.input.Close() }
