// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestNewGroupFieldCommand(t *testing.T) {
	cmd := NewGroupFieldCommand("category")
	if cmd == nil {
		t.Fatal("Expected GroupFieldCommand to be created")
	}

	if cmd.GetField() != "category" {
		t.Errorf("Expected field 'category', got '%s'", cmd.GetField())
	}

	if cmd.GetTopN() != 10 {
		t.Errorf("Expected default topN 10, got %d", cmd.GetTopN())
	}
}

func TestGroupFieldCommandSetters(t *testing.T) {
	cmd := NewGroupFieldCommand("category")

	cmd.SetTopN(20)
	if cmd.GetTopN() != 20 {
		t.Errorf("Expected topN 20, got %d", cmd.GetTopN())
	}

	cmd.SetIncludeMaxScore(true)
	if !cmd.IsIncludeMaxScore() {
		t.Error("Expected includeMaxScore to be true")
	}
}

func TestGroupFieldCommandSetSort(t *testing.T) {
	cmd := NewGroupFieldCommand("category")

	sort := *search.NewSortByScore()
	cmd.SetSort(sort)

	// Just verify sort was set without error
	_ = cmd.GetSort()
}

func TestNewGroupFacetCommand(t *testing.T) {
	cmd := NewGroupFacetCommand("tags")
	if cmd == nil {
		t.Fatal("Expected GroupFacetCommand to be created")
	}

	if cmd.GetField() != "tags" {
		t.Errorf("Expected field 'tags', got '%s'", cmd.GetField())
	}

	if cmd.GetTopN() != 10 {
		t.Errorf("Expected default topN 10, got %d", cmd.GetTopN())
	}
}

func TestGroupFacetCommandSetters(t *testing.T) {
	cmd := NewGroupFacetCommand("tags")

	cmd.SetTopN(15)
	if cmd.GetTopN() != 15 {
		t.Errorf("Expected topN 15, got %d", cmd.GetTopN())
	}

	cmd.SetIncludeMaxScore(true)
	if !cmd.IsIncludeMaxScore() {
		t.Error("Expected includeMaxScore to be true")
	}
}

func TestGroupCommandInterface(t *testing.T) {
	// Test that GroupFieldCommand implements GroupCommand
	var _ GroupCommand = (*GroupFieldCommand)(nil)

	// Test that GroupFacetCommand implements GroupCommand
	var _ GroupCommand = (*GroupFacetCommand)(nil)
}

func TestNewTermGroupFacetCollector(t *testing.T) {
	collector := NewTermGroupFacetCollector("category", "tags")
	if collector == nil {
		t.Fatal("Expected TermGroupFacetCollector to be created")
	}

	if collector.GetGroupField() != "category" {
		t.Errorf("Expected group field 'category', got '%s'", collector.GetGroupField())
	}

	if collector.GetFacetField() != "tags" {
		t.Errorf("Expected facet field 'tags', got '%s'", collector.GetFacetField())
	}
}

func TestTermGroupFacetCollectorCollect(t *testing.T) {
	collector := NewTermGroupFacetCollector("category", "tags")

	// Collect some documents
	collector.Collect(1, 1.0, "electronics", "phone")
	collector.Collect(2, 0.8, "electronics", "laptop")
	collector.Collect(3, 0.9, "electronics", "phone")
	collector.Collect(4, 0.7, "books", "fiction")

	// Check group count
	if collector.GetGroupCount() != 2 {
		t.Errorf("Expected 2 groups, got %d", collector.GetGroupCount())
	}

	// Check total hits
	if collector.GetTotalHits() != 4 {
		t.Errorf("Expected 4 total hits, got %d", collector.GetTotalHits())
	}
}

func TestTermGroupFacetCollectorGetGroup(t *testing.T) {
	collector := NewTermGroupFacetCollector("category", "tags")

	collector.Collect(1, 1.0, "electronics", "phone")
	collector.Collect(2, 0.8, "electronics", "laptop")

	group := collector.GetGroup("electronics")
	if group == nil {
		t.Fatal("Expected to find electronics group")
	}

	if group.TotalHits != 2 {
		t.Errorf("Expected 2 hits in group, got %d", group.TotalHits)
	}
}

