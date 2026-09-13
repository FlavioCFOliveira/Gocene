// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"unicode"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// MockTokenizer is the Go port of Lucene 10.4.0's
// org.apache.lucene.tests.analysis.MockTokenizer.
//
// It is a test-only tokenizer that can stand in for WhitespaceTokenizer,
// KeywordTokenizer, or LetterTokenizer by selecting a CharacterRunAutomaton.
// It also performs state-machine checks of the consumer workflow (reset /
// incrementToken / end / close) unless they are disabled.
//
// Lucene reference:
//
//	lucene/test-framework/src/java/org/apache/lucene/tests/analysis/MockTokenizer.java
//
// Gocene divergence: the underlying reader is read by full UTF-8 runes rather
// than UTF-16 code units, and offsets are byte offsets.
type MockTokenizer struct {
	*analysis.BaseTokenizer

	runAutomaton   *automaton.CharacterRunAutomaton
	lowerCase      bool
	maxTokenLength int

	state             int
	off               int
	bufferedCodePoint int
	bufferedOff       int
	lastOffset        int

	termAtt   analysis.CharTermAttribute
	offsetAtt analysis.OffsetAttribute

	enableChecks bool
	streamState  tokenizerState

	reader *bufio.Reader

	// random selects between different one-character read paths. It does not
	// change tokenization semantics, only how bytes are pulled from the reader.
	random *rand.Rand
}

// tokenizerState mirrors MockTokenizer.State.
type tokenizerState int

const (
	stateSetReader tokenizerState = iota
	stateReset
	stateIncrement
	stateIncrementFalse
	stateEnd
	stateClose
)

// DefaultMaxTokenLength matches Lucene's DEFAULT_MAX_TOKEN_LENGTH.
const DefaultMaxTokenLength = 255

var (
	// WHITESPACE is a CharacterRunAutomaton that accepts runs of
	// non-whitespace characters, like WhitespaceTokenizer.
	WHITESPACE = mustCharacterRunAutomaton("\\S+")

	// KEYWORD is a CharacterRunAutomaton that accepts any characters,
	// like KeywordTokenizer.
	KEYWORD = mustCharacterRunAutomaton(".*")

	// SIMPLE is a CharacterRunAutomaton that accepts letters, like
	// LetterTokenizer. This is an intentionally incomplete Unicode 5.2
	// [:Letter:] approximation matching the Java reference.
	SIMPLE = mustCharacterRunAutomaton("[A-Za-z]+")
)

func mustCharacterRunAutomaton(expr string) *automaton.CharacterRunAutomaton {
	re, err := automaton.NewRegExp(expr)
	if err != nil {
		panic(err)
	}
	raw, err := re.ToAutomaton()
	if err != nil {
		panic(err)
	}
	a, err := automaton.Determinize(raw, automaton.DefaultDeterminizeWorkLimit)
	if err != nil {
		panic(err)
	}
	return automaton.NewCharacterRunAutomaton(a)
}

// NewMockTokenizer creates a MockTokenizer using the default attribute factory.
func NewMockTokenizer(runAutomaton *automaton.CharacterRunAutomaton, lowerCase bool, maxTokenLength int) *MockTokenizer {
	return NewMockTokenizerWithFactory(util.DefaultAttributeFactoryInstance, runAutomaton, lowerCase, maxTokenLength)
}

