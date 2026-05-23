// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package cjk

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Script flag constants for CJKBigramFilter.
const (
	// Han enables bigrams for Han ideographs.
	Han = 1
	// Hiragana enables bigrams for Hiragana characters.
	Hiragana = 2
	// Katakana enables bigrams for Katakana characters.
	Katakana = 4
	// Hangul enables bigrams for Hangul characters.
	Hangul = 8
)

// CJK token type strings as emitted by StandardTokenizer.
const (
	// DoubleType is the type tag emitted for bigram tokens.
	DoubleType = "<DOUBLE>"
	// SingleType is the type tag emitted for unigram CJK tokens.
	SingleType = "<SINGLE>"
)

// hanType etc. are the string values assigned to CJK tokens by StandardTokenizer.
var (
	hanType      = analysis.StandardTokenTypes[analysis.TokenTypeIdeographic]
	hiraganaType = analysis.StandardTokenTypes[analysis.TokenTypeHiragana]
	katakanaType = analysis.StandardTokenTypes[analysis.TokenTypeKatakana]
	hangulType   = analysis.StandardTokenTypes[analysis.TokenTypeHangul]
)

// CJKBigramFilter forms bigrams of CJK terms produced by StandardTokenizer.
//
// CJK types are set by StandardTokenizer. You can also restrict which scripts
// are bigrammed via the flags parameter.
//
// By default, an isolated CJK character is output as a unigram. If
// outputUnigrams is true, unigrams are always output alongside bigrams.
//
// This is the Go port of
// org.apache.lucene.analysis.cjk.CJKBigramFilter from
// Apache Lucene 10.4.0.
type CJKBigramFilter struct {
	*analysis.BaseTokenFilter

	// which types to bigram (nil means "pass through")
	doHan      *string
	doHiragana *string
	doKatakana *string
	doHangul   *string

	outputUnigrams bool
	ngramState     bool // false=unigram, true=bigram (when outputUnigrams=true)

	termAttr   analysis.CharTermAttribute
	typeAttr   analysis.TypeAttribute
	offsetAttr analysis.OffsetAttribute
	posIncAttr analysis.PositionIncrementAttribute
	posLenAttr analysis.PositionLengthAttribute

	// codepoint + offset buffers (parallel arrays)
	buffer      []rune
	startOffset []int
	endOffset   []int
	bufferLen   int
	index       int

	lastEndOffset int
	exhausted     bool
	loneState     *util.AttributeState
}

// NewCJKBigramFilter creates a CJKBigramFilter with all scripts enabled and
// unigrams only on isolation.
func NewCJKBigramFilter(input analysis.TokenStream) *CJKBigramFilter {
	return NewCJKBigramFilterWithFlags(input, Han|Hiragana|Katakana|Hangul)
}

// NewCJKBigramFilterWithFlags creates a CJKBigramFilter with specific script flags.
func NewCJKBigramFilterWithFlags(input analysis.TokenStream, flags int) *CJKBigramFilter {
	return NewCJKBigramFilterFull(input, flags, false)
}

// NewCJKBigramFilterFull creates a CJKBigramFilter with full configuration.
func NewCJKBigramFilterFull(input analysis.TokenStream, flags int, outputUnigrams bool) *CJKBigramFilter {
	f := &CJKBigramFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		outputUnigrams:  outputUnigrams,
		buffer:          make([]rune, 8),
		startOffset:     make([]int, 8),
		endOffset:       make([]int, 8),
	}

	if flags&Han != 0 {
		f.doHan = &hanType
	}
	if flags&Hiragana != 0 {
		f.doHiragana = &hiraganaType
	}
	if flags&Katakana != 0 {
		f.doKatakana = &katakanaType
	}
	if flags&Hangul != 0 {
		f.doHangul = &hangulType
	}

	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(analysis.TypeAttributeType); a != nil {
			f.typeAttr = a.(analysis.TypeAttribute)
		}
		if a := src.GetAttribute(analysis.OffsetAttributeType); a != nil {
			f.offsetAttr = a.(analysis.OffsetAttribute)
		}
		if a := src.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
			f.posIncAttr = a.(analysis.PositionIncrementAttribute)
		}
		if a := src.GetAttribute(analysis.PositionLengthAttributeType); a != nil {
			f.posLenAttr = a.(analysis.PositionLengthAttribute)
		}
	}

	return f
}

