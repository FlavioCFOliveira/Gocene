package search

import (
	"math"
	"strings"
	"testing"
)

func TestNewComplexExplanation(t *testing.T) {
	e := NewComplexExplanation()

	if e == nil {
		t.Fatal("Expected ComplexExplanation to be created")
	}

	if e.GetDetailCount() != 0 {
		t.Errorf("Expected 0 details, got %d", e.GetDetailCount())
	}

	if !e.IsMatch() {
		t.Error("Expected IsMatch to be true by default")
	}
}

func TestNewComplexExplanationWithValue(t *testing.T) {
	e := NewComplexExplanationWithValue(1.5, "test description")

	if e == nil {
		t.Fatal("Expected ComplexExplanation to be created")
	}

	if e.GetValue() != 1.5 {
		t.Errorf("Expected value 1.5, got %f", e.GetValue())
	}

	if e.GetDescription() != "test description" {
		t.Errorf("Expected description 'test description', got '%s'", e.GetDescription())
	}
}

func TestComplexExplanationSetters(t *testing.T) {
	e := NewComplexExplanation()

	// Test SetValue
	e.SetValue(2.5)
	if e.GetValue() != 2.5 {
		t.Errorf("Expected value 2.5, got %f", e.GetValue())
	}

	// Test SetDescription
	e.SetDescription("new description")
	if e.GetDescription() != "new description" {
		t.Errorf("Expected description 'new description', got '%s'", e.GetDescription())
	}

	// Test SetMatch
	e.SetMatch(false)
	if e.IsMatch() {
		t.Error("Expected IsMatch to be false")
	}
}

func TestComplexExplanationAddDetail(t *testing.T) {
	e := NewComplexExplanation()
	detail := NewComplexExplanationWithValue(1.0, "detail")

	e.AddDetail(detail)

	if e.GetDetailCount() != 1 {
		t.Errorf("Expected 1 detail, got %d", e.GetDetailCount())
	}

	details := e.GetDetails()
	if len(details) != 1 {
		t.Errorf("Expected 1 detail in GetDetails, got %d", len(details))
	}
}

func TestComplexExplanationIsValid(t *testing.T) {
	e := NewComplexExplanation()

	// Default value should be valid (0)
	if !e.IsValid() {
		t.Error("Expected explanation to be valid")
	}

	// NaN should be invalid
	e.SetValue(float32(math.NaN()))
	if e.IsValid() {
		t.Error("Expected explanation with NaN to be invalid")
	}
}

func TestComplexExplanationString(t *testing.T) {
	e := NewComplexExplanationWithValue(1.5, "test")
	str := e.String()

	if str == "" {
		t.Error("Expected non-empty string representation")
	}

	if !strings.Contains(str, "1.50000") {
		t.Error("Expected string to contain value")
	}

	if !strings.Contains(str, "test") {
		t.Error("Expected string to contain description")
	}
}

func TestComplexExplanationSummary(t *testing.T) {
	e := NewComplexExplanationWithValue(1.5, "test")
	summary := e.Summary()

	if !strings.Contains(summary, "1.50000") {
		t.Error("Expected summary to contain value")
	}

	if !strings.Contains(summary, "test") {
		t.Error("Expected summary to contain description")
	}
}

func TestComplexExplanationFindExplanation(t *testing.T) {
	root := NewComplexExplanationWithValue(1.0, "root")
	child := NewComplexExplanationWithValue(0.5, "child")
	root.AddDetail(child)

	found := root.FindExplanation("child")
	if found == nil {
		t.Error("Expected to find child explanation")
	}

	if found.GetDescription() != "child" {
		t.Errorf("Expected found description 'child', got '%s'", found.GetDescription())
	}

	notFound := root.FindExplanation("nonexistent")
	if notFound != nil {
		t.Error("Expected nil for non-existent explanation")
	}
}

func TestComplexExplanationGetTotalValue(t *testing.T) {
	root := NewComplexExplanationWithValue(1.0, "root")
	root.AddDetail(NewComplexExplanationWithValue(0.5, "child1"))
	root.AddDetail(NewComplexExplanationWithValue(0.3, "child2"))

	total := root.GetTotalValue()
	if total != 1.8 {
		t.Errorf("Expected total value 1.8, got %f", total)
	}
}

func TestNewExplanationBuilder(t *testing.T) {
	b := NewExplanationBuilder()

	if b == nil {
		t.Fatal("Expected ExplanationBuilder to be created")
	}
}

func TestExplanationBuilder(t *testing.T) {
	b := NewExplanationBuilder()

	result := b.SetValue(1.5).
		SetDescription("test").
		SetMatch(false).
		AddDetailWithValue(0.5, "detail").
		Build()

	if result.GetValue() != 1.5 {
		t.Errorf("Expected value 1.5, got %f", result.GetValue())
	}

	if result.GetDescription() != "test" {
		t.Errorf("Expected description 'test', got '%s'", result.GetDescription())
	}

	if result.IsMatch() {
		t.Error("Expected IsMatch to be false")
	}

	if result.GetDetailCount() != 1 {
		t.Errorf("Expected 1 detail, got %d", result.GetDetailCount())
	}
}

func TestNewExplanationFormatter(t *testing.T) {
	f := NewExplanationFormatter()

	if f == nil {
		t.Fatal("Expected ExplanationFormatter to be created")
	}
}

func TestExplanationFormatterFormat(t *testing.T) {
	f := NewExplanationFormatter()

	e := NewComplexExplanationWithValue(1.5, "test")
	formatted := f.Format(e)

	if formatted == "" {
		t.Error("Expected non-empty formatted string")
	}
}
