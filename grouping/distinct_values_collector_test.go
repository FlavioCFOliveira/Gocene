package grouping

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestNewDistinctValuesCollector(t *testing.T) {
	groupSelector := NewTermGroupSelector("category")
	valueSelector := NewTermGroupSelector("tag")
	sort := *search.NewSortByScore()

	collector := NewDistinctValuesCollector(groupSelector, valueSelector, sort, 10, 5)

	if collector == nil {
		t.Fatal("Expected DistinctValuesCollector to be created")
	}

	if collector.groupSelector != groupSelector {
		t.Error("Expected group selector to match")
	}

	if collector.valueSelector != valueSelector {
		t.Error("Expected value selector to match")
	}

	if collector.topN != 10 {
		t.Errorf("Expected topN 10, got %d", collector.topN)
	}

	if collector.maxValuesPerGroup != 5 {
		t.Errorf("Expected maxValuesPerGroup 5, got %d", collector.maxValuesPerGroup)
	}
}

func TestDistinctValuesCollectorCollect(t *testing.T) {
	groupSelector := NewTermGroupSelector("category")
	valueSelector := NewTermGroupSelector("tag")
	sort := *search.NewSortByScore()

	collector := NewDistinctValuesCollector(groupSelector, valueSelector, sort, 10, 0)

	// Set up selectors
	groupSelector.SetValue(1, "electronics")
	valueSelector.SetValue(1, "phone")

	groupSelector.SetValue(2, "electronics")
	valueSelector.SetValue(2, "laptop")

	groupSelector.SetValue(3, "electronics")
	valueSelector.SetValue(3, "phone") // Duplicate value

	groupSelector.SetValue(4, "books")
	valueSelector.SetValue(4, "fiction")

	// Collect documents
	collector.Collect(1, 1.0)
	collector.Collect(2, 0.8)
	collector.Collect(3, 0.9)
	collector.Collect(4, 0.7)

	// Check total hits
	if collector.GetTotalHits() != 4 {
		t.Errorf("Expected 4 total hits, got %d", collector.GetTotalHits())
	}

	// Check group count
	if collector.GetGroupCount() != 2 {
		t.Errorf("Expected 2 groups, got %d", collector.GetGroupCount())
	}

	// Check distinct values for electronics
	electronicsCount := collector.GetDistinctValueCount("electronics")
	if electronicsCount != 2 {
		t.Errorf("Expected 2 distinct values for electronics, got %d", electronicsCount)
	}

	// Check distinct values for books
	booksCount := collector.GetDistinctValueCount("books")
	if booksCount != 1 {
		t.Errorf("Expected 1 distinct value for books, got %d", booksCount)
	}
}

func TestDistinctValuesCollectorGetGroupDistinctValues(t *testing.T) {
	groupSelector := NewTermGroupSelector("category")
	valueSelector := NewTermGroupSelector("tag")
	sort := *search.NewSortByScore()

	collector := NewDistinctValuesCollector(groupSelector, valueSelector, sort, 10, 0)

	// Set up selectors
	groupSelector.SetValue(1, "electronics")
	valueSelector.SetValue(1, "phone")

	groupSelector.SetValue(2, "electronics")
	valueSelector.SetValue(2, "laptop")

	// Collect documents
	collector.Collect(1, 1.0)
	collector.Collect(2, 0.8)

	// Get group distinct values
	groupValues := collector.GetGroupDistinctValues("electronics")
	if groupValues == nil {
		t.Fatal("Expected to find electronics group")
	}

	if len(groupValues.OrderedValues) != 2 {
		t.Errorf("Expected 2 ordered values, got %d", len(groupValues.OrderedValues))
	}

	// Check non-existent group
	nonExistent := collector.GetGroupDistinctValues("nonexistent")
	if nonExistent != nil {
		t.Error("Expected nil for non-existent group")
	}
}

func TestDistinctValuesCollectorMaxValuesPerGroup(t *testing.T) {
	groupSelector := NewTermGroupSelector("category")
	valueSelector := NewTermGroupSelector("tag")
	sort := *search.NewSortByScore()

	// Limit to 2 distinct values per group
	collector := NewDistinctValuesCollector(groupSelector, valueSelector, sort, 10, 2)

	// Set up selectors with 3 different values
	groupSelector.SetValue(1, "electronics")
	valueSelector.SetValue(1, "phone")

	groupSelector.SetValue(2, "electronics")
	valueSelector.SetValue(2, "laptop")

	groupSelector.SetValue(3, "electronics")
	valueSelector.SetValue(3, "tablet")

	// Collect documents
	collector.Collect(1, 1.0)
	collector.Collect(2, 0.8)
	collector.Collect(3, 0.9)

	// Should only have 2 distinct values due to limit
	electronicsCount := collector.GetDistinctValueCount("electronics")
	if electronicsCount != 2 {
		t.Errorf("Expected 2 distinct values (max limit), got %d", electronicsCount)
	}
}