// IncrementToken advances to the next token.
func (f *CJKBigramFilter) IncrementToken() (bool, error) {
	for {
		if f.hasBufferedBigram() {
			// Case 1: multiple codepoints buffered → emit bigram (or unigram+rewind).
			if f.outputUnigrams {
				if f.ngramState {
					f.flushBigram()
				} else {
					f.flushUnigram()
					f.index--
				}
				f.ngramState = !f.ngramState
			} else {
				f.flushBigram()
			}
			return true, nil
		}

		// Case 2: advance input.
		ok, err := f.doNext()
		if err != nil {
			return false, err
		}
		if ok {
			if f.typeAttr == nil {
				return true, nil
			}
			typ := f.typeAttr.GetType()
			if f.isCJKType(typ) {
				// CJK token: buffer if offsets aligned, else clear and restart.
				if f.offsetAttr != nil && f.offsetAttr.StartOffset() != f.lastEndOffset {
					if f.hasBufferedUnigram() {
						f.loneState = f.GetAttributeSource().CaptureState()
						f.flushUnigram()
						return true, nil
					}
					f.index = 0
					f.bufferLen = 0
				}
				f.refill()
			} else {
				// Non-CJK: flush any buffered unigram first.
				if f.hasBufferedUnigram() {
					f.loneState = f.GetAttributeSource().CaptureState()
					f.flushUnigram()
					return true, nil
				}
				return true, nil
			}
		} else {
			// Case 3: input exhausted.
			if f.hasBufferedUnigram() {
				f.flushUnigram()
				return true, nil
			}
			return false, nil
		}
	}
}

// isCJKType reports whether the type string corresponds to an enabled CJK script.
func (f *CJKBigramFilter) isCJKType(typ string) bool {
	return (f.doHan != nil && typ == *f.doHan) ||
		(f.doHiragana != nil && typ == *f.doHiragana) ||
		(f.doKatakana != nil && typ == *f.doKatakana) ||
		(f.doHangul != nil && typ == *f.doHangul)
}

// doNext returns the next token, restoring loneState if pending.
func (f *CJKBigramFilter) doNext() (bool, error) {
	if f.loneState != nil {
		f.GetAttributeSource().RestoreState(f.loneState)
		f.loneState = nil
		return true, nil
	}
	if f.exhausted {
		return false, nil
	}
	ok, err := f.GetInput().IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		f.exhausted = true
	}
	return ok, nil
}

// refill adds the current token's codepoints into the buffer.
func (f *CJKBigramFilter) refill() {
	// Compact the buffer to keep it small; retain only the last codepoint.
	if f.bufferLen > 64 {
		last := f.bufferLen - 1
		f.buffer[0] = f.buffer[last]
		f.startOffset[0] = f.startOffset[last]
		f.endOffset[0] = f.endOffset[last]
		f.bufferLen = 1
		f.index -= last
	}

	termRunes := []rune(f.termAttr.String())
	start := 0
	end := 0
	if f.offsetAttr != nil {
		start = f.offsetAttr.StartOffset()
		end = f.offsetAttr.EndOffset()
		f.lastEndOffset = end
	}

	newSize := f.bufferLen + len(termRunes)
	f.buffer = growRunes(f.buffer, newSize)
	f.startOffset = growInts(f.startOffset, newSize)
	f.endOffset = growInts(f.endOffset, newSize)

	if end-start != len(termRunes) {
		// crazy offsets: preserve start/end for all codepoints
		for _, cp := range termRunes {
			f.buffer[f.bufferLen] = cp
			f.startOffset[f.bufferLen] = start
			f.endOffset[f.bufferLen] = end
			f.bufferLen++
		}
	} else {
		// normal offsets: compute per-rune
		pos := start
		for _, cp := range termRunes {
			cpLen := len(string(cp))
			f.buffer[f.bufferLen] = cp
			f.startOffset[f.bufferLen] = pos
			pos += cpLen
			f.endOffset[f.bufferLen] = pos
			f.bufferLen++
		}
	}
}

