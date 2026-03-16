package facets

import (
	"testing"
)

func TestNewLabelAndValue(t *testing.T) {
	lv := NewLabelAndValue("electronics", 42)

	if lv.Label != "electronics" {
		t.Errorf("Expected Label to be 'electronics', got '%s'", lv.Label)
	}

	if lv.Value != 42 {
		t.Errorf("Expected Value to be 42, got %d", lv.Value)
	}
}

func TestLabelAndValueString(t *testing.T) {
	lv := NewLabelAndValue("books", 100)
	expected := "books (100)"

	if lv.String() != expected {
		t.Errorf("Expected String() to be '%s', got '%s'", expected, lv.String())
	}
}

func TestNewFacetResult(t *testing.T) {
	fr := NewFacetResult("category")

	if fr.Dim != "category" {
		t.Errorf("Expected Dim to be 'category', got '%s'", fr.Dim)
	}

	if fr.LabelValues == nil {
		t.Error("Expected LabelValues to be initialized")
	}

	if len(fr.LabelValues) != 0 {
		t.Errorf("Expected LabelValues to be empty, got %d items", len(fr.LabelValues))
	}

	if len(fr.Path) != 0 {
		t.Errorf("Expected Path to be empty, got %d items", len(fr.Path))
	}
}

func TestNewFacetResultWithPath(t *testing.T) {
	path := []string{"electronics", "phones"}
	fr := NewFacetResultWithPath("category", path)

	if fr.Dim != "category" {
		t.Errorf("Expected Dim to be 'category', got '%s'", fr.Dim)
	}

	if len(fr.Path) != 2 {
		t.Errorf("Expected Path to have 2 items, got %d", len(fr.Path))
	}

	if fr.Path[0] != "electronics" {
		t.Errorf("Expected Path[0] to be 'electronics', got '%s'", fr.Path[0])
	}

	if fr.Path[1] != "phones" {
		t.Errorf("Expected Path[1] to be 'phones', got '%s'", fr.Path[1])
	}
}

func TestFacetResultAddLabelValue(t *testing.T) {
	fr := NewFacetResult("category")
	lv := NewLabelAndValue("books", 50)

	fr.AddLabelValue(lv)

	if len(fr.LabelValues) != 1 {
		t.Errorf("Expected 1 LabelValue, got %d", len(fr.LabelValues))
	}

	if fr.LabelValues[0].Label != "books" {
		t.Errorf("Expected Label to be 'books', got '%s'", fr.LabelValues[0].Label)
	}
}

func TestFacetResultAddLabelValueWithCount(t *testing.T) {
	fr := NewFacetResult("category")
	lv := fr.AddLabelValueWithCount("electronics", 25)

	if len(fr.LabelValues) != 1 {
		t.Errorf("Expected 1 LabelValue, got %d", len(fr.LabelValues))
	}

	if lv.Label != "electronics" {
		t.Errorf("Expected Label to be 'electronics', got '%s'", lv.Label)
	}

	if lv.Value != 25 {
		t.Errorf("Expected Value to be 25, got %d", lv.Value)
	}
}

func TestFacetResultString(t *testing.T) {
	fr := NewFacetResult("category")
	fr.Value = 100

	expected := "category=100"
	if fr.String() != expected {
		t.Errorf("Expected String() to be '%s', got '%s'", expected, fr.String())
	}
}

func TestFacetResultStringWithPath(t *testing.T) {
	fr := NewFacetResultWithPath("category", []string{"electronics", "phones"})
	fr.Value = 50

	expected := "category/[electronics phones]=50"
	if fr.String() != expected {
		t.Errorf("Expected String() to be '%s', got '%s'", expected, fr.String())
	}
}

func TestNewFacetResults(t *testing.T) {
	frs := NewFacetResults()

	if frs.Results == nil {
		t.Error("Expected Results to be initialized")
	}

	if len(frs.Results) != 0 {
		t.Errorf("Expected Results to be empty, got %d items", len(frs.Results))
	}
}

func TestFacetResultsAddResult(t *testing.T) {
	frs := NewFacetResults()
	fr := NewFacetResult("category")

	frs.AddResult(fr)

	if len(frs.Results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(frs.Results))
	}
}

func TestFacetResultsGetResult(t *testing.T) {
	frs := NewFacetResults()
	fr := NewFacetResult("category")
	frs.AddResult(fr)

	result := frs.GetResult("category")
	if result == nil {
		t.Error("Expected to find 'category' result")
	}

	if result.Dim != "category" {
		t.Errorf("Expected Dim to be 'category', got '%s'", result.Dim)
	}

	// Test non-existent dimension
	result = frs.GetResult("nonexistent")
	if result != nil {
		t.Error("Expected nil for non-existent dimension")
	}
}

func TestFacetResultsSize(t *testing.T) {
	frs := NewFacetResults()

	if frs.Size() != 0 {
		t.Errorf("Expected Size() to be 0, got %d", frs.Size())
	}

	frs.AddResult(NewFacetResult("category"))
	frs.AddResult(NewFacetResult("price"))

	if frs.Size() != 2 {
		t.Errorf("Expected Size() to be 2, got %d", frs.Size())
	}
}
