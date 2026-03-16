package facets

import (
	"testing"
)

func TestNewTaxonomyWriter(t *testing.T) {
	tw := NewTaxonomyWriter()

	if tw == nil {
		t.Fatal("Expected TaxonomyWriter to be created")
	}

	if !tw.IsOpen() {
		t.Error("Expected writer to be open")
	}

	if tw.GetSize() != 0 {
		t.Errorf("Expected size 0, got %d", tw.GetSize())
	}
}

func TestTaxonomyWriterAddCategory(t *testing.T) {
	tw := NewTaxonomyWriter()

	// Add first category
	ord1, err := tw.AddCategory("/a")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if ord1 != 1 {
		t.Errorf("Expected ordinal 1, got %d", ord1)
	}

	// Add same category again (should return same ordinal)
	ord1Again, err := tw.AddCategory("/a")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if ord1Again != ord1 {
		t.Errorf("Expected same ordinal %d, got %d", ord1, ord1Again)
	}

	// Add child category
	ord2, err := tw.AddCategory("/a/b")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if ord2 != 2 {
		t.Errorf("Expected ordinal 2, got %d", ord2)
	}

	if tw.GetSize() != 2 {
		t.Errorf("Expected size 2, got %d", tw.GetSize())
	}
}

func TestTaxonomyWriterAddCategoryPath(t *testing.T) {
	tw := NewTaxonomyWriter()

	components := []string{"a", "b", "c"}
	ord, err := tw.AddCategoryPath(components)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if ord != 3 { // a=1, a/b=2, a/b/c=3
		t.Errorf("Expected ordinal 3, got %d", ord)
	}
}

func TestTaxonomyWriterAddCategoryPathEmpty(t *testing.T) {
	tw := NewTaxonomyWriter()

	_, err := tw.AddCategoryPath([]string{})
	if err == nil {
		t.Error("Expected error for empty path")
	}
}

func TestTaxonomyWriterGetOrdinal(t *testing.T) {
	tw := NewTaxonomyWriter()
	tw.AddCategory("/a")

	ord := tw.GetOrdinal("/a")
	if ord != 1 {
		t.Errorf("Expected ordinal 1, got %d", ord)
	}

	// Non-existent
	if tw.GetOrdinal("/b") != -1 {
		t.Error("Expected -1 for non-existent path")
	}
}

func TestTaxonomyWriterGetPath(t *testing.T) {
	tw := NewTaxonomyWriter()
	tw.AddCategory("/a")

	path := tw.GetPath(1)
	if path != "/a" {
		t.Errorf("Expected path '/a', got '%s'", path)
	}

	// Non-existent
	if tw.GetPath(999) != "" {
		t.Error("Expected empty string for non-existent ordinal")
	}
}

func TestTaxonomyWriterCommit(t *testing.T) {
	tw := NewTaxonomyWriter()
	tw.AddCategory("/a")

	if err := tw.Commit(); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestTaxonomyWriterClose(t *testing.T) {
	tw := NewTaxonomyWriter()
	tw.AddCategory("/a")

	if err := tw.Close(); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if tw.IsOpen() {
		t.Error("Expected writer to be closed")
	}

	// Try to add after close
	_, err := tw.AddCategory("/b")
	if err == nil {
		t.Error("Expected error when adding to closed writer")
	}
}

func TestTaxonomyWriterGetReader(t *testing.T) {
	tw := NewTaxonomyWriter()
	tw.AddCategory("/a")
	tw.AddCategory("/a/b")

	reader, err := tw.GetReader()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if reader == nil {
		t.Fatal("Expected reader to be created")
	}

	if reader.GetSize() != 2 {
		t.Errorf("Expected reader size 2, got %d", reader.GetSize())
	}

	// Check ordinals
	if reader.GetOrdinal("/a") != 1 {
		t.Error("Expected ordinal 1 for '/a'")
	}

	if reader.GetOrdinal("/a/b") != 2 {
		t.Error("Expected ordinal 2 for '/a/b'")
	}
}

func TestTaxonomyWriterGetReaderClosed(t *testing.T) {
	tw := NewTaxonomyWriter()
	tw.Close()

	_, err := tw.GetReader()
	if err == nil {
		t.Error("Expected error for closed writer")
	}
}

func TestNewTaxonomyWriterWithReader(t *testing.T) {
	// Create a reader with some data
	reader := NewTaxonomyReader()
	reader.ordinals["/a"] = 1
	reader.paths[1] = "/a"
	reader.nextOrdinal = 2

	tw := NewTaxonomyWriterWithReader(reader)

	if tw.GetOrdinal("/a") != 1 {
		t.Error("Expected ordinal 1 for '/a'")
	}

	if tw.GetPath(1) != "/a" {
		t.Error("Expected path '/a' for ordinal 1")
	}
}

func TestNewTaxonomyWriterFactory(t *testing.T) {
	twf := NewTaxonomyWriterFactory("/tmp/taxonomy")

	if twf == nil {
		t.Fatal("Expected TaxonomyWriterFactory to be created")
	}
}

func TestTaxonomyWriterFactoryOpen(t *testing.T) {
	twf := NewTaxonomyWriterFactory("/tmp/taxonomy")

	tw, err := twf.Open()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if tw == nil {
		t.Fatal("Expected TaxonomyWriter to be created")
	}
}

func TestTaxonomyWriterFactoryOpenWithReader(t *testing.T) {
	twf := NewTaxonomyWriterFactory("/tmp/taxonomy")
	reader := NewTaxonomyReader()

	tw, err := twf.OpenWithReader(reader)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if tw == nil {
		t.Fatal("Expected TaxonomyWriter to be created")
	}
}
