// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"testing"
	"time"
)

func TestFuzzyConfig(t *testing.T) {
	c := NewFuzzyConfig()
	if c.GetMinSimilarity() != 2.0 {
		t.Errorf("default minSim = %f, want 2.0", c.GetMinSimilarity())
	}
	if c.GetPrefixLength() != 0 {
		t.Errorf("default prefixLen = %d, want 0", c.GetPrefixLength())
	}
	c.SetMinSimilarity(0.8)
	c.SetPrefixLength(3)
	if c.GetMinSimilarity() != 0.8 {
		t.Errorf("minSim = %f, want 0.8", c.GetMinSimilarity())
	}
	if c.GetPrefixLength() != 3 {
		t.Errorf("prefixLen = %d, want 3", c.GetPrefixLength())
	}
}

func TestNumberDateFormat(t *testing.T) {
	f := NewNumberDateFormat("2006-01-02")
	now := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	formatted := f.Format(now)
	if formatted != "2024-03-15" {
		t.Errorf("Format() = %q, want 2024-03-15", formatted)
	}
	parsed, err := f.Parse("2024-03-15")
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if parsed.Year() != 2024 || parsed.Month() != 3 || parsed.Day() != 15 {
		t.Errorf("Parse() = %v unexpected", parsed)
	}
	if f.GetLayout() != "2006-01-02" {
		t.Errorf("GetLayout() = %q", f.GetLayout())
	}
}

func TestNumberDateFormat_DefaultLayout(t *testing.T) {
	f := NewNumberDateFormat("")
	if f.GetLayout() != DefaultNumberDateLayout {
		t.Errorf("empty layout should use default, got %q", f.GetLayout())
	}
}

func TestPointsConfig(t *testing.T) {
	tests := []struct {
		pt          PointsType
		dims        int
		bytesPerDim int
	}{
		{PointsTypeInt, 1, 4},
		{PointsTypeLong, 2, 8},
		{PointsTypeFloat, 1, 4},
		{PointsTypeDouble, 3, 8},
	}
	for _, tc := range tests {
		c := NewPointsConfig(tc.pt, tc.dims)
		if c.GetType() != tc.pt {
			t.Errorf("GetType() = %v, want %v", c.GetType(), tc.pt)
		}
		if c.GetNumDims() != tc.dims {
			t.Errorf("GetNumDims() = %d, want %d", c.GetNumDims(), tc.dims)
		}
		if c.GetBytesPerDim() != tc.bytesPerDim {
			t.Errorf("GetBytesPerDim() = %d, want %d", c.GetBytesPerDim(), tc.bytesPerDim)
		}
	}
}

func TestStandardQueryConfigHandlerFull(t *testing.T) {
	h := NewStandardQueryConfigHandlerFull()

	// Defaults
	if !h.GetEnablePositionIncrements() {
		t.Error("default enablePositionIncrements should be true")
	}
	if h.GetAllowLeadingWildcard() {
		t.Error("default allowLeadingWildcard should be false")
	}
	if !h.GetLowercaseExpandedTerms() {
		t.Error("default lowercaseExpandedTerms should be true")
	}
	if h.GetPhraseSlop() != 0 {
		t.Errorf("default phraseSlop = %d, want 0", h.GetPhraseSlop())
	}
	if h.GetFuzzyMinSim() != 2.0 {
		t.Errorf("default fuzzyMinSim = %f, want 2.0", h.GetFuzzyMinSim())
	}
	if h.GetFuzzyPrefixLength() != 0 {
		t.Errorf("default fuzzyPrefixLength = %d, want 0", h.GetFuzzyPrefixLength())
	}

	// Setters
	h.SetEnablePositionIncrements(false)
	h.SetAllowLeadingWildcard(true)
	h.SetLowercaseExpandedTerms(false)
	h.SetPhraseSlop(2)
	h.SetFuzzyMinSim(0.5)
	h.SetFuzzyPrefixLength(4)

	if h.GetEnablePositionIncrements() {
		t.Error("enablePositionIncrements should now be false")
	}
	if !h.GetAllowLeadingWildcard() {
		t.Error("allowLeadingWildcard should now be true")
	}
	if h.GetLowercaseExpandedTerms() {
		t.Error("lowercaseExpandedTerms should now be false")
	}
	if h.GetPhraseSlop() != 2 {
		t.Errorf("phraseSlop = %d, want 2", h.GetPhraseSlop())
	}
	if h.GetFuzzyMinSim() != 0.5 {
		t.Errorf("fuzzyMinSim = %f, want 0.5", h.GetFuzzyMinSim())
	}
	if h.GetFuzzyPrefixLength() != 4 {
		t.Errorf("fuzzyPrefixLength = %d, want 4", h.GetFuzzyPrefixLength())
	}

	// PointsConfig
	pc := NewPointsConfig(PointsTypeInt, 1)
	h.SetPointsConfig("price", pc)
	got := h.GetPointsConfig("price")
	if got != pc {
		t.Error("GetPointsConfig returned wrong config")
	}
	if h.GetPointsConfig("missing") != nil {
		t.Error("GetPointsConfig for missing field should return nil")
	}
}

func TestSSPTokenConstants(t *testing.T) {
	if SSPKindEOF != 0 {
		t.Errorf("SSPKindEOF = %d, want 0", SSPKindEOF)
	}
	if SSPKindNumber != 18 {
		t.Errorf("SSPKindNumber = %d, want 18", SSPKindNumber)
	}
	if len(SSPTokenImage) != 19 {
		t.Errorf("SSPTokenImage len = %d, want 19", len(SSPTokenImage))
	}
}

func TestStandardSyntaxParserToken(t *testing.T) {
	tok := NewStandardSyntaxParserToken(SSPKindTerm, "hello")
	if tok.Kind != SSPKindTerm || tok.Image != "hello" {
		t.Errorf("token = {%d, %q}", tok.Kind, tok.Image)
	}
}

func TestStandardSyntaxParserTokenMgrError(t *testing.T) {
	err := NewStandardSyntaxParserTokenMgrError("lex error", 1)
	if err.Error() != "lex error" {
		t.Errorf("Error() = %q", err.Error())
	}
	if err.ErrorCode != 1 {
		t.Errorf("ErrorCode = %d", err.ErrorCode)
	}
}

func TestStandardSyntaxParserParseException(t *testing.T) {
	cause := NewStandardSyntaxParserTokenMgrError("lex", 0)
	exc := NewStandardSyntaxParserParseException("parse error", cause)
	if exc.Error() != "parse error" {
		t.Errorf("Error() = %q", exc.Error())
	}
	if exc.Unwrap() != cause {
		t.Error("Unwrap() should return cause")
	}
}
