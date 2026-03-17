// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"testing"
)

func TestNewTopChildrenResult(t *testing.T) {
	result := NewTopChildrenResult("category")

	if result.Dim != "category" {
		t.Errorf("expected Dim='category', got '%s'", result.Dim)
	}

	if result.Path == nil {
		t.Error("expected Path to be initialized")
	}

	if len(result.Path) != 0 {
		t.Errorf("expected empty Path, got %v", result.Path)
	}

	if result.LabelValues == nil {
		t.Error("expected LabelValues to be initialized")
	}

	if len(result.LabelValues) != 0 {
		t.Errorf("expected empty LabelValues, got %v", result.LabelValues)
	}
}

func TestNewTopChildrenResultWithPath(t *testing.T) {
	path := []string{"electronics", "phones"}
	result := NewTopChildrenResultWithPath("category", path)

	if result.Dim != "category" {
		t.Errorf("expected Dim='category', got '%s'", result.Dim)
	}

	if len(result.Path) != 2 {
		t.Errorf("expected Path length 2, got %d", len(result.Path))
	}

	if result.Path[0] != "electronics" || result.Path[1] != "phones" {
		t.Errorf("expected Path=['electronics', 'phones'], got %v", result.Path)
	}
}

func TestTopChildrenResult_AddLabelValue(t *testing.T) {
	result := NewTopChildrenResult("category")
	lv := NewLabelAndValue("books", 10)

	result.AddLabelValue(lv)

	if len(result.LabelValues) != 1 {
		t.Errorf("expected 1 LabelValue, got %d", len(result.LabelValues))
	}

	if result.LabelValues[0].Label != "books" {
		t.Errorf("expected Label='books', got '%s'", result.LabelValues[0].Label)
	}

	if result.LabelValues[0].Value != 10 {
		t.Errorf("expected Value=10, got %d", result.LabelValues[0].Value)
	}
}

func TestTopChildrenResult_GetTotalCount(t *testing.T) {
	result := NewTopChildrenResult("category")
	result.AddLabelValue(NewLabelAndValue("books", 10))
	result.AddLabelValue(NewLabelAndValue("electronics", 20))
	result.AddLabelValue(NewLabelAndValue("clothing", 30))

	total := result.GetTotalCount()
	if total != 60 {
		t.Errorf("expected total count 60, got %d", total)
	}
}

func TestTopChildrenResult_Size(t *testing.T) {
	result := NewTopChildrenResult("category")

	if result.Size() != 0 {
		t.Errorf("expected size 0, got %d", result.Size())
	}

	result.AddLabelValue(NewLabelAndValue("books", 10))
	if result.Size() != 1 {
		t.Errorf("expected size 1, got %d", result.Size())
	}

	result.AddLabelValue(NewLabelAndValue("electronics", 20))
	if result.Size() != 2 {
		t.Errorf("expected size 2, got %d", result.Size())
	}
}

func TestTopChildrenResult_IsEmpty(t *testing.T) {
	result := NewTopChildrenResult("category")

	if !result.IsEmpty() {
		t.Error("expected IsEmpty() to be true for new result")
	}

	result.AddLabelValue(NewLabelAndValue("books", 10))
	if result.IsEmpty() {
		t.Error("expected IsEmpty() to be false after adding label value")
	}
}

func TestTopChildrenResult_GetTotalCount_Empty(t *testing.T) {
	result := NewTopChildrenResult("category")

	total := result.GetTotalCount()
	if total != 0 {
		t.Errorf("expected total count 0 for empty result, got %d", total)
	}
}
