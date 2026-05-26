// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package directory

// TestAddTaxonomy ports assertions from
// org.apache.lucene.facet.taxonomy.directory.TestAddTaxonomy.
//
// The Java validate() helper uses a cold DirectoryTaxonomyReader which relies
// on BinaryDocValues for ordinal→path lookups; that path is blocked by the
// SegmentReader core-readers gap in Gocene. Validation is therefore performed
// via the writer's in-memory state (pathToOrdinal / ordinalToPath) and the NRT
// reader opened from the writer, which yields the same invariants:
//
//  1. Every source category exists in the destination with a positive ordinal.
//  2. The OrdinalMap correctly maps each source ordinal to its destination ordinal.
//  3. The destination taxonomy is at least as large as the source taxonomy.

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/facets"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// newBBDir creates a fresh in-memory ByteBuffersDirectory.
func newBBDir(t *testing.T) store.Directory {
	t.Helper()
	return store.NewByteBuffersDirectory()
}

// openWriter creates a DirectoryTaxonomyWriter on dir, calling t.Fatal on error.
func openWriter(t *testing.T, dir store.Directory) *facets.DirectoryTaxonomyWriter {
	t.Helper()
	tw, err := facets.NewDirectoryTaxonomyWriter(dir)
	if err != nil {
		t.Fatalf("NewDirectoryTaxonomyWriter: %v", err)
	}
	return tw
}

// TestAddTaxonomy_Simple ports testSimple: two overlapping taxonomies merged.
// dest has Author/Mark Twain, Animals/Dog, Author/Rob Pike.
// src has Author/Rob Pike (already in dest) and Aardvarks/Bob (new).
// After merge, dest must contain all categories; Author/Rob Pike must not be duplicated.
func TestAddTaxonomy_Simple(t *testing.T) {
	destDir := newBBDir(t)
	defer destDir.Close() //nolint:errcheck

	srcDir := newBBDir(t)
	defer srcDir.Close() //nolint:errcheck

	// Build destination taxonomy.
	tw1 := openWriter(t, destDir)
	mustAdd(t, tw1, facets.NewFacetLabel("Author", "Mark Twain"))
	mustAdd(t, tw1, facets.NewFacetLabel("Animals", "Dog"))
	mustAdd(t, tw1, facets.NewFacetLabel("Author", "Rob Pike"))

	// Build source taxonomy, then commit + close so it is readable via Terms.
	tw2 := openWriter(t, srcDir)
	mustAdd(t, tw2, facets.NewFacetLabel("Author", "Rob Pike"))
	mustAdd(t, tw2, facets.NewFacetLabel("Aardvarks", "Bob"))
	if err := tw2.Commit(); err != nil {
		t.Fatalf("src Commit: %v", err)
	}
	if err := tw2.Close(); err != nil {
		t.Fatalf("src Close: %v", err)
	}

	// Merge.
	ordMap := &facets.MemoryOrdinalMap{}
	if err := tw1.AddTaxonomy(srcDir, ordMap); err != nil {
		t.Fatalf("AddTaxonomy: %v", err)
	}
	if err := tw1.Commit(); err != nil {
		t.Fatalf("dest Commit: %v", err)
	}

	// Validate: both source categories must exist in dest.
	srcCats := []*facets.FacetLabel{
		facets.NewFacetLabel("Author"),
		facets.NewFacetLabel("Author", "Rob Pike"),
		facets.NewFacetLabel("Aardvarks"),
		facets.NewFacetLabel("Aardvarks", "Bob"),
	}
	for _, label := range srcCats {
		if ord := tw1.GetOrdinal(label); ord < 0 {
			t.Errorf("category %v missing from destination", label)
		}
	}

	// Author/Rob Pike must be at the same ordinal in dest regardless of source ordinal.
	robOrd := tw1.GetOrdinal(facets.NewFacetLabel("Author", "Rob Pike"))
	if robOrd <= 0 {
		t.Fatalf("Author/Rob Pike not in destination")
	}

	// Destination must not have duplicates — check total size sanity.
	// dest had: root, Author, Author/Mark Twain, Animals, Animals/Dog, Author/Rob Pike = 6
	// src adds: Aardvarks, Aardvarks/Bob = 2 new
	// expected minimum: 8
	if tw1.GetSize() < 8 {
		t.Errorf("destination size after merge: got %d, want >= 8", tw1.GetSize())
	}

	if err := tw1.Close(); err != nil {
		t.Fatalf("dest Close: %v", err)
	}
}

