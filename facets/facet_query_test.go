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
	const delim = "\x1f"
	if PathToString("dim", nil) != "dim" {
		t.Errorf("dim alone = %q", PathToString("dim", nil))
	}
	// Indexed facet terms join components with DelimChar (U+001F), byte-faithful
	// to Lucene's FacetsConfig.pathToString.
	if got := PathToString("d", []string{"a", "b"}); got != "d"+delim+"a"+delim+"b" {
		t.Errorf("hierarchical = %q", got)
	}
}

// TestPathRoundTrip verifies PathToString/StringToPath are exact inverses,
// including labels that contain the display separator '/' and the delimiter
// itself, mirroring FacetsConfig.pathToString/stringToPath escaping.
func TestPathRoundTrip(t *testing.T) {
	cases := [][]string{
		{"dim"},
		{"d", "a", "b"},
		{"category", "a/b", "c"},        // component containing '/'
		{"weird", "x\x1fy", "z"},        // component containing DelimChar
		{"esc", "p\x1eq"},               // component containing escapeChar
		{"all", "a/b\x1fc\x1ed", "e/f"}, // mix of all three
	}
	for _, full := range cases {
		dim, path := full[0], full[1:]
		encoded := PathToString(dim, path)
		got := StringToPath(encoded)
		if len(got) != len(full) {
			t.Fatalf("PathToString(%q,%v) -> %q -> %v: len mismatch", dim, path, encoded, got)
		}
		for i := range full {
			if got[i] != full[i] {
				t.Fatalf("round-trip %v -> %q -> %v: component %d = %q, want %q",
					full, encoded, got, i, got[i], full[i])
			}
		}
	}
	if len(StringToPath("")) != 0 {
		t.Errorf("StringToPath(\"\") should be empty")
	}
}
