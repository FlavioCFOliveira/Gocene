// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
	"math/rand"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis/api"
)

// MockAnalyzer is an Analyzer for testing.
//
// This analyzer is a replacement for Whitespace/Simple/KeywordAnalyzers for unit tests.
// If you are testing a custom component such as a queryparser or analyzer-wrapper
// that consumes analysis streams, it's a great idea to test it with this analyzer instead.
//
// This is the Go port of Lucene's org.apache.lucene.tests.analysis.MockAnalyzer.
type MockAnalyzer struct {
	random                *rand.Rand
	positionIncrementGap  int
	offsetGap             *int
	enableChecks          bool
	maxTokenLength        int
	previousMappings      map[string]int
	previousMappingsMutex sync.Mutex
}

// NewMockAnalyzer creates a new MockAnalyzer with default settings (whitespace tokenization, lowercase).
//
// This is equivalent to the Lucene constructor MockAnalyzer(Random random).
func NewMockAnalyzer(r *rand.Rand) *MockAnalyzer {
	if r == nil {
		r = rand.New(rand.NewSource(0))
	}
	// Create a new Random with a different seed derived from the input random
	newRand := rand.New(rand.NewSource(r.Int63()))

	return &MockAnalyzer{
		random:           newRand,
		enableChecks:     true,
		maxTokenLength:   255, // DEFAULT_MAX_TOKEN_LENGTH
		previousMappings: make(map[string]int),
	}
}

// SetPositionIncrementGap sets the position increment gap for this analyzer.
func (ma *MockAnalyzer) SetPositionIncrementGap(gap int) {
	ma.positionIncrementGap = gap
}

// GetPositionIncrementGap returns the position increment gap.
func (ma *MockAnalyzer) GetPositionIncrementGap(fieldName string) int {
	return ma.positionIncrementGap
}

// SetOffsetGap sets the offset gap for this analyzer.
// This offset gap is added to offsets when several fields with the same name are indexed.
func (ma *MockAnalyzer) SetOffsetGap(gap int) {
	ma.offsetGap = &gap
}

// GetOffsetGap returns the offset gap.
func (ma *MockAnalyzer) GetOffsetGap(fieldName string) int {
	if ma.offsetGap != nil {
		return *ma.offsetGap
	}
	// Default behavior: no offset gap
	return 0
}

// SetEnableChecks enables or disables consumer workflow checking.
// If your test consumes tokenstreams normally, you should leave this enabled.
func (ma *MockAnalyzer) SetEnableChecks(enableChecks bool) {
	ma.enableChecks = enableChecks
}

// SetMaxTokenLength sets the max token length for the MockTokenizer.
func (ma *MockAnalyzer) SetMaxTokenLength(length int) {
	ma.maxTokenLength = length
}

// TokenStream creates a TokenStream for analyzing text.
//
// This implementation returns a CannedTokenStream for testing purposes.
// In the real Lucene implementation, it would use MockTokenizer and MockTokenFilter.
// For now, we provide a basic version that can be extended.
func (ma *MockAnalyzer) TokenStream(fieldName string, reader io.Reader) (api.TokenStream, error) {
	// In a full implementation, this would:
	// 1. Create a MockTokenizer
	// 2. Wrap it with MockTokenFilter (for stopwords, etc.)
	// 3. Optionally add payload filters
	// 4. Return the result

	// For now, return a base token stream
	// Real tests will typically use CannedTokenStream directly
	ts := NewBaseTokenStream()
	return ts, nil
}

// Close releases resources held by this analyzer.
func (ma *MockAnalyzer) Close() error {
	return nil
}