func TestDistinctValuesCollectorGetTopGroups(t *testing.T) {
	groupSelector := NewTermGroupSelector("category")
	valueSelector := NewTermGroupSelector("tag")
	sort := *search.NewSortByScore()

	collector := NewDistinctValuesCollector(groupSelector, valueSelector, sort, 10, 0)

	// Set up selectors
	groupSelector.SetValue(1, "electronics")
	valueSelector.SetValue(1, "phone")

	groupSelector.SetValue(2, "books")
	valueSelector.SetValue(2, "fiction")

	// Collect documents
	collector.Collect(1, 1.0)
	collector.Collect(2, 0.8)

	// Get top groups
	topGroups := collector.GetTopGroups()
	if len(topGroups) != 2 {
		t.Errorf("Expected 2 top groups, got %d", len(topGroups))
	}
}

func TestDistinctValuesCollectorGetTotalDistinctValues(t *testing.T) {
	groupSelector := NewTermGroupSelector("category")
	valueSelector := NewTermGroupSelector("tag")
	sort := *search.NewSortByScore()

	collector := NewDistinctValuesCollector(groupSelector, valueSelector, sort, 10, 0)

	// Set up selectors
	groupSelector.SetValue(1, "electronics")
	valueSelector.SetValue(1, "phone")

	groupSelector.SetValue(2, "electronics")
	valueSelector.SetValue(2, "laptop")

	groupSelector.SetValue(3, "books")
	valueSelector.SetValue(3, "fiction")

	// Collect documents
	collector.Collect(1, 1.0)
	collector.Collect(2, 0.8)
	collector.Collect(3, 0.9)

	// Total distinct values should be 3 (phone, laptop, fiction)
	totalDistinct := collector.GetTotalDistinctValues()
	if totalDistinct != 3 {
		t.Errorf("Expected 3 total distinct values, got %d", totalDistinct)
	}
}

func TestDistinctValuesCollectorReset(t *testing.T) {
	groupSelector := NewTermGroupSelector("category")
	valueSelector := NewTermGroupSelector("tag")
	sort := *search.NewSortByScore()

	collector := NewDistinctValuesCollector(groupSelector, valueSelector, sort, 10, 0)

	// Set up selectors
	groupSelector.SetValue(1, "electronics")
	valueSelector.SetValue(1, "phone")

	// Collect document
	collector.Collect(1, 1.0)

	if collector.GetTotalHits() != 1 {
		t.Errorf("Expected 1 hit before reset, got %d", collector.GetTotalHits())
	}

	// Reset
	collector.Reset()

	if collector.GetTotalHits() != 0 {
		t.Errorf("Expected 0 hits after reset, got %d", collector.GetTotalHits())
	}

	if collector.GetGroupCount() != 0 {
		t.Errorf("Expected 0 groups after reset, got %d", collector.GetGroupCount())
	}
}

func TestDistinctValuesCollectorString(t *testing.T) {
	groupSelector := NewTermGroupSelector("category")
	valueSelector := NewTermGroupSelector("tag")
	sort := *search.NewSortByScore()

	collector := NewDistinctValuesCollector(groupSelector, valueSelector, sort, 10, 0)

	// Set up selectors
	groupSelector.SetValue(1, "electronics")
	valueSelector.SetValue(1, "phone")

	collector.Collect(1, 1.0)

	str := collector.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}

	// Should contain expected fields
	if !strings.Contains(str, "DistinctValuesCollector") {
		t.Errorf("Expected string to contain 'DistinctValuesCollector', got '%s'", str)
	}
}

func TestDistinctValuesCollectorNilGroupValue(t *testing.T) {
	groupSelector := NewTermGroupSelector("category")
	valueSelector := NewTermGroupSelector("tag")
	sort := *search.NewSortByScore()

	collector := NewDistinctValuesCollector(groupSelector, valueSelector, sort, 10, 0)

	// Don't set any values - selectors will return nil
	err := collector.Collect(1, 1.0)
	if err != nil {
		t.Errorf("Expected no error for nil group value, got %v", err)
	}

	// Should have 0 hits since group value was nil
	if collector.GetTotalHits() != 0 {
		t.Errorf("Expected 0 hits (nil group), got %d", collector.GetTotalHits())
	}
}

func TestGroupDistinctValuesStructure(t *testing.T) {
	groupValues := &GroupDistinctValues{
		GroupValue:     "electronics",
		DistinctValues: make(map[interface{}]bool),
		OrderedValues:  make([]interface{}, 0),
		Score:          1.0,
	}

	groupValues.DistinctValues["phone"] = true
	groupValues.OrderedValues = append(groupValues.OrderedValues, "phone")

	if groupValues.GroupValue != "electronics" {
		t.Errorf("Expected group value 'electronics', got %v", groupValues.GroupValue)
	}

	if len(groupValues.OrderedValues) != 1 {
		t.Errorf("Expected 1 ordered value, got %d", len(groupValues.OrderedValues))
	}
}
