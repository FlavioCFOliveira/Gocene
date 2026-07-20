// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// MockTokenFilter is the Go port of Lucene 10.4.0's
// org.apache.lucene.tests.analysis.MockTokenFilter.
//
// It filters out tokens whose term text is contained in a stop set, and
// performs the same consumer workflow checks as MockTokenizer unless they
// are disabled.
//
// Lucene reference:
//
//	lucene/test-framework/src/java/org/apache/lucene/tests/analysis/MockTokenFilter.java
type MockTokenFilter struct {
	*analysis.BaseTokenFilter

	stopSet      map[string]struct{}
	enableChecks bool
	streamState  tokenFilterState
}

type tokenFilterState int

const (
	tfStateReset tokenFilterState = iota
	tfStateIncrement
	tfStateIncrementFalse
	tfStateEnd
	tfStateClose
)

// EMPTY_STOPSET is an empty stop set.
var EMPTY_STOPSET = map[string]struct{}{}

// ENGLISH_STOPSET is a simple English stop set.
var ENGLISH_STOPSET = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {},
	"but": {}, "by": {}, "for": {}, "if": {}, "in": {}, "into": {}, "is": {},
	"it": {}, "no": {}, "not": {}, "of": {}, "on": {}, "or": {}, "such": {},
	"that": {}, "the": {}, "their": {}, "then": {}, "there": {}, "these": {},
	"they": {}, "this": {}, "to": {}, "was": {}, "will": {}, "with": {},
}

// NewMockTokenFilter creates a filter wrapping the given input and stop set.
func NewMockTokenFilter(input analysis.TokenStream, stopSet map[string]struct{}) *MockTokenFilter {
	if input == nil {
		panic("MockTokenFilter: input must not be nil")
	}
	if stopSet == nil {
		stopSet = EMPTY_STOPSET
	}
	return &MockTokenFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		stopSet:         stopSet,
		enableChecks:    true,
		streamState:     tfStateClose,
	}
}

// SetEnableChecks toggles the consumer workflow checks.
func (f *MockTokenFilter) SetEnableChecks(enable bool) {
	f.enableChecks = enable
}

func (f *MockTokenFilter) fail(msg string) {
	if f.enableChecks {
		panic(fmt.Sprintf("MockTokenFilter: %s (state=%d)", msg, f.streamState))
	}
}

// IncrementToken advances to the next non-stop token.
func (f *MockTokenFilter) IncrementToken() (bool, error) {
	if f.streamState == tfStateClose {
		f.streamState = tfStateReset
	}
	if f.streamState != tfStateReset && f.streamState != tfStateIncrement {
		f.fail("incrementToken() called in wrong state")
	}
	posIncAttr := f.GetAttributeSource().GetAttribute(analysis.PositionIncrementAttributeType)
	posInc, _ := posIncAttr.(analysis.PositionIncrementAttribute)

	for {
		next, err := f.GetInput().IncrementToken()
		if err != nil {
			return false, err
		}
		if !next {
			f.streamState = tfStateIncrementFalse
			return false, nil
		}
		termAttr := f.GetAttributeSource().GetAttribute(analysis.CharTermAttributeType)
		if termAttr == nil {
			f.streamState = tfStateIncrement
			return true, nil
		}
		term := termAttr.(analysis.CharTermAttribute).String()
		if _, stopped := f.stopSet[term]; !stopped {
			f.streamState = tfStateIncrement
			return true, nil
		}
		if posInc != nil {
			// removed token: roll its position increment into the next
			// emitted token.
			posInc.SetPositionIncrement(posInc.GetPositionIncrement() + 1)
		}
	}
}

// End performs end-of-stream operations.
func (f *MockTokenFilter) End() error {
	if f.streamState != tfStateIncrementFalse {
		f.fail("end() called in wrong state")
	}
	f.streamState = tfStateEnd
	return f.GetInput().End()
}

// Close releases resources.
func (f *MockTokenFilter) Close() error {
	if !(f.streamState == tfStateEnd || f.streamState == tfStateClose) {
		f.fail("close() called in wrong state")
	}
	f.streamState = tfStateClose
	return f.GetInput().Close()
}

// Ensure MockTokenFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*MockTokenFilter)(nil)
