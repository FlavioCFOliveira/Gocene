package facets

import "testing"

func TestFacetQueryBasic(t *testing.T) {
	q := NewFacetQuery(nil, "color", "red")
	if q.GetDim() != "color" {
		t.Errorf("dim = %q", q.GetDim())
	}
	path := q.GetPath()
	if len(path) != 1 || path[0] != "red" {
		t.Errorf("path = %v", path)
	}
}

func TestFacetQueryHierarchical(t *testing.T) {
	q := NewFacetQuery(nil, "tag", "tech", "ai", "ml")
	if len(q.GetPath()) != 3 {
		t.Errorf("path len = %d", len(q.GetPath()))
	}
}

func TestFacetQueryDrillDownField(t *testing.T) {
	if DrillDownFieldName(nil, "anyDim") != "$facets" {
		t.Error("nil config should give default")
	}
	cfg := NewFacetsConfig()
	cfg.SetIndexFieldName("color", "color_idx")
	if DrillDownFieldName(cfg, "color") != "color_idx" {
		t.Errorf("expected custom index field")
	}
}

func TestPathToString(t *testing.T) {
	if PathToString("dim", nil) != "dim" {
		t.Error("dim alone")
	}
	if PathToString("d", []string{"a", "b"}) != "d/a/b" {
		t.Error("hierarchical")
	}
}
