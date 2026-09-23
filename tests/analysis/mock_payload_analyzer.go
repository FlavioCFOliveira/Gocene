// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
)

// MockPayloadAnalyzer wraps a whitespace tokenizer with a filter that sets the
// first token, and odd tokens to posinc=1, and all others to 0, encoding the
// position as "pos: XXX" in the payload.
//
// This is the Go port of Lucene's
// org.apache.lucene.tests.analysis.MockPayloadAnalyzer
// (lucene/test-framework/src/java/org/apache/lucene/tests/analysis/MockPayloadAnalyzer.java,
// Apache Lucene 10.5.0). The Java class extends Analyzer and overrides
// createComponents; Gocene renders the override through
// BaseAnalyzer.CreateComponents.
type MockPayloadAnalyzer struct {
	*analysis.BaseAnalyzer
}

// NewMockPayloadAnalyzer renders the implicit `public MockPayloadAnalyzer()`
// constructor. Analyzer() uses the global reuse strategy.
func NewMockPayloadAnalyzer() *MockPayloadAnalyzer {
	a := &MockPayloadAnalyzer{BaseAnalyzer: analysis.NewAnalyzer(analysis.GlobalReuseStrategy)}
	a.CreateComponents = a.createComponents
	return a
}

// createComponents renders
// `public TokenStreamComponents createComponents(String fieldName)`.
func (a *MockPayloadAnalyzer) createComponents(fieldName string) *analysis.TokenStreamComponents {
	result := NewMockTokenizer(WHITESPACE, true, DefaultMaxTokenLength)
	return &analysis.TokenStreamComponents{
		Source: func(r io.Reader) error {
			result.SetReader(r)
			return nil
		},
		Sink: newMockPayloadFilter(result, fieldName),
	}
}

// mockPayloadFilter renders the package-private final class MockPayloadFilter
// declared in MockPayloadAnalyzer.java.
type mockPayloadFilter struct {
	*analysis.BaseTokenFilter

	fieldName string

	pos int

	i int

	posIncrAttr tokenattributes.PositionIncrementAttribute
	payloadAttr analysis.PayloadAttribute
	termAttr    analysis.CharTermAttribute
}

// newMockPayloadFilter renders
// `public MockPayloadFilter(TokenStream input, String fieldName)`. The
// attributes are added to input, as in Java.
func newMockPayloadFilter(input analysis.TokenStream, fieldName string) *mockPayloadFilter {
	f := &mockPayloadFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		fieldName:       fieldName,
		pos:             0,
		i:               0,
	}
	source := input.GetAttributeSource()
	f.posIncrAttr = source.AddAttribute(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
	f.payloadAttr = source.AddAttribute(analysis.PayloadAttributeType).(analysis.PayloadAttribute)
	f.termAttr = source.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	return f
}

// IncrementToken renders `public boolean incrementToken()`.
func (f *mockPayloadFilter) IncrementToken() (bool, error) {
	more, err := f.GetInput().IncrementToken()
	if err != nil {
		return false, err
	}
	if !more {
		return false, nil
	}
	f.payloadAttr.SetPayload([]byte("pos: " + strconv.Itoa(f.pos)))
	var posIncr int
	if f.pos == 0 || f.i%2 == 1 {
		posIncr = 1
	} else {
		posIncr = 0
	}
	f.posIncrAttr.SetPositionIncrement(posIncr)
	f.pos += posIncr
	f.i++
	return true, nil
}

// Reset renders `public void reset()`.
func (f *mockPayloadFilter) Reset() error {
	if err := f.BaseTokenFilter.Reset(); err != nil {
		return err
	}
	f.i = 0
	f.pos = 0
	return nil
}