// NewMockTokenizerWithFactory creates a MockTokenizer with a custom attribute
// factory.
func NewMockTokenizerWithFactory(factory util.AttributeFactory, runAutomaton *automaton.CharacterRunAutomaton, lowerCase bool, maxTokenLength int) *MockTokenizer {
	if runAutomaton == nil {
		panic("MockTokenizer: runAutomaton must not be nil")
	}
	if maxTokenLength <= 0 {
		maxTokenLength = DefaultMaxTokenLength
	}
	t := &MockTokenizer{
		BaseTokenizer:     analysis.NewBaseTokenizerWithFactory(factory),
		runAutomaton:      runAutomaton,
		lowerCase:         lowerCase,
		maxTokenLength:    maxTokenLength,
		enableChecks:      true,
		streamState:       stateClose,
		bufferedCodePoint: -1,
		random:            rand.New(rand.NewSource(0xDEADBEEF)),
	}
	t.AddAttribute(analysis.NewCharTermAttribute())
	t.AddAttribute(analysis.NewOffsetAttribute())
	t.termAtt = t.GetAttributeSource().GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	t.offsetAtt = t.GetAttributeSource().GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	return t
}

// SetEnableChecks toggles the consumer workflow checks.
func (t *MockTokenizer) SetEnableChecks(enable bool) {
	t.enableChecks = enable
}

func (t *MockTokenizer) fail(msg string) {
	if t.enableChecks {
		panic(fmt.Sprintf("MockTokenizer: %s (state=%d)", msg, t.streamState))
	}
}

func (t *MockTokenizer) failAlways(msg string) {
	panic(fmt.Sprintf("MockTokenizer: %s", msg))
}

// SetReader sets the input source for this tokenizer.
func (t *MockTokenizer) SetReader(input io.Reader) error {
	if t.streamState != stateClose {
		t.fail("setReader() called in wrong state")
	}
	defer func() { t.streamState = stateSetReader }()
	if input == nil {
		return errors.New("MockTokenizer.SetReader: input must not be nil")
	}
	if err := t.BaseTokenizer.SetReader(input); err != nil {
		return err
	}
	t.reader = bufio.NewReader(input)
	return nil
}

// Reset prepares the tokenizer for a new tokenization session.
func (t *MockTokenizer) Reset() error {
	if t.streamState == stateReset {
		t.fail("double reset()")
	}
	if err := t.BaseTokenizer.Reset(); err != nil {
		return err
	}
	t.state = 0
	t.off = 0
	t.lastOffset = 0
	t.bufferedCodePoint = -1
	t.bufferedOff = -1
	t.streamState = stateReset
	return nil
}

// IncrementToken advances to the next token.
func (t *MockTokenizer) IncrementToken() (bool, error) {
	if t.streamState == stateSetReader {
		if err := t.Reset(); err != nil {
			return false, err
		}
	}
	if t.streamState != stateReset && t.streamState != stateIncrement {
		t.fail("incrementToken() called while in wrong state")
	}
	t.GetAttributeSource().ClearAttributes()

	for {
		var startOffset int
		var cp int
		if t.bufferedCodePoint >= 0 {
			cp = t.bufferedCodePoint
			startOffset = t.bufferedOff
			t.bufferedCodePoint = -1
		} else {
			startOffset = t.off
			cp = t.readCodePoint()
		}
		if cp < 0 {
			break
		}
		if t.isTokenChar(cp) {
			var runeBuf [utf8.UTFMax]byte
			cp = t.normalize(cp)
			n := utf8.EncodeRune(runeBuf[:], rune(cp))
			t.termAtt.Append(runeBuf[:n])
			endOffset := t.off
			for t.termAtt.Length() < t.maxTokenLength {
				cp2 := t.readCodePoint()
				if cp2 < 0 || !t.isTokenChar(cp2) {
					if cp2 >= 0 {
						// buffer the rejected character as the start of a future token
						t.bufferedCodePoint = cp2
						t.bufferedOff = t.off
					}
					break
				}
				n2 := utf8.EncodeRune(runeBuf[:], rune(t.normalize(cp2)))
				t.termAtt.Append(runeBuf[:n2])
				endOffset = t.off
			}
			if t.termAtt.Length() >= t.maxTokenLength {
				t.bufferedCodePoint = -1
			}
			if startOffset < 0 {
				t.failAlways(fmt.Sprintf("invalid start offset: %d", startOffset))
			}
			if endOffset < 0 {
				t.failAlways(fmt.Sprintf("invalid end offset: %d", endOffset))
			}
			if startOffset < t.lastOffset {
				t.failAlways(fmt.Sprintf("start offset went backwards: %d < lastOffset=%d", startOffset, t.lastOffset))
			}
			t.lastOffset = startOffset
			if endOffset < startOffset {
				t.failAlways(fmt.Sprintf("end offset %d is before start offset %d", endOffset, startOffset))
			}
			t.offsetAtt.SetOffset(startOffset, endOffset)
			if t.state == -1 || t.runAutomaton.IsAccept(t.state) {
				t.streamState = stateIncrement
				return true, nil
			}
		}
	}
	t.streamState = stateIncrementFalse
	return false, nil
}

