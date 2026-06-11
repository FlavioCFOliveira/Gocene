// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy_test

// TestTaxonomyFacetCounts2 ports the test assertions from
// org.apache.lucene.facet.taxonomy.TestTaxonomyFacetCounts2.
//
// The Java class is a class-level setup test that builds a multi-segment
// index with taxonomy categories, then queries it with FacetsCollector +
// FastTaxonomyFacetCounts. The full integration path is now wired.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/facets/taxonomy"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	cpA = "A"
	cpB = "B"
	cpC = "C"
	cpD = "D"

	numChildrenCpA = 5
	numChildrenCpB = 3
	numChildrenCpC = 5
	numChildrenCpD = 5
)

// categoriesA through categoriesD mirror the Java static initializers.
var (
	categoriesA []*facets.FacetField
	categoriesB []*facets.FacetField
	categoriesC []*facets.FacetField
	categoriesD []*facets.FacetField
)

func init() {
	categoriesA = make([]*facets.FacetField, numChildrenCpA)
	for i := 0; i < numChildrenCpA; i++ {
		categoriesA[i] = facets.NewFacetField(cpA, itoa(i))
	}
	categoriesB = make([]*facets.FacetField, numChildrenCpB)
	for i := 0; i < numChildrenCpB; i++ {
		categoriesB[i] = facets.NewFacetField(cpB, itoa(i))
	}
	// NO_PARENTS categories
	categoriesC = make([]*facets.FacetField, numChildrenCpC)
	for i := 0; i < numChildrenCpC; i++ {
		categoriesC[i] = facets.NewFacetField(cpC, itoa(i))
	}
	// Multi-level categories
	categoriesD = make([]*facets.FacetField, numChildrenCpD)
	for i := 0; i < numChildrenCpD; i++ {
		val := itoa(i)
		categoriesD[i] = facets.NewFacetFieldWithPath(cpD, []string{val}, val+val)
	}
}

// facetCounts2Config returns the FacetsConfig used by the Java test class.
func facetCounts2Config() *facets.FacetsConfig {
	cfg := facets.NewFacetsConfig()
	cfg.SetMultiValued(cpA, true)
	cfg.SetMultiValued(cpB, true)
	cfg.SetRequireDimCount(cpB, true)
	cfg.SetHierarchical(cpD, true)
	return cfg
}

// TestTaxonomyFacetCounts2_ConfigSetup verifies that the FacetsConfig settings
// mirror the Java test fixture: A multi-valued, B multi-valued+requireDimCount,
// D hierarchical.
func TestTaxonomyFacetCounts2_ConfigSetup(t *testing.T) {
	cfg := facetCounts2Config()

	if !cfg.GetDimConfig(cpA).MultiValued {
		t.Errorf("expected dim A to be multi-valued")
	}
	if !cfg.GetDimConfig(cpB).MultiValued {
		t.Errorf("expected dim B to be multi-valued")
	}
	if !cfg.GetDimConfig(cpB).RequireDimCount {
		t.Errorf("expected dim B to require dim count")
	}
	if !cfg.GetDimConfig(cpD).Hierarchical {
		t.Errorf("expected dim D to be hierarchical")
	}
	// C and A should NOT require dim count by default
	if cfg.GetDimConfig(cpA).RequireDimCount {
		t.Errorf("expected dim A NOT to require dim count")
	}
}

// TestTaxonomyFacetCounts2_CategoryKeys verifies category key construction
// used in the Java test's expected-count maps (dim+"/"+child format).
func TestTaxonomyFacetCounts2_CategoryKeys(t *testing.T) {
	wantKeys := make([]string, 0)
	for i := 0; i < numChildrenCpA; i++ {
		wantKeys = append(wantKeys, cpA+"/"+itoa(i))
	}
	for i := 0; i < numChildrenCpB; i++ {
		wantKeys = append(wantKeys, cpB+"/"+itoa(i))
	}
	// verify no duplicates
	seen := make(map[string]struct{}, len(wantKeys))
	for _, k := range wantKeys {
		if _, dup := seen[k]; dup {
			t.Errorf("duplicate category key: %s", k)
		}
		seen[k] = struct{}{}
	}
	if len(seen) != numChildrenCpA+numChildrenCpB {
		t.Errorf("expected %d unique keys, got %d", numChildrenCpA+numChildrenCpB, len(seen))
	}
}

// itoa converts a non-negative int to its decimal string without importing fmt.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

