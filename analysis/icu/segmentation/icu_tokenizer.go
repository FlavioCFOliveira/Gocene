// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

import (
	"bufio"
	"io"
	"unicode"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/icu/tokenattributes"
)

const ioBuffer = 4096

// ICUTokenizer breaks text into words according to UAX #29: Unicode Text
// Segmentation (https://www.unicode.org/reports/tr29/).
//
// Words are broken across script boundaries, then segmented according to the
// BreakIterator and typing provided by the ICUTokenizerConfig.
//
// Go port of
// org.apache.lucene.analysis.icu.segmentation.ICUTokenizer
// (Apache Lucene 10.4.0).
//
// Deviation: Java uses a char[] buffer (UTF-16). Go uses []rune (Unicode code
// points). The buffer size IOBUFFER = 4096 is preserved; it counts runes, not
// bytes. CorrectOffset is not called on offsets because Gocene tokenizers emit
// raw rune offsets; offset correction is chained through CharFilter readers.
type ICUTokenizer struct {
	*analysis.BaseTokenizer

	buffer       []rune
	length       int
	usableLength int
	offset       int

	breaker *CompositeBreakIterator
	config  ICUTokenizerConfig

	// br is a bufio.Reader that wraps the current input reader to allow
	// efficient rune-at-a-time decoding across multiple refill calls.
	// It is reset whenever SetReader is called.
	br *bufio.Reader

	offsetAttr analysis.OffsetAttribute
	termAttr   analysis.CharTermAttribute
	typeAttr   analysis.TypeAttribute
	scriptAttr *tokenattributes.ScriptAttributeImpl
}

// NewICUTokenizer creates a new ICUTokenizer using
// DefaultICUTokenizerConfig(cjkAsWords=true, myanmarAsWords=true).
func NewICUTokenizer() *ICUTokenizer {
	return NewICUTokenizerWith(NewDefaultICUTokenizerConfig(true, true))
}

// NewICUTokenizerWith creates a new ICUTokenizer with a custom config.
func NewICUTokenizerWith(config ICUTokenizerConfig) *ICUTokenizer {
	t := &ICUTokenizer{
		BaseTokenizer: analysis.NewBaseTokenizer(),
		buffer:        make([]rune, ioBuffer),
		config:        config,
		breaker:       NewCompositeBreakIterator(config),
	}

	t.termAttr = analysis.NewCharTermAttribute()
	t.offsetAttr = analysis.NewOffsetAttribute()
	t.typeAttr = analysis.NewTypeAttribute()
	t.scriptAttr = tokenattributes.NewScriptAttributeImpl()

	t.AddAttribute(t.termAttr)
	t.AddAttribute(t.offsetAttr)
	t.AddAttribute(t.typeAttr)
	t.AddAttribute(t.scriptAttr)

	return t
}

// SetReader sets the input reader and resets the internal bufio.Reader.
func (t *ICUTokenizer) SetReader(input io.Reader) error {
	if err := t.BaseTokenizer.SetReader(input); err != nil {
		return err
	}
	if input == nil {
		t.br = nil
	} else {
		t.br = bufio.NewReader(input)
	}
	return nil
}

// IncrementToken advances to the next token.
// Returns (true, nil) when a token is available, (false, nil) at EOF.
func (t *ICUTokenizer) IncrementToken() (bool, error) {
	t.ClearAttributes()
	if t.length == 0 {
		if err := t.refill(); err != nil && err != io.EOF {
			return false, err
		}
	}
	for !t.incrementTokenBuffer() {
		if err := t.refill(); err != nil && err != io.EOF {
			return false, err
		}
		if t.length <= 0 {
			return false, nil
		}
	}
	return true, nil
}

// Reset prepares the tokenizer for a new tokenization session.
func (t *ICUTokenizer) Reset() error {
	if err := t.BaseTokenizer.Reset(); err != nil {
		return err
	}
	t.breaker.SetText(t.buffer, 0, 0)
	t.length = 0
	t.usableLength = 0
	t.offset = 0
	return nil
}

// End sets the final offset after the stream is exhausted.
func (t *ICUTokenizer) End() error {
	if err := t.BaseTokenizer.End(); err != nil {
		return err
	}
	finalOffset := t.offset
	if t.length >= 0 {
		finalOffset = t.offset + t.length
	}
	t.offsetAttr.SetOffset(finalOffset, finalOffset)
	return nil
}

// findSafeEnd returns the index one past the last whitespace rune in
// buffer[:length], or -1 if none is found.
func (t *ICUTokenizer) findSafeEnd() int {
	for i := t.length - 1; i >= 0; i-- {
		if unicode.IsSpace(t.buffer[i]) {
			return i + 1
		}
	}
	return -1
}

// refill reads more runes from the input reader into the buffer.
func (t *ICUTokenizer) refill() error {
	t.offset += t.usableLength
	leftover := t.length - t.usableLength
	copy(t.buffer, t.buffer[t.usableLength:t.usableLength+leftover])

	requested := len(t.buffer) - leftover
	returned, err := readRunesFull(t.br, t.buffer[leftover:leftover+requested])
	t.length = returned + leftover

	if returned < requested {
		// Reader exhausted — process everything remaining.
		t.usableLength = t.length
	} else {
		// Find a safe stopping point (whitespace boundary).
		safe := t.findSafeEnd()
		if safe < 0 {
			t.usableLength = t.length
		} else {
			t.usableLength = safe
		}
	}

	t.breaker.SetText(t.buffer, 0, max(0, t.usableLength))
	return err
}

// incrementTokenBuffer finds the next real token in the current buffer window.
// Returns true when a token has been placed into the attributes.
func (t *ICUTokenizer) incrementTokenBuffer() bool {
	start := t.breaker.Current()
	if start == Done {
		return false
	}

	// Advance past non-tokens (rule status 0 → spaces/punctuation).
	end := t.breaker.Next()
	for end != Done && t.breaker.GetRuleStatus() == RuleStatusWordNone {
		start = end
		end = t.breaker.Next()
	}

	if end == Done {
		return false
	}

	t.termAttr.SetValue(string(t.buffer[start:end]))
	t.offsetAttr.SetOffset(t.offset+start, t.offset+end)
	t.typeAttr.SetType(t.config.GetType(t.breaker.GetScriptCode(), t.breaker.GetRuleStatus()))
	t.scriptAttr.SetCode(t.breaker.GetScriptCode())

	return true
}

// readRunesFull reads up to len(dst) runes from br into dst.
// Returns the number of runes written and the first non-nil error
// (io.EOF when the reader is exhausted).
func readRunesFull(br *bufio.Reader, dst []rune) (int, error) {
	if br == nil {
		return 0, io.EOF
	}
	var readErr error
	idx := 0
	for idx < len(dst) {
		ch, _, err := br.ReadRune()
		if err != nil {
			readErr = err
			break
		}
		dst[idx] = ch
		idx++
	}
	return idx, readErr
}

// Ensure ICUTokenizer implements the Tokenizer contract.
var _ analysis.Tokenizer = (*ICUTokenizer)(nil)
