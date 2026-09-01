// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil

import (
	"hash/fnv"
	"io"
	"math/rand"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// MockAnalyzer is the Go port of Lucene 10.4.0's
// org.apache.lucene.tests.analysis.MockAnalyzer.
//
// It builds a simple, deterministic analysis chain based on a chosen
// CharacterRunAutomaton (WHITESPACE, KEYWORD, or SIMPLE), an optional
// lower-case filter, an optional stop set, and an optional random-payload
// filter. All components perform consumer workflow checks unless those are
// disabled.
//
// Lucene reference:
//
//	lucene/test-framework/src/java/org/apache/lucene/tests/analysis/MockAnalyzer.java
type MockAnalyzer struct {
	runAutomaton   *automaton.CharacterRunAutomaton
	lowerCase      bool
	maxTokenLength int
	stopSet        map[string]struct{}
	enableChecks   bool

	usePayloads      bool
	payloadMaxLength int
	payloadRandom    *rand.Rand

	positionIncrementGap int
	offsetGap            int
	reuseTokenStream     bool

	// reuse holds a lazily-created TokenStreamComponents pair when
	// SetReuseTokenStream(true) is used.
	reuse *analysis.TokenStreamComponents
}

// NewMockAnalyzer creates a MockAnalyzer with the given run automaton.
func NewMockAnalyzer(runAutomaton *automaton.CharacterRunAutomaton, lowerCase bool, maxTokenLength int, stopSet map[string]struct{}, enableChecks bool) *MockAnalyzer {
	if runAutomaton == nil {
		runAutomaton = WHITESPACE
	}
	if maxTokenLength <= 0 {
		maxTokenLength = DefaultMaxTokenLength
	}
	if stopSet == nil {
		stopSet = EMPTY_STOPSET
	}
	return &MockAnalyzer{
		runAutomaton:         runAutomaton,
		lowerCase:            lowerCase,
		maxTokenLength:       maxTokenLength,
		stopSet:              stopSet,
		enableChecks:         enableChecks,
		positionIncrementGap: 0,
		offsetGap:            1,
	}
}

// NewMockAnalyzerRandom is a convenience constructor that builds a
// deterministic but varied MockAnalyzer from a single random seed. It is
// used by tests that need a "standard" analyzer shape.
func NewMockAnalyzerRandom(r *rand.Rand, lowerCase bool, maxTokenLength int, stopSet map[string]struct{}, enableChecks bool) *MockAnalyzer {
	if r == nil {
		r = rand.New(rand.NewSource(0))
	}
	// Choose an automaton based on the random source.
	var run *automaton.CharacterRunAutomaton
	switch r.Intn(3) {
	case 0:
		run = WHITESPACE
	case 1:
		run = KEYWORD
	default:
		run = SIMPLE
	}
	a := NewMockAnalyzer(run, lowerCase, maxTokenLength, stopSet, enableChecks)
	a.payloadRandom = r
	return a
}

// SetMaxTokenLength sets the tokenizer limit.
func (a *MockAnalyzer) SetMaxTokenLength(maxTokenLength int) {
	a.maxTokenLength = maxTokenLength
}

// GetMaxTokenLength returns the tokenizer limit.
func (a *MockAnalyzer) GetMaxTokenLength() int {
	return a.maxTokenLength
}

// SetEnableChecks toggles workflow checks on the tokenizer and filters.
func (a *MockAnalyzer) SetEnableChecks(enable bool) {
	a.enableChecks = enable
}

// IsEnableChecks reports whether workflow checks are enabled.
func (a *MockAnalyzer) IsEnableChecks() bool {
	return a.enableChecks
}

// SetUsePayloads enables random payload generation for token streams.
func (a *MockAnalyzer) SetUsePayloads(use bool) {
	a.usePayloads = use
}

// SetPayloadMaxLength sets the upper bound for random payload sizes.
func (a *MockAnalyzer) SetPayloadMaxLength(maxLength int) {
	a.payloadMaxLength = maxLength
}

// SetPayloadRandom provides the random source used for payload generation.
func (a *MockAnalyzer) SetPayloadRandom(r *rand.Rand) {
	a.payloadRandom = r
}

// SetPositionIncrementGap sets the gap inserted between field values.
func (a *MockAnalyzer) SetPositionIncrementGap(gap int) {
	a.positionIncrementGap = gap
}

// GetPositionIncrementGap returns the configured position increment gap.
func (a *MockAnalyzer) GetPositionIncrementGap() int {
	return a.positionIncrementGap
}

// SetOffsetGap sets the offset gap inserted between field values.
func (a *MockAnalyzer) SetOffsetGap(gap int) {
	a.offsetGap = gap
}

// GetOffsetGap returns the configured offset gap.
func (a *MockAnalyzer) GetOffsetGap() int {
	return a.offsetGap
}

// TokenStream builds a fresh analysis chain for the requested field.
func (a *MockAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	components := a.CreateComponents(fieldName)
	src := components.GetSource()
	if err := src.SetReader(reader); err != nil {
		return nil, err
	}
	if err := src.Reset(); err != nil {
		return nil, err
	}
	return components.GetSink(), nil
}

// CreateComponents builds the (Tokenizer, TokenStream) pair for a field.
func (a *MockAnalyzer) CreateComponents(fieldName string) *analysis.TokenStreamComponents {
	if a.reuse != nil && a.reuseTokenStream {
		return a.reuse
	}

	tokenizer := NewMockTokenizerWithFactory(util.DefaultAttributeFactoryInstance, a.runAutomaton, a.lowerCase, a.maxTokenLength)
	tokenizer.SetEnableChecks(a.enableChecks)

	var stream analysis.TokenStream = tokenizer
	if a.lowerCase {
		stream = analysis.NewLowerCaseFilter(stream)
	}
	if len(a.stopSet) > 0 {
		filter := NewMockTokenFilter(stream, a.stopSet)
		filter.SetEnableChecks(a.enableChecks)
		stream = filter
	}

	if a.usePayloads {
		r := a.fieldRandom(fieldName)
		if r.Intn(2) == 0 {
			stream = NewMockFixedLengthPayloadFilter(stream, r.Intn(a.payloadMaxLength+1), r)
		} else {
			stream = NewMockVariableLengthPayloadFilter(stream, a.payloadMaxLength, r)
		}
	}

	components := analysis.NewTokenStreamComponents(tokenizer, stream)
	if a.reuseTokenStream {
		a.reuse = components
	}
	return components
}

// fieldRandom returns a deterministic random source for a field name.
func (a *MockAnalyzer) fieldRandom(fieldName string) *rand.Rand {
	seed := int64(0xdeadbeef)
	h := fnv.New64a()
	h.Write([]byte(fieldName))
	seed ^= int64(h.Sum64())
	if a.payloadRandom != nil {
		seed ^= a.payloadRandom.Int63()
	}
	return rand.New(rand.NewSource(seed))
}

// SetReuseTokenStream enables component reuse across TokenStream calls.
func (a *MockAnalyzer) SetReuseTokenStream(reuse bool) {
	a.reuseTokenStream = reuse
	if !reuse {
		a.reuse = nil
	}
}

// Close releases resources held by this analyzer.
func (a *MockAnalyzer) Close() error {
	if a.reuse != nil {
		if err := a.reuse.GetSink().Close(); err != nil {
			return err
		}
		a.reuse = nil
	}
	return nil
}

// Ensure MockAnalyzer implements analysis.Analyzer.
var _ analysis.Analyzer = (*MockAnalyzer)(nil)