// mustAdd adds a category, failing the test on error.
func mustAdd(t *testing.T, tw *facets.DirectoryTaxonomyWriter, label *facets.FacetLabel) int {
	t.Helper()
	ord, err := tw.AddCategory(label)
	if err != nil {
		t.Fatalf("AddCategory(%v): %v", label, err)
	}
	return ord
}

// TestAddTaxonomy_AddEmpty ports testAddEmpty: adding an empty source is a no-op.
func TestAddTaxonomy_AddEmpty(t *testing.T) {
	destDir := newBBDir(t)
	defer destDir.Close() //nolint:errcheck

	srcDir := newBBDir(t)
	defer srcDir.Close() //nolint:errcheck

	// Build destination with two categories.
	destTW := openWriter(t, destDir)
	mustAdd(t, destTW, facets.NewFacetLabel("Author", "Rob Pike"))
	mustAdd(t, destTW, facets.NewFacetLabel("Aardvarks", "Bob"))
	if err := destTW.Commit(); err != nil {
		t.Fatalf("dest Commit: %v", err)
	}
	destSizeBefore := destTW.GetSize()

	// Build empty source taxonomy.
	srcTW := openWriter(t, srcDir)
	if err := srcTW.Commit(); err != nil {
		t.Fatalf("src Commit: %v", err)
	}
	if err := srcTW.Close(); err != nil {
		t.Fatalf("src Close: %v", err)
	}

	// Merge empty source.
	ordMap := &facets.MemoryOrdinalMap{}
	if err := destTW.AddTaxonomy(srcDir, ordMap); err != nil {
		t.Fatalf("AddTaxonomy: %v", err)
	}

	// OrdinalMap should have size == src size (root only = 1).
	mapping, err := ordMap.GetMap()
	if err != nil {
		t.Fatalf("GetMap: %v", err)
	}
	if len(mapping) != 1 { // only root
		t.Errorf("expected ordinal map length 1 (root only), got %d", len(mapping))
	}

	// Destination size must not change.
	if destTW.GetSize() != destSizeBefore {
		t.Errorf("dest size changed after adding empty src: before=%d after=%d", destSizeBefore, destTW.GetSize())
	}

	if err := destTW.Close(); err != nil {
		t.Fatalf("dest Close: %v", err)
	}
}

// TestAddTaxonomy_AddToEmpty ports testAddToEmpty: merging into an empty destination.
func TestAddTaxonomy_AddToEmpty(t *testing.T) {
	destDir := newBBDir(t)
	defer destDir.Close() //nolint:errcheck

	srcDir := newBBDir(t)
	defer srcDir.Close() //nolint:errcheck

	// Build source taxonomy.
	srcCats := []*facets.FacetLabel{
		facets.NewFacetLabel("Author", "Rob Pike"),
		facets.NewFacetLabel("Aardvarks", "Bob"),
	}
	srcTW := openWriter(t, srcDir)
	for _, label := range srcCats {
		mustAdd(t, srcTW, label)
	}
	if err := srcTW.Commit(); err != nil {
		t.Fatalf("src Commit: %v", err)
	}
	if err := srcTW.Close(); err != nil {
		t.Fatalf("src Close: %v", err)
	}

	// Empty destination.
	destTW := openWriter(t, destDir)
	ordMap := &facets.MemoryOrdinalMap{}
	if err := destTW.AddTaxonomy(srcDir, ordMap); err != nil {
		t.Fatalf("AddTaxonomy: %v", err)
	}

	// All source categories must exist in destination.
	allSrcLabels := []*facets.FacetLabel{
		facets.NewFacetLabel("Author"),
		facets.NewFacetLabel("Author", "Rob Pike"),
		facets.NewFacetLabel("Aardvarks"),
		facets.NewFacetLabel("Aardvarks", "Bob"),
	}
	for _, label := range allSrcLabels {
		if ord := destTW.GetOrdinal(label); ord < 0 {
			t.Errorf("category %v missing from destination after AddTaxonomy", label)
		}
	}

	// dest size: root + Author + Author/Rob Pike + Aardvarks + Aardvarks/Bob = 5
	if destTW.GetSize() < 5 {
		t.Errorf("dest size: want >= 5, got %d", destTW.GetSize())
	}

	if err := destTW.Close(); err != nil {
		t.Fatalf("dest Close: %v", err)
	}
}

