// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil

import (
	"math/rand"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// MockFixedLengthPayloadFilter is the Go port of Lucene 10.4.0's
// org.apache.lucene.tests.analysis.MockFixedLengthPayloadFilter.
//
// It adds a fixed-length random payload to every token. The Go port uses
// []byte where Lucene uses BytesRef.
//
// Lucene reference:
//
//	lucene/test-framework/src/java/org/apache/lucene/tests/analysis/MockFixedLengthPayloadFilter.java
type MockFixedLengthPayloadFilter struct {
	*analysis.BaseTokenFilter

	payloadAttr analysis.PayloadAttribute
	random      *rand.Rand
	length      int
}

// NewMockFixedLengthPayloadFilter creates a filter adding length-byte payloads.
func NewMockFixedLengthPayloadFilter(input analysis.TokenStream, length int, r *rand.Rand) *MockFixedLengthPayloadFilter {
	if input == nil {
		panic("MockFixedLengthPayloadFilter: input must not be nil")
	}
	if r == nil {
		r = rand.New(rand.NewSource(0))
	}
	f := &MockFixedLengthPayloadFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		random:          r,
		length:          length,
	}
	f.AddAttribute(analysis.NewPayloadAttributeImpl())
	f.payloadAttr = f.GetAttributeSource().GetAttribute(analysis.PayloadAttributeType).(analysis.PayloadAttribute)
	return f
}

// IncrementToken advances to the next token and assigns a payload.
func (f *MockFixedLengthPayloadFilter) IncrementToken() (bool, error) {
	next, err := f.GetInput().IncrementToken()
	if err != nil {
		return false, err
	}
	if !next {
		return false, nil
	}
	payload := make([]byte, f.length)
	f.random.Read(payload)
	f.payloadAttr.SetPayload(payload)
	return true, nil
}

// Ensure MockFixedLengthPayloadFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*MockFixedLengthPayloadFilter)(nil)

// MockVariableLengthPayloadFilter is the Go port of Lucene 10.4.0's
// org.apache.lucene.tests.analysis.MockVariableLengthPayloadFilter.
//
// It adds a variable-length random payload (0..length) to every token.
//
// Lucene reference:
//
//	lucene/test-framework/src/java/org/apache/lucene/tests/analysis/MockVariableLengthPayloadFilter.java
type MockVariableLengthPayloadFilter struct {
	*analysis.BaseTokenFilter

	payloadAttr analysis.PayloadAttribute
	random      *rand.Rand
	length      int
}

// NewMockVariableLengthPayloadFilter creates a filter adding 0..length-byte payloads.
func NewMockVariableLengthPayloadFilter(input analysis.TokenStream, length int, r *rand.Rand) *MockVariableLengthPayloadFilter {
	if input == nil {
		panic("MockVariableLengthPayloadFilter: input must not be nil")
	}
	if r == nil {
		r = rand.New(rand.NewSource(0))
	}
	f := &MockVariableLengthPayloadFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		random:          r,
		length:          length,
	}
	f.AddAttribute(analysis.NewPayloadAttributeImpl())
	f.payloadAttr = f.GetAttributeSource().GetAttribute(analysis.PayloadAttributeType).(analysis.PayloadAttribute)
	return f
}

// IncrementToken advances to the next token and assigns a variable-length payload.
func (f *MockVariableLengthPayloadFilter) IncrementToken() (bool, error) {
	next, err := f.GetInput().IncrementToken()
	if err != nil {
		return false, err
	}
	if !next {
		return false, nil
	}
	n := f.random.Intn(f.length + 1)
	payload := make([]byte, n)
	f.random.Read(payload)
	f.payloadAttr.SetPayload(payload)
	return true, nil
}

// Ensure MockVariableLengthPayloadFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*MockVariableLengthPayloadFilter)(nil)
