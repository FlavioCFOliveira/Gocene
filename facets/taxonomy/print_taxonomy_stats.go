package taxonomy

import (
	"fmt"
	"io"
)

// PrintTaxonomyStats writes a debug summary of a taxonomy to w. Mirrors
// org.apache.lucene.facet.taxonomy.PrintTaxonomyStats.
//
// The Go port operates on the lightweight ParallelTaxonomyArrays view so the
// caller does not need a full DirectoryTaxonomyReader.
func PrintTaxonomyStats(w io.Writer, name string, arrays ParallelTaxonomyArrays) {
	fmt.Fprintf(w, "TAXONOMY STATS (%s)\n", name)
	parents := arrays.Parents()
	children := arrays.Children()
	siblings := arrays.Siblings()
	fmt.Fprintf(w, "  size: %d ordinal(s)\n", len(parents))
	leaves := 0
	for _, c := range children {
		if c < 0 {
			leaves++
		}
	}
	fmt.Fprintf(w, "  leaves: %d\n", leaves)
	roots := 0
	for _, p := range parents {
		if p < 0 {
			roots++
		}
	}
	fmt.Fprintf(w, "  roots: %d\n", roots)
	fmt.Fprintf(w, "  siblings recorded: %d\n", countNonNegative(siblings))
}

func countNonNegative(xs []int) int {
	c := 0
	for _, x := range xs {
		if x >= 0 {
			c++
		}
	}
	return c
}