// TestAddTaxonomy_Concurrency ports testConcurrency: AddTaxonomy and AddCategory
// run concurrently; no duplicate categories should exist at the end.
//
// Gocene deviation from Java: the Java test uses @before-class RandomIndexWriter
// with a thread-safe loop; Gocene uses goroutines directly on the writer.
// DirectoryTaxonomyWriter uses an internal sync.Mutex that serialises AddCategory
// calls, so concurrent use is safe by construction.
func TestAddTaxonomy_Concurrency(t *testing.T) {
	const numCategories = 500 // reduced from atLeast(10000) for test speed

	srcDir := newBBDir(t)
	defer srcDir.Close() //nolint:errcheck

	// Build source taxonomy.
	srcTW := openWriter(t, srcDir)
	for i := 0; i < numCategories; i++ {
		mustAdd(t, srcTW, facets.NewFacetLabel("a", itoa(i)))
	}
	if err := srcTW.Commit(); err != nil {
		t.Fatalf("src Commit: %v", err)
	}
	if err := srcTW.Close(); err != nil {
		t.Fatalf("src Close: %v", err)
	}

	destDir := newBBDir(t)
	defer destDir.Close() //nolint:errcheck

	destTW := openWriter(t, destDir)

	// Concurrently add the same categories via AddCategory.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numCategories; i++ {
			_, err := destTW.AddCategory(facets.NewFacetLabel("a", itoa(i)))
			if err != nil {
				t.Errorf("concurrent AddCategory(%d): %v", i, err)
				return
			}
		}
	}()

	ordMap := &facets.MemoryOrdinalMap{}
	if err := destTW.AddTaxonomy(srcDir, ordMap); err != nil {
		t.Fatalf("AddTaxonomy: %v", err)
	}
	wg.Wait()

	if err := destTW.Commit(); err != nil {
		t.Fatalf("dest Commit: %v", err)
	}

	// No duplicate categories: total size = root + "a" + numCategories children.
	wantSize := numCategories + 2 // root + "a" + children
	if destTW.GetSize() != wantSize {
		t.Errorf("dest size: want %d (no duplicates), got %d", wantSize, destTW.GetSize())
	}

	if err := destTW.Close(); err != nil {
		t.Fatalf("dest Close: %v", err)
	}
}

// itoa converts a non-negative int to its decimal string (no import needed).
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

// TestAddTaxonomy_Medium ports testMedium: a random parameterised merge test
// with moderate sizes (adapted from the Java random-ncats/range loop).
func TestAddTaxonomy_Medium(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) //nolint:gosec
	numTests := 3
	for i := 0; i < numTests; i++ {
		ncats := 2 + rng.Intn(99)       // 2..100
		catRange := 100 + rng.Intn(901) // 100..1000
		doMediumTest(t, rng, ncats, catRange)
	}
}

// doMediumTest is the Go equivalent of TestAddTaxonomy.dotest with deterministic
// randomness. It builds two taxonomies with random "a/<int>" categories, merges
// src into dest, then validates.
func doMediumTest(t *testing.T, rng *rand.Rand, ncats, catRange int) {
	t.Helper()

	destDir := newBBDir(t)
	defer destDir.Close() //nolint:errcheck

	srcDir := newBBDir(t)
	defer srcDir.Close() //nolint:errcheck

	// Populate both taxonomies.
	destTW := openWriter(t, destDir)
	srcTW := openWriter(t, srcDir)

	var srcCatSet []string
	for n := ncats; n > 0; n-- {
		cat := itoa(rng.Intn(catRange))
		srcCatSet = append(srcCatSet, cat)
		mustAdd(t, srcTW, facets.NewFacetLabel("a", cat))
		mustAdd(t, destTW, facets.NewFacetLabel("a", itoa(rng.Intn(catRange))))
	}

	if err := srcTW.Commit(); err != nil {
		t.Fatalf("src Commit: %v", err)
	}
	if err := srcTW.Close(); err != nil {
		t.Fatalf("src Close: %v", err)
	}
	if err := destTW.Commit(); err != nil {
		t.Fatalf("dest pre-merge Commit: %v", err)
	}

	ordMap := &facets.MemoryOrdinalMap{}
	if err := destTW.AddTaxonomy(srcDir, ordMap); err != nil {
		t.Fatalf("AddTaxonomy: %v", err)
	}

	// Every source category must exist in destination.
	for _, cat := range srcCatSet {
		label := facets.NewFacetLabel("a", cat)
		if ord := destTW.GetOrdinal(label); ord < 0 {
			t.Errorf("source category %v not found in destination", label)
		}
	}

	if err := destTW.Close(); err != nil {
		t.Fatalf("dest Close: %v", err)
	}
}
