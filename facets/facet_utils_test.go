package facets

import "testing"

func TestLabelToOrd(t *testing.T) {
	labels := []string{"a", "b", "c"}
	if LabelToOrd(labels, "b") != 1 {
		t.Error("b")
	}
	if LabelToOrd(labels, "x") != -1 {
		t.Error("missing")
	}
}

func TestJoinSplitPath(t *testing.T) {
	if JoinPath("d") != "d" {
		t.Error("dim only")
	}
	if JoinPath("d", "a", "b") != "d/a/b" {
		t.Error("hierarchical")
	}
	dim, p := SplitPath("d/a/b")
	if dim != "d" || len(p) != 2 || p[0] != "a" || p[1] != "b" {
		t.Errorf("split: %q %v", dim, p)
	}
}

func TestCompareLabel(t *testing.T) {
	if CompareLabel("a", "b") >= 0 {
		t.Error("a<b")
	}
}
