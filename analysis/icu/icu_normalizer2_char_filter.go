// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package icu

import (
	"io"
	"strings"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/analysis/charfilter"
)

// ICUNormalizer2CharFilter is a CharFilter that normalises character text
// using a Normalizer2 before passing it to the tokenizer.
//
// Go port of org.apache.lucene.analysis.icu.ICUNormalizer2CharFilter
// (Apache Lucene 10.4.0).
//
// Deviation: The Java implementation does incremental span-based processing
// using ICU4J's Normalizer2.spanQuickCheckYes and hasBoundaryBefore to
// identify segments that can be flushed to the output without full
// normalisation. This Go port performs equivalent boundary detection using
// the Normalizer2.SpanQuickCheckYes and HasBoundaryBefore methods. When
// the normalizer is created via NewNFKCCaseFoldNormalizer, SpanQuickCheckYes
// returns 0 (conservative) and the filter always performs full normalisation
// on each segment, which is correct but not as fast as the ICU4J variant.
type ICUNormalizer2CharFilter struct {
	*charfilter.BaseCharFilter
	normalizer   Normalizer2
	inputBuf     strings.Builder
	resultBuf    strings.Builder
	inputFinished bool
	// afterQuickCheckYes tracks whether the last read used the quick-check
	// path (so the next read pivots to boundary detection).
	afterQuickCheckYes bool
	// checkedInputBoundary is the position up to which we've already found
	// or confirmed a normalisation boundary.
	checkedInputBoundary int
	// charCount tracks how many output characters have been emitted so far,
	// used to build the offset-correction table.
	charCount int

	// tmpBuf is a scratch buffer for reads from the underlying reader.
	tmpBuf []byte
}

const icuNorm2CharFilterDefaultBufSize = 128

// NewICUNormalizer2CharFilter creates a filter that applies NFKC+CaseFold
// normalisation (the Lucene default: "nfkc_cf").
func NewICUNormalizer2CharFilter(input io.Reader) *ICUNormalizer2CharFilter {
	return NewICUNormalizer2CharFilterWith(input, NewNFKCCaseFoldNormalizer())
}

// NewICUNormalizer2CharFilterWith creates a filter that applies the supplied
// normalizer.
func NewICUNormalizer2CharFilterWith(input io.Reader, normalizer Normalizer2) *ICUNormalizer2CharFilter {
	return &ICUNormalizer2CharFilter{
		BaseCharFilter: charfilter.NewBaseCharFilter(input),
		normalizer:     normalizer,
		tmpBuf:         make([]byte, icuNorm2CharFilterDefaultBufSize),
	}
}

// Read implements io.Reader. It normalises the underlying character stream
// and corrects offsets so that Tokenizer.correctOffset works correctly.
func (f *ICUNormalizer2CharFilter) Read(cbuf []byte) (int, error) {
	if len(cbuf) == 0 {
		return 0, nil
	}

	for !f.inputFinished || f.inputBuf.Len() > 0 || f.resultBuf.Len() > 0 {
		// 1. Drain the result buffer first.
		if f.resultBuf.Len() > 0 {
			n := f.outputFromResultBuffer(cbuf)
			if n > 0 {
				return n, nil
			}
		}

		// 2. Try to normalise more input.
		n := f.readAndNormaliseFromInput()
		if n > 0 {
			ret := f.outputFromResultBuffer(cbuf)
			if ret > 0 {
				return ret, nil
			}
		}

		// 3. Refill the input buffer from the underlying reader.
		if err := f.readInputToBuffer(); err != nil && err != io.EOF {
			return 0, err
		}
	}

	return 0, io.EOF
}

// readInputToBuffer fills inputBuf from the underlying reader.
func (f *ICUNormalizer2CharFilter) readInputToBuffer() error {
	n, err := f.GetInput().Read(f.tmpBuf)
	if n > 0 {
		f.inputBuf.Write(f.tmpBuf[:n])
		if f.checkedInputBoundary > 0 {
			f.checkedInputBoundary = max(f.checkedInputBoundary-1, 0)
		}
	}
	if err == io.EOF {
		f.inputFinished = true
		return nil
	}
	return err
}

// readAndNormaliseFromInput attempts to produce normalised output from the
// input buffer.
func (f *ICUNormalizer2CharFilter) readAndNormaliseFromInput() int {
	if f.inputBuf.Len() == 0 {
		f.afterQuickCheckYes = false
		return 0
	}
	if !f.afterQuickCheckYes {
		n := f.readFromInputWhileSpanQuickCheckYes()
		if n > 0 {
			return n
		}
	}
	n := f.readFromIONormaliseUptoBoundary()
	if n > 0 {
		f.afterQuickCheckYes = false
	}
	return n
}

