package taxonomy

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintTaxonomyStats(t *testing.T) {
	arrays := NewInMemoryParallelTaxonomyArrays(
		[]int{-1, 0, 0},
		[]int{1, -1, -1},
		[]int{-1, 2, -1},
	)
	var buf bytes.Buffer
	PrintTaxonomyStats(&buf, "demo", arrays)
	out := buf.String()
	if !strings.Contains(out, "TAXONOMY STATS (demo)") {
		t.Errorf("header missing: %q", out)
	}
	if !strings.Contains(out, "size: 3 ordinal(s)") {
		t.Errorf("size: %q", out)
	}
	if !strings.Contains(out, "leaves: 2") {
		t.Errorf("leaves: %q", out)
	}
	if !strings.Contains(out, "roots: 1") {
		t.Errorf("roots: %q", out)
	}
}
