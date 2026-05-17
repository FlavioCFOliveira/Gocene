// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestFilteringTokenFilter_KeepsAccepted verifies the basic filter contract:
// only tokens for which AcceptFn returns true are emitted.
func TestFilteringTokenFilter_KeepsAccepted(t *testing.T) {
	tok := NewWhitespaceTokenizer()
	if err := tok.SetReader(strings.NewReader("alpha beta gamma delta")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}

	// Accept only tokens whose CharTermAttribute starts with 'a' or 'g'.
	termAttr := lookupCharTermAttribute(t, tok)
	f := NewFilteringTokenFilter(tok, func() (bool, error) {
		s := termAttr.String()
		return strings.HasPrefix(s, "a") || strings.HasPrefix(s, "g"), nil
	})

	got := collectTerms(t, f)
	want := []string{"alpha", "gamma"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// TestFilteringTokenFilter_PositionIncrementGap verifies that the
// PositionIncrement of an accepted token reflects the increments of skipped
// tokens, matching Lucene's contract for stop-word style filtering.
func TestFilteringTokenFilter_PositionIncrementGap(t *testing.T) {
	tok := NewWhitespaceTokenizer()
	if err := tok.SetReader(strings.NewReader("a b c d")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}

	termAttr := lookupCharTermAttribute(t, tok)
	posAttr := lookupPositionIncrementAttribute(t, tok)
	f := NewFilteringTokenFilter(tok, func() (bool, error) {
		// drop "b" and "c"
		s := termAttr.String()
		return s != "b" && s != "c", nil
	})

	var terms []string
	var positions []int
	for {
		ok, err := f.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		terms = append(terms, termAttr.String())
		positions = append(positions, posAttr.GetPositionIncrement())
	}

	wantTerms := []string{"a", "d"}
	if !reflect.DeepEqual(terms, wantTerms) {
		t.Errorf("expected terms %v, got %v", wantTerms, terms)
	}
	// "a" carries posInc 1; "d" carries posInc 3 (1 for itself + 2 skipped).
	wantPos := []int{1, 3}
	if !reflect.DeepEqual(positions, wantPos) {
		t.Errorf("expected positions %v, got %v", wantPos, positions)
	}
}

// lookupCharTermAttribute fetches the CharTermAttribute from the stream's
// AttributeSource. Fails the test if missing.
func lookupCharTermAttribute(t *testing.T, stream TokenStream) CharTermAttribute {
	t.Helper()
	src, ok := stream.(interface{ GetAttributeSource() *AttributeSource })
	if !ok {
		t.Fatal("stream has no AttributeSource")
	}
	attr := src.GetAttributeSource().GetAttribute("CharTermAttribute")
	if attr == nil {
		t.Fatal("CharTermAttribute not found")
	}
	term, ok := attr.(CharTermAttribute)
	if !ok {
		t.Fatalf("attribute is not a CharTermAttribute: %T", attr)
	}
	return term
}

// lookupPositionIncrementAttribute fetches the PositionIncrementAttribute
// from the stream's AttributeSource. Fails the test if missing.
func lookupPositionIncrementAttribute(t *testing.T, stream TokenStream) PositionIncrementAttribute {
	t.Helper()
	src, ok := stream.(interface{ GetAttributeSource() *AttributeSource })
	if !ok {
		t.Fatal("stream has no AttributeSource")
	}
	attr := src.GetAttributeSource().GetAttribute("PositionIncrementAttribute")
	if attr == nil {
		t.Fatal("PositionIncrementAttribute not found")
	}
	pos, ok := attr.(PositionIncrementAttribute)
	if !ok {
		t.Fatalf("attribute is not a PositionIncrementAttribute: %T", attr)
	}
	return pos
}
