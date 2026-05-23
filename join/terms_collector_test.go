// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestTermsCollectorSV_Construction(t *testing.T) {
	c := NewTermsCollectorSV("myField")
	if c == nil {
		t.Fatal("expected non-nil collector")
	}
	if c.ScoreMode() != search.COMPLETE_NO_SCORES {
		t.Errorf("ScoreMode() = %v, want COMPLETE_NO_SCORES", c.ScoreMode())
	}
	if c.GetCollectorTerms() == nil {
		t.Error("GetCollectorTerms() returned nil")
	}
}

func TestTermsCollectorSV_GetLeafCollector_NilLeafReader(t *testing.T) {
	c := NewTermsCollectorSV("f")
	lc, err := c.GetLeafCollector(stubIndexReaderForJoin{})
	if err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}
	if lc == nil {
		t.Fatal("expected non-nil leaf collector")
	}
}

func TestTermsCollectorMV_Construction(t *testing.T) {
	c := NewTermsCollectorMV("myField")
	if c == nil {
		t.Fatal("expected non-nil collector")
	}
	if c.ScoreMode() != search.COMPLETE_NO_SCORES {
		t.Errorf("ScoreMode() = %v, want COMPLETE_NO_SCORES", c.ScoreMode())
	}
	if c.GetCollectorTerms() == nil {
		t.Error("GetCollectorTerms() returned nil")
	}
}

func TestTermsCollectorMV_GetLeafCollector_NilLeafReader(t *testing.T) {
	c := NewTermsCollectorMV("f")
	lc, err := c.GetLeafCollector(stubIndexReaderForJoin{})
	if err != nil {
		t.Fatalf("GetLeafCollector: %v", err)
	}
	if lc == nil {
		t.Fatal("expected non-nil leaf collector")
	}
}

func TestCreateTermsCollector_SV(t *testing.T) {
	c := CreateTermsCollector("f", false)
	if _, ok := c.(*TermsCollectorSV); !ok {
		t.Errorf("CreateTermsCollector(false) returned %T, want *TermsCollectorSV", c)
	}
}

func TestCreateTermsCollector_MV(t *testing.T) {
	c := CreateTermsCollector("f", true)
	if _, ok := c.(*TermsCollectorMV); !ok {
		t.Errorf("CreateTermsCollector(true) returned %T, want *TermsCollectorMV", c)
	}
}