func TestTermGroupFacetCollectorGetFacetCounts(t *testing.T) {
	collector := NewTermGroupFacetCollector("category", "tags")

	collector.Collect(1, 1.0, "electronics", "phone")
	collector.Collect(2, 0.8, "electronics", "laptop")
	collector.Collect(3, 0.9, "electronics", "phone")

	facetCounts := collector.GetFacetCounts("electronics")
	if facetCounts == nil {
		t.Fatal("Expected facet counts")
	}

	if facetCounts["phone"] != 2 {
		t.Errorf("Expected phone count 2, got %d", facetCounts["phone"])
	}

	if facetCounts["laptop"] != 1 {
		t.Errorf("Expected laptop count 1, got %d", facetCounts["laptop"])
	}
}

func TestTermGroupFacetCollectorGetTopGroups(t *testing.T) {
	collector := NewTermGroupFacetCollector("category", "tags")

	collector.Collect(1, 1.0, "electronics", "phone")
	collector.Collect(2, 0.5, "books", "fiction")
	collector.Collect(3, 0.8, "clothing", "shirt")

	topGroups := collector.GetTopGroups(2)
	if len(topGroups) != 2 {
		t.Errorf("Expected 2 top groups, got %d", len(topGroups))
	}

	// First group should be electronics (highest score)
	if topGroups[0].GroupValue != "electronics" {
		t.Errorf("Expected first group 'electronics', got '%s'", topGroups[0].GroupValue)
	}
}

func TestTermGroupFacetCollectorReset(t *testing.T) {
	collector := NewTermGroupFacetCollector("category", "tags")

	collector.Collect(1, 1.0, "electronics", "phone")
	collector.Reset()

	if collector.GetGroupCount() != 0 {
		t.Errorf("Expected 0 groups after reset, got %d", collector.GetGroupCount())
	}

	if collector.GetTotalHits() != 0 {
		t.Errorf("Expected 0 total hits after reset, got %d", collector.GetTotalHits())
	}
}

func TestNewAbstractFirstPassGroupingCollector(t *testing.T) {
	selector := NewTermGroupSelector("category")
	sort := *search.NewSortByScore()

	collector := NewAbstractFirstPassGroupingCollector(selector, sort, 10)
	if collector == nil {
		t.Fatal("Expected AbstractFirstPassGroupingCollector to be created")
	}

	if collector.GetTotalHits() != 0 {
		t.Errorf("Expected 0 total hits initially, got %d", collector.GetTotalHits())
	}
}

func TestAbstractFirstPassGroupingCollectorCollect(t *testing.T) {
	selector := NewTermGroupSelector("category")
	sort := *search.NewSortByScore()

	collector := NewAbstractFirstPassGroupingCollector(selector, sort, 10)

	// Set up the selector to return different values
	selector.SetValue(1, "electronics")
	selector.SetValue(2, "electronics")
	selector.SetValue(3, "books")

	collector.Collect(1, 1.0)
	collector.Collect(2, 0.8)
	collector.Collect(3, 0.9)

	// Should have 2 unique groups
	if len(collector.GetCollectedGroups()) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(collector.GetCollectedGroups()))
	}

	// Should have 3 total hits
	if collector.GetTotalHits() != 3 {
		t.Errorf("Expected 3 total hits, got %d", collector.GetTotalHits())
	}
}

func TestNewAbstractSecondPassGroupingCollector(t *testing.T) {
	selector := NewTermGroupSelector("category")
	groupSort := *search.NewSortByScore()
	docSort := *search.NewSortByScore()

	collector := NewAbstractSecondPassGroupingCollector(selector, groupSort, docSort, 0, 10, 0, 3)
	if collector == nil {
		t.Fatal("Expected AbstractSecondPassGroupingCollector to be created")
	}

	if collector.GetTotalHits() != 0 {
		t.Errorf("Expected 0 total hits initially, got %d", collector.GetTotalHits())
	}
}

func TestAbstractSecondPassGroupingCollectorCollect(t *testing.T) {
	selector := NewTermGroupSelector("category")
	groupSort := *search.NewSortByScore()
	docSort := *search.NewSortByScore()

	collector := NewAbstractSecondPassGroupingCollector(selector, groupSort, docSort, 0, 10, 0, 3)

	// Set up the selector
	selector.SetValue(1, "electronics")
	selector.SetValue(2, "electronics")
	selector.SetValue(3, "books")

	collector.Collect(1, 1.0)
	collector.Collect(2, 0.8)
	collector.Collect(3, 0.9)

	// Should have 2 groups
	if collector.GetGroupCount() != 2 {
		t.Errorf("Expected 2 groups, got %d", collector.GetGroupCount())
	}

	// Electronics group should have 2 documents
	electronicsGroup := collector.GetGroup("electronics")
	if electronicsGroup == nil {
		t.Fatal("Expected to find electronics group")
	}
	if electronicsGroup.GetScoreDocCount() != 2 {
		t.Errorf("Expected 2 documents in electronics group, got %d", electronicsGroup.GetScoreDocCount())
	}
}

