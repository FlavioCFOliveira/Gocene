package facets

import (
	"testing"
)

func TestNewFacetField(t *testing.T) {
	ff := NewFacetField("category", "electronics")

	if ff.GetDim() != "category" {
		t.Errorf("Expected dim to be 'category', got '%s'", ff.GetDim())
	}

	if ff.GetValue() != "electronics" {
		t.Errorf("Expected value to be 'electronics', got '%s'", ff.GetValue())
	}

	if len(ff.GetPath()) != 0 {
		t.Errorf("Expected empty path, got %v", ff.GetPath())
	}
}

func TestNewFacetFieldWithPath(t *testing.T) {
	path := []string{"electronics", "phones"}
	ff := NewFacetFieldWithPath("category", path, "smartphones")

	if ff.GetDim() != "category" {
		t.Errorf("Expected dim to be 'category', got '%s'", ff.GetDim())
	}

	if ff.GetValue() != "smartphones" {
		t.Errorf("Expected value to be 'smartphones', got '%s'", ff.GetValue())
	}

	if len(ff.GetPath()) != 2 {
		t.Errorf("Expected path length 2, got %d", len(ff.GetPath()))
	}
}

func TestFacetFieldGetFullPath(t *testing.T) {
	ff := NewFacetFieldWithPath("category", []string{"electronics"}, "phones")
	fullPath := ff.GetFullPath()

	if len(fullPath) != 2 {
		t.Errorf("Expected full path length 2, got %d", len(fullPath))
	}

	if fullPath[0] != "electronics" {
		t.Errorf("Expected path[0] to be 'electronics', got '%s'", fullPath[0])
	}

	if fullPath[1] != "phones" {
		t.Errorf("Expected path[1] to be 'phones', got '%s'", fullPath[1])
	}
}

func TestFacetFieldGetPathString(t *testing.T) {
	ff := NewFacetFieldWithPath("category", []string{"electronics", "phones"}, "smartphones")
	pathStr := ff.GetPathString("/")

	expected := "electronics/phones/smartphones"
	if pathStr != expected {
		t.Errorf("Expected path string '%s', got '%s'", expected, pathStr)
	}
}

func TestFacetFieldGetPathStringNoPath(t *testing.T) {
	ff := NewFacetField("category", "electronics")
	pathStr := ff.GetPathString("/")

	if pathStr != "electronics" {
		t.Errorf("Expected path string 'electronics', got '%s'", pathStr)
	}
}

func TestFacetFieldValidate(t *testing.T) {
	// Valid field
	ff := NewFacetField("category", "electronics")
	if err := ff.Validate(); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Empty dimension
	ff2 := NewFacetField("", "electronics")
	if err := ff2.Validate(); err == nil {
		t.Error("Expected error for empty dimension")
	}

	// Empty value
	ff3 := NewFacetField("category", "")
	if err := ff3.Validate(); err == nil {
		t.Error("Expected error for empty value")
	}
}

func TestFacetFieldString(t *testing.T) {
	ff := NewFacetField("category", "electronics")
	if ff.String() != "category=electronics" {
		t.Errorf("Expected 'category=electronics', got '%s'", ff.String())
	}

	ff2 := NewFacetFieldWithPath("category", []string{"electronics"}, "phones")
	if ff2.String() != "category/electronics=phones" {
		t.Errorf("Expected 'category/electronics=phones', got '%s'", ff2.String())
	}
}

func TestNewFacetFields(t *testing.T) {
	ffs := NewFacetFields()

	if ffs.Size() != 0 {
		t.Errorf("Expected size 0, got %d", ffs.Size())
	}
}

func TestFacetFieldsAddAndGet(t *testing.T) {
	ffs := NewFacetFields()
	ff := NewFacetField("category", "electronics")

	ffs.Add(ff)

	if ffs.Size() != 1 {
		t.Errorf("Expected size 1, got %d", ffs.Size())
	}

	retrieved := ffs.Get(0)
	if retrieved != ff {
		t.Error("Expected retrieved field to be the same")
	}

	// Out of bounds
	if ffs.Get(1) != nil {
		t.Error("Expected nil for out of bounds")
	}
}

func TestFacetFieldsGetByDim(t *testing.T) {
	ffs := NewFacetFields()
	ffs.Add(NewFacetField("category", "electronics"))
	ffs.Add(NewFacetField("category", "books"))
	ffs.Add(NewFacetField("price", "high"))

	categoryFields := ffs.GetByDim("category")
	if len(categoryFields) != 2 {
		t.Errorf("Expected 2 category fields, got %d", len(categoryFields))
	}

	priceFields := ffs.GetByDim("price")
	if len(priceFields) != 1 {
		t.Errorf("Expected 1 price field, got %d", len(priceFields))
	}
}

func TestNewFacetFieldBuilder(t *testing.T) {
	ffb := NewFacetFieldBuilder()
	ff, err := ffb.SetDim("category").AddPathComponent("electronics").SetValue("phones").Build()

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if ff.GetDim() != "category" {
		t.Errorf("Expected dim 'category', got '%s'", ff.GetDim())
	}

	if ff.GetValue() != "phones" {
		t.Errorf("Expected value 'phones', got '%s'", ff.GetValue())
	}
}

func TestFacetFieldBuilderSetPath(t *testing.T) {
	ffb := NewFacetFieldBuilder()
	ff, err := ffb.SetDim("category").SetPath([]string{"electronics", "phones"}).SetValue("smartphones").Build()

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(ff.GetPath()) != 2 {
		t.Errorf("Expected path length 2, got %d", len(ff.GetPath()))
	}
}

func TestFacetFieldBuilderInvalid(t *testing.T) {
	ffb := NewFacetFieldBuilder()
	_, err := ffb.Build()

	if err == nil {
		t.Error("Expected error for empty builder")
	}
}