// flushBigram emits a bigram from buffer[index] and buffer[index+1].
func (f *CJKBigramFilter) flushBigram() {
	f.ClearAttributes()
	bigram := string(f.buffer[f.index : f.index+2])
	if f.termAttr != nil {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(bigram)
	}
	if f.offsetAttr != nil {
		f.offsetAttr.SetOffset(f.startOffset[f.index], f.endOffset[f.index+1])
	}
	if f.typeAttr != nil {
		f.typeAttr.SetType(DoubleType)
	}
	if f.outputUnigrams {
		if f.posIncAttr != nil {
			f.posIncAttr.SetPositionIncrement(0)
		}
		if f.posLenAttr != nil {
			f.posLenAttr.SetPositionLength(2)
		}
	}
	f.index++
}

// flushUnigram emits a unigram from buffer[index].
func (f *CJKBigramFilter) flushUnigram() {
	f.ClearAttributes()
	uni := string(f.buffer[f.index])
	if f.termAttr != nil {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(uni)
	}
	if f.offsetAttr != nil {
		f.offsetAttr.SetOffset(f.startOffset[f.index], f.endOffset[f.index])
	}
	if f.typeAttr != nil {
		f.typeAttr.SetType(SingleType)
	}
	f.index++
}

func (f *CJKBigramFilter) hasBufferedBigram() bool {
	return f.bufferLen-f.index > 1
}

func (f *CJKBigramFilter) hasBufferedUnigram() bool {
	if f.outputUnigrams {
		return f.bufferLen-f.index == 1
	}
	return f.bufferLen == 1 && f.index == 0
}

// Reset resets the filter state.
func (f *CJKBigramFilter) Reset() error {
	if r, ok := f.GetInput().(interface{ Reset() error }); ok {
		if err := r.Reset(); err != nil {
			return err
		}
	}
	f.bufferLen = 0
	f.index = 0
	f.lastEndOffset = 0
	f.loneState = nil
	f.exhausted = false
	f.ngramState = false
	return nil
}

// Ensure CJKBigramFilter implements TokenFilter.
var _ analysis.TokenFilter = (*CJKBigramFilter)(nil)

// CJKBigramFilterFactory creates CJKBigramFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.cjk.CJKBigramFilterFactory from
// Apache Lucene 10.4.0.
type CJKBigramFilterFactory struct {
	flags          int
	outputUnigrams bool
}

// NewCJKBigramFilterFactory creates a factory with all scripts enabled,
// no unigram output.
func NewCJKBigramFilterFactory() *CJKBigramFilterFactory {
	return &CJKBigramFilterFactory{
		flags:          Han | Hiragana | Katakana | Hangul,
		outputUnigrams: false,
	}
}

// NewCJKBigramFilterFactoryFull creates a factory with explicit configuration.
func NewCJKBigramFilterFactoryFull(han, hiragana, katakana, hangul, outputUnigrams bool) *CJKBigramFilterFactory {
	flags := 0
	if han {
		flags |= Han
	}
	if hiragana {
		flags |= Hiragana
	}
	if katakana {
		flags |= Katakana
	}
	if hangul {
		flags |= Hangul
	}
	return &CJKBigramFilterFactory{flags: flags, outputUnigrams: outputUnigrams}
}

// Create creates a CJKBigramFilter wrapping input.
func (f *CJKBigramFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewCJKBigramFilterFull(input, f.flags, f.outputUnigrams)
}

// Ensure factory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*CJKBigramFilterFactory)(nil)

// growRunes grows a []rune slice to at least n elements.
func growRunes(s []rune, n int) []rune {
	if cap(s) >= n {
		return s[:cap(s)]
	}
	newCap := cap(s) * 2
	if newCap < n {
		newCap = n
	}
	out := make([]rune, newCap)
	copy(out, s)
	return out
}

// growInts grows a []int slice to at least n elements.
func growInts(s []int, n int) []int {
	if cap(s) >= n {
		return s[:cap(s)]
	}
	newCap := cap(s) * 2
	if newCap < n {
		newCap = n
	}
	out := make([]int, newCap)
	copy(out, s)
	return out
}
