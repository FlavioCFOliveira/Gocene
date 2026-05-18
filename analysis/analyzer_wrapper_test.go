// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestAnalyzerWrapper_DelegatesByField verifies that AnalyzerWrapper picks
// the wrapped Analyzer per field name and delegates token production to it.
// Mirrors the per-field intent of Lucene's PerFieldAnalyzerWrapper which is
// the most common consumer of AnalyzerWrapper.
func TestAnalyzerWrapper_DelegatesByField(t *testing.T) {
	whitespace := NewWhitespaceAnalyzer()
	defer whitespace.Close()
	simple := NewSimpleAnalyzer()
	defer simple.Close()

	wrapper := NewAnalyzerWrapper(func(fieldName string) Analyzer {
		if fieldName == "ws" {
			return whitespace
		}
		return simple
	})
	defer wrapper.Close()

	cases := []struct {
		field    string
		input    string
		expected []string
	}{
		// WhitespaceAnalyzer: split on whitespace, no lowercasing.
		{field: "ws", input: "Hello World", expected: []string{"Hello", "World"}},
		// SimpleAnalyzer: letter tokenizer + lowercasing.
		{field: "any", input: "Hello, World", expected: []string{"hello", "world"}},
	}
	for _, tc := range cases {
		t.Run(tc.field+"/"+tc.input, func(t *testing.T) {
			stream, err := wrapper.TokenStream(tc.field, strings.NewReader(tc.input))
			if err != nil {
				t.Fatalf("TokenStream: %v", err)
			}
			defer stream.Close()
			got := collectTerms(t, stream)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

// TestAnalyzerWrapper_WrapReaderApplied verifies that WrapReader is applied
// to the input Reader before delegation. Models a CharFilter pre-processing
// step: replace 'X' with 'a' in the input.
func TestAnalyzerWrapper_WrapReaderApplied(t *testing.T) {
	inner := NewWhitespaceAnalyzer()
	defer inner.Close()

	wrapper := NewAnalyzerWrapper(func(_ string) Analyzer { return inner })
	wrapper.WrapReader = func(_ string, r io.Reader) io.Reader {
		b, err := io.ReadAll(r)
		if err != nil {
			return strings.NewReader("")
		}
		return strings.NewReader(strings.ReplaceAll(string(b), "X", "a"))
	}
	defer wrapper.Close()

	stream, err := wrapper.TokenStream("f", strings.NewReader("XbcX defX"))
	if err != nil {
		t.Fatalf("TokenStream: %v", err)
	}
	defer stream.Close()
	got := collectTerms(t, stream)
	want := []string{"abca", "defa"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// TestAnalyzerWrapper_WrapTokenStreamApplied verifies that WrapTokenStream
// can chain extra filters onto the wrapped Analyzer's output. Wraps the
// underlying stream in a LowerCaseFilter and checks the result.
func TestAnalyzerWrapper_WrapTokenStreamApplied(t *testing.T) {
	inner := NewWhitespaceAnalyzer()
	defer inner.Close()

	wrapper := NewAnalyzerWrapper(func(_ string) Analyzer { return inner })
	wrapper.WrapTokenStream = func(_ string, in TokenStream) TokenStream {
		return NewLowerCaseFilter(in)
	}
	defer wrapper.Close()

	stream, err := wrapper.TokenStream("f", strings.NewReader("Hello WORLD"))
	if err != nil {
		t.Fatalf("TokenStream: %v", err)
	}
	defer stream.Close()
	got := collectTerms(t, stream)
	want := []string{"hello", "world"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// TestDelegatingAnalyzerWrapper_DelegatesByField verifies that
// DelegatingAnalyzerWrapper picks the wrapped Analyzer per field and never
// modifies the resulting TokenStream.
func TestDelegatingAnalyzerWrapper_DelegatesByField(t *testing.T) {
	whitespace := NewWhitespaceAnalyzer()
	defer whitespace.Close()
	simple := NewSimpleAnalyzer()
	defer simple.Close()

	wrapper := NewDelegatingAnalyzerWrapper(func(fieldName string) Analyzer {
		if fieldName == "ws" {
			return whitespace
		}
		return simple
	})
	defer wrapper.Close()

	cases := []struct {
		field    string
		input    string
		expected []string
	}{
		{field: "ws", input: "Hello World", expected: []string{"Hello", "World"}},
		{field: "x", input: "Hello, World", expected: []string{"hello", "world"}},
	}
	for _, tc := range cases {
		t.Run(tc.field+"/"+tc.input, func(t *testing.T) {
			stream, err := wrapper.TokenStream(tc.field, strings.NewReader(tc.input))
			if err != nil {
				t.Fatalf("TokenStream: %v", err)
			}
			defer stream.Close()
			got := collectTerms(t, stream)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

// collectTerms drains the given TokenStream and returns the CharTermAttribute
// values in order. It is a tiny helper to keep the tests focused on the
// AnalyzerWrapper behaviour rather than attribute-source plumbing.
func collectTerms(t *testing.T, stream TokenStream) []string {
	t.Helper()
	var terms []string
	for {
		ok, err := stream.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		src, ok := stream.(interface {
			GetAttributeSource() *util.AttributeSource
			GetAttribute(string) AttributeImpl
		})
		if !ok {
			continue
		}
		attr := src.GetAttribute("CharTermAttribute")
		if attr == nil {
			continue
		}
		if term, ok := attr.(CharTermAttribute); ok {
			terms = append(terms, term.String())
		}
	}
	return terms
}