func TestNewAbstractAllGroupHeadsCollector(t *testing.T) {
	selector := NewTermGroupSelector("category")

	collector := NewAbstractAllGroupHeadsCollector(selector)
	if collector == nil {
		t.Fatal("Expected AbstractAllGroupHeadsCollector to be created")
	}

	if collector.GetGroupCount() != 0 {
		t.Errorf("Expected 0 groups initially, got %d", collector.GetGroupCount())
	}
}

func TestAbstractAllGroupHeadsCollectorCollect(t *testing.T) {
	selector := NewTermGroupSelector("category")

	collector := NewAbstractAllGroupHeadsCollector(selector)

	// Set up the selector
	selector.SetValue(1, "electronics")
	selector.SetValue(2, "electronics")
	selector.SetValue(3, "books")

	collector.Collect(1, 1.0)
	collector.Collect(2, 0.8) // Should not replace doc 1 as head
	collector.Collect(3, 0.9)

	// Should have 2 groups
	if collector.GetGroupCount() != 2 {
		t.Errorf("Expected 2 groups, got %d", collector.GetGroupCount())
	}

	// Check heads
	heads := collector.GetHeads()
	if heads["electronics"] == nil || heads["electronics"].Doc != 1 {
		t.Error("Expected electronics head to be doc 1")
	}
	if heads["books"] == nil || heads["books"].Doc != 3 {
		t.Error("Expected books head to be doc 3")
	}

	// Check head docs
	headDocs := collector.GetHeadDocs()
	if len(headDocs) != 2 {
		t.Errorf("Expected 2 head docs, got %d", len(headDocs))
	}
}

func TestAbstractAllGroupHeadsCollectorReset(t *testing.T) {
	selector := NewTermGroupSelector("category")

	collector := NewAbstractAllGroupHeadsCollector(selector)

	selector.SetValue(1, "electronics")
	collector.Collect(1, 1.0)

	collector.Reset()

	if collector.GetGroupCount() != 0 {
		t.Errorf("Expected 0 groups after reset, got %d", collector.GetGroupCount())
	}

	if collector.GetTotalHits() != 0 {
		t.Errorf("Expected 0 total hits after reset, got %d", collector.GetTotalHits())
	}
}

func TestNewValueSourceCommand(t *testing.T) {
	valueSource := NewDoubleValueSource("price")
	cmd := NewValueSourceCommand(valueSource)

	if cmd == nil {
		t.Fatal("Expected ValueSourceCommand to be created")
	}

	if cmd.GetValueSource() != valueSource {
		t.Error("Expected value source to match")
	}

	if cmd.GetTopN() != 10 {
		t.Errorf("Expected default topN 10, got %d", cmd.GetTopN())
	}
}

func TestValueSourceCommandSetters(t *testing.T) {
	valueSource := NewDoubleValueSource("price")
	cmd := NewValueSourceCommand(valueSource)

	cmd.SetTopN(25)
	if cmd.GetTopN() != 25 {
		t.Errorf("Expected topN 25, got %d", cmd.GetTopN())
	}

	cmd.SetIncludeMaxScore(true)
	if !cmd.IsIncludeMaxScore() {
		t.Error("Expected includeMaxScore to be true")
	}
}

func TestValueSourceCommandSetSort(t *testing.T) {
	valueSource := NewDoubleValueSource("price")
	cmd := NewValueSourceCommand(valueSource)

	sort := *search.NewSortByScore()
	cmd.SetSort(sort)

	// Just verify sort was set without error
	_ = cmd.GetSort()
}

func TestValueSourceCommandString(t *testing.T) {
	valueSource := NewDoubleValueSource("price")
	cmd := NewValueSourceCommand(valueSource)

	str := cmd.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}

	// Should contain the value source description
	if !strings.Contains(str, "price") {
		t.Errorf("Expected string to contain 'price', got '%s'", str)
	}
}

func TestValueSourceCommandImplementsGroupCommand(t *testing.T) {
	// Test that ValueSourceCommand implements GroupCommand
	var _ GroupCommand = (*ValueSourceCommand)(nil)
}
