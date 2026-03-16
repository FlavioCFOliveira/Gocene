package facets

import (
	"testing"
)

func TestNewTaxonomyReader(t *testing.T) {
	tr := NewTaxonomyReader()

	if tr == nil {
		t.Fatal("Expected TaxonomyReader to be created")
	}

	if tr.GetSize() != 0 {
		t.Errorf("Expected size 0, got %d", tr.GetSize())
	}
}

func TestTaxonomyReaderGetOrdinal(t *testing.T) {
	tr := NewTaxonomyReader()

	// Non-existent path
	if tr.GetOrdinal("/a/b") != -1 {
		t.Error("Expected -1 for non-existent path")
	}
}

func TestTaxonomyReaderGetPath(t *testing.T) {
	tr := NewTaxonomyReader()

	// Non-existent ordinal
	if tr.GetPath(1) != "" {
		t.Error("Expected empty string for non-existent ordinal")
	}
}

func TestTaxonomyReaderGetParent(t *testing.T) {
	tr := NewTaxonomyReader()

	// Non-existent ordinal
	if tr.GetParent(1) != -1 {
		t.Error("Expected -1 for non-existent ordinal")
	}
}

func TestTaxonomyReaderGetChildren(t *testing.T) {
	tr := NewTaxonomyReader()

	children := tr.GetChildren(1)
	if len(children) != 0 {
		t.Errorf("Expected empty children, got %d", len(children))
	}
}

func TestTaxonomyReaderGetRootOrdinals(t *testing.T) {
	tr := NewTaxonomyReader()

	roots := tr.GetRootOrdinals()
	if len(roots) != 0 {
		t.Errorf("Expected 0 roots, got %d", len(roots))
	}
}

func TestTaxonomyReaderGetSiblings(t *testing.T) {
	tr := NewTaxonomyReader()

	siblings := tr.GetSiblings(1)
	if len(siblings) != 0 {
		t.Errorf("Expected 0 siblings, got %d", len(siblings))
	}
}

func TestTaxonomyReaderGetDepth(t *testing.T) {
	tr := NewTaxonomyReader()

	depth := tr.GetDepth(1)
	if depth != 0 {
		t.Errorf("Expected depth 0, got %d", depth)
	}
}

func TestTaxonomyReaderGetAncestors(t *testing.T) {
	tr := NewTaxonomyReader()

	ancestors := tr.GetAncestors(1)
	if len(ancestors) != 0 {
		t.Errorf("Expected 0 ancestors, got %d", len(ancestors))
	}
}

func TestTaxonomyReaderGetDescendants(t *testing.T) {
	tr := NewTaxonomyReader()

	descendants := tr.GetDescendants(1)
	if len(descendants) != 0 {
		t.Errorf("Expected 0 descendants, got %d", len(descendants))
	}
}

func TestTaxonomyReaderIsDescendantOf(t *testing.T) {
	tr := NewTaxonomyReader()

	// Non-existent ordinals
	if tr.IsDescendantOf(1, 2) {
		t.Error("Expected false for non-existent ordinals")
	}
}

func TestTaxonomyReaderClose(t *testing.T) {
	tr := NewTaxonomyReader()

	if err := tr.Close(); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestNewTaxonomyReaderManager(t *testing.T) {
	tr := NewTaxonomyReader()
	trm := NewTaxonomyReaderManager(tr)

	if trm == nil {
		t.Fatal("Expected TaxonomyReaderManager to be created")
	}

	acquired := trm.Acquire()
	if acquired != tr {
		t.Error("Expected acquired reader to be the same")
	}
}

func TestTaxonomyReaderManagerMaybeRefresh(t *testing.T) {
	tr := NewTaxonomyReader()
	trm := NewTaxonomyReaderManager(tr)

	if err := trm.MaybeRefresh(); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestTaxonomyReaderManagerClose(t *testing.T) {
	tr := NewTaxonomyReader()
	trm := NewTaxonomyReaderManager(tr)

	if err := trm.Close(); err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}