// setupIndexForCounts2 builds a fixed index with categories A/B/C/D as in the
// Java test fixture and returns the directory, taxonomy reader, and config.
func setupIndexForCounts2(t *testing.T) (store.Directory, taxonomy.TaxonomyReaderI, *facets.FacetsConfig) {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	taxoDir := store.NewByteBuffersDirectory()

	taxoWriter, err := facets.NewDirectoryTaxonomyWriter(taxoDir)
	if err != nil {
		t.Fatalf("creating taxonomy writer: %v", err)
	}

	config := facetCounts2Config()

	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("creating index writer: %v", err)
	}

	// Index 20 documents with random categories from the pools, plus a string
	// field "f" with value "a" to allow filtered queries.
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()

		// Add the identifying field.
		fField, _ := document.NewStringField("f", "a", true)
		doc.Add(fField)

		// Build facet fields: pick one random from each category pool.
		var fields []*facets.FacetField
		// A: always add first category (deterministic)
		fields = append(fields, categoriesA[i%numChildrenCpA])
		// B: always add first category
		fields = append(fields, categoriesB[i%numChildrenCpB])
		// C: always add first category (NO_PARENTS)
		fields = append(fields, categoriesC[i%numChildrenCpC])
		// D: always add first category (multi-level)
		val := itoa(i % numChildrenCpD)
		fields = append(fields, facets.NewFacetFieldWithPath(cpD, []string{val}, val+val))

		builtDoc, err := config.BuildWithTaxonomy(taxoWriter, doc, fields...)
		if err != nil {
			t.Fatalf("BuildWithTaxonomy: %v", err)
		}
		if err := writer.AddDocument(builtDoc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("writer commit: %v", err)
	}
	if err := taxoWriter.Commit(); err != nil {
		t.Fatalf("taxonomy commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}

	taxoReader, err := facets.NewDirectoryTaxonomyReaderFromWriter(taxoWriter)
	if err != nil {
		t.Fatalf("taxonomy reader: %v", err)
	}

	adapter := taxonomy.NewDirectoryTaxonomyReaderAdapter(taxoReader)
	t.Cleanup(func() {
		reader.Close()
	})

	return dir, adapter, config
}

// Helper: search and accumulate.
func searchAndAccumulate(t *testing.T, dir store.Directory, taxoReader taxonomy.TaxonomyReaderI, config *facets.FacetsConfig, query search.Query) *taxonomy.FastTaxonomyFacetCounts {
	t.Helper()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}
	t.Cleanup(func() { reader.Close() })

	searcher := search.NewIndexSearcher(reader)
	fc := facets.NewFacetsCollector()
	if err := searcher.SearchWithCollector(query, fc); err != nil {
		t.Fatalf("search: %v", err)
	}
	if err := fc.Finish(); err != nil {
		t.Fatalf("finish: %v", err)
	}

	ftfc := taxonomy.NewFastTaxonomyFacetCounts("$facets", taxoReader, config)
	if err := ftfc.Accumulate(fc.GetMatchingDocs()); err != nil {
		t.Fatalf("accumulate: %v", err)
	}
	return ftfc
}

func TestTaxonomyFacetCounts2_DifferentNumResults(t *testing.T) {
	dir, taxoReader, config := setupIndexForCounts2(t)
	ftfc := searchAndAccumulate(t, dir, taxoReader, config, search.NewMatchAllDocsQuery())

	// Verify with different topN values (should return fewer if there are fewer
	// children).
	result, err := ftfc.GetTopChildren(2, cpA)
	if err != nil {
		t.Fatalf("GetTopChildren A top2: %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
	if len(result.LabelValues) > 2 {
		t.Errorf("expected at most 2 label values, got %d", len(result.LabelValues))
	}

	// topN = 0 should return error.
	_, err = ftfc.GetTopChildren(0, cpA)
	if err == nil {
		t.Error("expected error for topN=0")
	}
}

func TestTaxonomyFacetCounts2_AllCounts(t *testing.T) {
	dir, taxoReader, config := setupIndexForCounts2(t)
	ftfc := searchAndAccumulate(t, dir, taxoReader, config, search.NewMatchAllDocsQuery())

	// Get all children for each dimension and verify total counts.
	result, err := ftfc.GetTopChildren(10, cpA)
	if err != nil {
		t.Fatalf("GetTopChildren A: %v", err)
	}
	if result == nil {
		t.Fatal("nil result for A")
	}
	var sumA int64
	for _, lv := range result.LabelValues {
		sumA += lv.Value
	}
	if sumA != 20 {
		t.Errorf("A total: want 20, got %d", sumA)
	}

	result, err = ftfc.GetTopChildren(10, cpB)
	if err != nil {
		t.Fatalf("GetTopChildren B: %v", err)
	}
	if result == nil {
		t.Fatal("nil result for B")
	}
	var sumB int64
	for _, lv := range result.LabelValues {
		sumB += lv.Value
	}
	// B has 3 children, each appears ceil(20/3) times = 7, 7, 6 = 20
	if sumB != 20 {
		t.Errorf("B total: want 20, got %d", sumB)
	}
}

func TestTaxonomyFacetCounts2_BigNumResults(t *testing.T) {
	dir, taxoReader, config := setupIndexForCounts2(t)
	ftfc := searchAndAccumulate(t, dir, taxoReader, config, search.NewMatchAllDocsQuery())

	// With large topN, expect all children returned.
	result, err := ftfc.GetTopChildren(100, cpA)
	if err != nil {
		t.Fatalf("GetTopChildren A: %v", err)
	}
	if result == nil {
		t.Fatal("nil result for A")
	}
	if len(result.LabelValues) != numChildrenCpA {
		t.Errorf("A children: want %d, got %d", numChildrenCpA, len(result.LabelValues))
	}
}

func TestTaxonomyFacetCounts2_NoParents(t *testing.T) {
	dir, taxoReader, config := setupIndexForCounts2(t)
	ftfc := searchAndAccumulate(t, dir, taxoReader, config, search.NewMatchAllDocsQuery())

	// C is NO_PARENTS - should still be countable.
	result, err := ftfc.GetTopChildren(10, cpC)
	if err != nil {
		t.Fatalf("GetTopChildren C: %v", err)
	}
	if result == nil {
		t.Fatal("nil result for C")
	}
	var sumC int64
	for _, lv := range result.LabelValues {
		sumC += lv.Value
	}
	if sumC != 20 {
		t.Errorf("C total: want 20, got %d", sumC)
	}
}
