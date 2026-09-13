// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"reflect"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
)

// SuffixingNGramTokenFilter generates position-aware NGrams from an input token
// stream, appending a fixed suffix to each. Tokens longer than maxTokenLength
// are replaced by a wildcard token.
//
// Port of org.apache.lucene.monitor.SuffixingNGramTokenFilter (Apache Lucene
// 10.5.0).
//
// PORT NOTE: Lucene's CharTermAttribute is a UTF-16 char buffer, so it
// distinguishes curTermLength (buffer units) from curCodePointCount (code
// points) and steps between the two with Character.offsetByCodePoints.
// Gocene's analysis.CharTermAttribute is a UTF-8 byte buffer, so curTermLength
// counts bytes and the same two-index arithmetic is carried out over bytes.
type SuffixingNGramTokenFilter struct {
	*analysis.BaseTokenFilter

	suffix         string
	maxTokenLength int
	anyToken       string

	curTermBuffer     []byte
	curTermLength     int
	curCodePointCount int
	curGramSize       int
	curPos            int
	curPosInc         int
	curPosLen         int
	tokStart          int
	tokEnd            int

	termAtt    analysis.CharTermAttribute
	posIncAtt  tokenattributes.PositionIncrementAttribute
	posLenAtt  analysis.PositionLengthAttribute
	offsetAtt  analysis.OffsetAttribute
	keywordAtt analysis.KeywordAttribute

	seenSuffixes map[string]struct{}
	seenInfixes  map[string]struct{}
}

// NewSuffixingNGramTokenFilter creates a SuffixingNGramTokenFilter.
//
//   - input: TokenStream holding the input to be tokenized
//   - suffix: a string to suffix to all ngrams
//   - wildcardToken: a token to emit if the input token is longer than maxTokenLength
//   - maxTokenLength: tokens longer than this will not be ngrammed
func NewSuffixingNGramTokenFilter(
	input analysis.TokenStream,
	suffix string,
	wildcardToken string,
	maxTokenLength int,
) *SuffixingNGramTokenFilter {
	f := &SuffixingNGramTokenFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		suffix:          suffix,
		anyToken:        wildcardToken,
		maxTokenLength:  maxTokenLength,
		seenSuffixes:    make(map[string]struct{}, 1024),
		seenInfixes:     make(map[string]struct{}, 1024),
	}

	src := f.GetAttributeSource()
	src.AddAttribute(analysis.CharTermAttributeType)
	src.AddAttribute(analysis.OffsetAttributeType)
	src.AddAttribute(analysis.KeywordAttributeType)
	src.AddAttribute(reflect.TypeOf((*tokenattributes.PositionIncrementAttribute)(nil)).Elem())
	src.AddAttribute(analysis.PositionLengthAttributeType)

	f.termAtt, _ = src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	f.offsetAtt, _ = src.GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	f.keywordAtt, _ = src.GetAttribute(analysis.KeywordAttributeType).(analysis.KeywordAttribute)
	f.posIncAtt, _ = src.GetAttribute(reflect.TypeOf((*tokenattributes.PositionIncrementAttribute)(nil)).Elem()).(tokenattributes.PositionIncrementAttribute)
	f.posLenAtt, _ = src.GetAttribute(analysis.PositionLengthAttributeType).(analysis.PositionLengthAttribute)

	return f
}

// offsetByCodePoints renders Character.offsetByCodePoints(buf, 0, count, index,
// codePointOffset) over a UTF-8 buffer: it returns the buffer index reached by
// stepping codePointOffset code points forward from index.
func offsetByCodePoints(buf []byte, index, codePointOffset int) int {
	for ; codePointOffset > 0 && index < len(buf); codePointOffset-- {
		_, size := utf8.DecodeRune(buf[index:])
		index += size
	}
	return index
}

// setAdd renders CharArraySet.add: it returns false when the set already held
// the value.
func setAdd(set map[string]struct{}, value string) bool {
	if _, ok := set[value]; ok {
		return false
	}
	set[value] = struct{}{}
	return true
}

// IncrementToken returns the next token in the stream, or false at EOS.
func (f *SuffixingNGramTokenFilter) IncrementToken() (bool, error) {
	for {
		if f.curTermBuffer == nil {
			ok, err := f.GetInput().IncrementToken()
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}

			if f.keywordAtt != nil && f.keywordAtt.IsKeywordToken() {
				return true, nil
			}

			f.curTermLength = f.termAtt.Length()
			f.curTermBuffer = append([]byte(nil), f.termAtt.Buffer()[:f.curTermLength]...)
			f.curCodePointCount = utf8.RuneCount(f.curTermBuffer)
			f.curGramSize = f.curTermLength
			f.curPos = 0
			if f.posIncAtt != nil {
				f.curPosInc = f.posIncAtt.GetPositionIncrement()
			}
			if f.posLenAtt != nil {
				f.curPosLen = f.posLenAtt.GetPositionLength()
			}
			if f.offsetAtt != nil {
				f.tokStart = f.offsetAtt.StartOffset()
				f.tokEnd = f.offsetAtt.EndOffset()
			}
			return true, nil
		}

		if f.curTermLength > f.maxTokenLength {
			f.GetAttributeSource().ClearAttributes()
			f.termAtt.AppendString(f.anyToken)
			f.curTermBuffer = nil
			return true, nil
		}

		if f.curGramSize == 0 {
			f.curPos++
			f.curGramSize = f.curTermLength - f.curPos
		}
		if f.curGramSize >= 0 && (f.curPos+f.curGramSize) <= f.curCodePointCount {
			f.GetAttributeSource().ClearAttributes()
			start := offsetByCodePoints(f.curTermBuffer, 0, f.curPos)
			end := offsetByCodePoints(f.curTermBuffer, start, f.curGramSize)
			f.termAtt.SetEmpty()
			f.termAtt.Append(f.curTermBuffer[start:end])
			f.termAtt.AppendString(f.suffix)
			gram := f.termAtt.String()
			if f.curGramSize == f.curTermLength-f.curPos && !setAdd(f.seenSuffixes, gram) {
				f.curTermBuffer = nil
				continue
			}
			if !setAdd(f.seenInfixes, gram) {
				f.curGramSize = 0
				continue
			}
			if f.posIncAtt != nil {
				f.posIncAtt.SetPositionIncrement(f.curPosInc)
			}
			f.curPosInc = 0
			if f.posLenAtt != nil {
				f.posLenAtt.SetPositionLength(f.curPosLen)
			}
			if f.offsetAtt != nil {
				f.offsetAtt.SetOffset(f.tokStart, f.tokEnd)
			}
			f.curGramSize--
			return true, nil
		}

		f.curTermBuffer = nil
	}
}

// Reset resets per-token state and the wrapped stream.
func (f *SuffixingNGramTokenFilter) Reset() error {
	if err := f.BaseTokenFilter.Reset(); err != nil {
		return err
	}
	f.curTermBuffer = nil
	clear(f.seenInfixes)
	clear(f.seenSuffixes)
	return nil
}