func (t *MockTokenizer) readCodePoint() int {
	ch, size, err := t.readChar()
	if err != nil || ch < 0 {
		return -1
	}
	t.off += size
	return ch
}

// readChar consumes a single rune from the reader, exercising a few different
// read paths the way the Java MockTokenizer does.
func (t *MockTokenizer) readChar() (int, int, error) {
	if t.reader == nil {
		return -1, 0, io.EOF
	}
	switch t.random.Intn(10) {
	case 0:
		var c [1]byte
		n, err := t.reader.Read(c[:])
		if err != nil || n < 1 {
			return -1, 0, err
		}
		if c[0] < 0x80 {
			return int(c[0]), 1, nil
		}
		cont := readContinuation(t.reader, sizeForLead(c[0])-1)
		r, size := utf8.DecodeRune(append(c[:1], cont...))
		return int(r), size, nil
	case 1:
		var c [2]byte
		n, err := t.reader.Read(c[1:2])
		if err != nil || n < 1 {
			return -1, 0, err
		}
		if c[1] < 0x80 {
			return int(c[1]), 1, nil
		}
		cont := readContinuation(t.reader, sizeForLead(c[1])-1)
		r, size := utf8.DecodeRune(append(c[1:2], cont...))
		return int(r), size, nil
	default:
		r, size, err := t.reader.ReadRune()
		if err != nil {
			return -1, 0, err
		}
		return int(r), size, nil
	}
}

func sizeForLead(b byte) int {
	size := 1
	for i := 7; i >= 0; i-- {
		if b&(1<<i) != 0 {
			size++
		} else {
			break
		}
	}
	if size > utf8.UTFMax {
		return 1
	}
	return size
}

func readContinuation(r *bufio.Reader, n int) []byte {
	if n <= 0 {
		return nil
	}
	out := make([]byte, n)
	_, _ = io.ReadFull(r, out)
	return out
}

func (t *MockTokenizer) isTokenChar(c int) bool {
	if t.state < 0 {
		t.state = 0
	}
	c = t.normalize(c)
	t.state = t.runAutomaton.Step(t.state, c)
	return t.state >= 0
}

func (t *MockTokenizer) normalize(c int) int {
	if t.lowerCase {
		return int(unicode.ToLower(rune(c)))
	}
	return c
}

// End performs end-of-stream operations.
func (t *MockTokenizer) End() error {
	if t.streamState != stateIncrementFalse {
		t.fail("end() called in wrong state")
	}
	if err := t.BaseTokenizer.End(); err != nil {
		return err
	}
	finalOffset := t.off
	t.offsetAtt.SetOffset(finalOffset, finalOffset)
	t.streamState = stateEnd
	return nil
}

// Close releases resources and enforces the final workflow state.
func (t *MockTokenizer) Close() error {
	if !(t.streamState == stateEnd || t.streamState == stateClose) {
		t.fail("close() called in wrong state")
	}
	t.streamState = stateClose
	t.reader = nil
	return t.BaseTokenizer.Close()
}

// Ensure MockTokenizer implements analysis.Tokenizer.
var _ analysis.Tokenizer = (*MockTokenizer)(nil)