// readFromInputWhileSpanQuickCheckYes copies the quick-check span of the
// input directly to the result buffer without normalisation.
func (f *ICUNormalizer2CharFilter) readFromInputWhileSpanQuickCheckYes() int {
	f.afterQuickCheckYes = true
	s := f.inputBuf.String()
	end := f.normalizer.SpanQuickCheckYes(s)
	if end <= 0 {
		return 0
	}

	// If end is at the buffer boundary, check hasBoundaryAfter to avoid
	// splitting a normalisation segment across a buffer boundary.
	if end == len(s) && !f.normalizer.HasBoundaryAfter(s, len(s)) {
		f.afterQuickCheckYes = false
		// Back up to the last rune boundary before any combining sequence.
		for end > 0 {
			// Step back one rune.
			_, size := utf8.DecodeLastRuneInString(s[:end])
			end -= size
			if end == 0 {
				return 0
			}
			if f.normalizer.HasBoundaryBefore(s, end) {
				break
			}
		}
		if end == 0 {
			return 0
		}
	}

	// Copy the quick-check span directly to the result buffer.
	span := s[:end]
	f.resultBuf.WriteString(span)
	remaining := s[end:]
	f.inputBuf.Reset()
	f.inputBuf.WriteString(remaining)
	f.checkedInputBoundary = max(f.checkedInputBoundary-end, 0)
	f.charCount += utf8.RuneCountInString(span)
	return end
}

// readFromIONormaliseUptoBoundary finds the next normalisation boundary in
// the input and normalises up to that point.
func (f *ICUNormalizer2CharFilter) readFromIONormaliseUptoBoundary() int {
	s := f.inputBuf.String()
	if len(s) == 0 {
		return 0
	}

	foundBoundary := false
	bufLen := len(s)

	// Advance through the buffer looking for a boundary after
	// checkedInputBoundary.
	pos := f.checkedInputBoundary
	for pos < bufLen {
		// Advance one rune.
		_, size := utf8.DecodeRuneInString(s[pos:])
		pos += size
		f.checkedInputBoundary = pos
		if pos < bufLen && f.normalizer.HasBoundaryBefore(s, pos) {
			foundBoundary = true
			break
		}
	}

	if !foundBoundary && f.checkedInputBoundary >= bufLen && f.inputFinished {
		foundBoundary = true
		f.checkedInputBoundary = bufLen
	}

	if !foundBoundary {
		return 0
	}

	return f.normaliseInputUpto(f.checkedInputBoundary)
}

// normaliseInputUpto normalises the first length bytes of the input buffer
// and appends the result to resultBuf.
func (f *ICUNormalizer2CharFilter) normaliseInputUpto(length int) int {
	s := f.inputBuf.String()
	segment := s[:length]
	destOrigLen := utf8.RuneCountInString(f.resultBuf.String())
	normalised := f.normalizer.Normalize(segment)
	f.resultBuf.WriteString(normalised)
	remaining := s[length:]
	f.inputBuf.Reset()
	f.inputBuf.WriteString(remaining)
	f.checkedInputBoundary = max(f.checkedInputBoundary-length, 0)
	resultLen := utf8.RuneCountInString(f.resultBuf.String()) - destOrigLen
	f.recordOffsetDiff(utf8.RuneCountInString(segment), resultLen)
	return resultLen
}

// recordOffsetDiff records an offset correction when input and output lengths
// differ.
func (f *ICUNormalizer2CharFilter) recordOffsetDiff(inputLen, outputLen int) {
	if inputLen == outputLen {
		f.charCount += outputLen
		return
	}
	diff := inputLen - outputLen
	cumuDiff := f.GetLastCumulativeDiff()
	if diff < 0 {
		for i := 1; i <= -diff; i++ {
			f.AddOffCorrectMap(f.charCount+i, cumuDiff-i)
		}
	} else {
		f.AddOffCorrectMap(f.charCount+outputLen, cumuDiff+diff)
	}
	f.charCount += outputLen
}

// outputFromResultBuffer drains as many bytes as possible from resultBuf
// into cbuf and returns the number of bytes written.
func (f *ICUNormalizer2CharFilter) outputFromResultBuffer(cbuf []byte) int {
	resultStr := f.resultBuf.String()
	if len(resultStr) == 0 {
		return 0
	}
	n := copy(cbuf, resultStr)
	remaining := resultStr[n:]
	f.resultBuf.Reset()
	f.resultBuf.WriteString(remaining)
	return n
}

// CorrectOffset corrects a character offset from the normalised stream back
// to the original input stream.
func (f *ICUNormalizer2CharFilter) CorrectOffset(currentOff int) int {
	return f.BaseCharFilter.CorrectOffset(currentOff)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
