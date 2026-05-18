package taxonomy

// TaxonomyMergeUtils contains the helpers used by the directory taxonomy
// writer to merge two taxonomies into one. Mirrors
// org.apache.lucene.facet.taxonomy.TaxonomyMergeUtils.

// MergeOrdinalMap merges srcParents into dstParents and returns an array that
// maps each src ordinal to its location in the destination. The mapping is
// computed by walking src in parent-first order so dependencies are present
// when each ordinal is added.
//
// The implementation is intentionally minimal: it covers the merge of
// in-memory ParallelTaxonomyArrays, the directory-backed case is handled in
// the directory subpackage. dstAppendOrd is invoked once per new ordinal so
// the caller can mirror the addition into its own storage.
func MergeOrdinalMap(srcParents []int, dstParents *[]int, dstAppendOrd func(parent int) int) []int {
	ordMap := make([]int, len(srcParents))
	for i := range ordMap {
		ordMap[i] = -1
	}
	for srcOrd := 0; srcOrd < len(srcParents); srcOrd++ {
		parent := srcParents[srcOrd]
		var dstParent int
		if parent < 0 {
			dstParent = -1
		} else {
			dstParent = ordMap[parent]
		}
		ordMap[srcOrd] = dstAppendOrd(dstParent)
		*dstParents = append(*dstParents, dstParent)
	}
	return ordMap
}
