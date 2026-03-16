package grouping

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestNewGroupingSearch(t *testing.T) {
	gs := NewGroupingSearch("category")

	if gs == nil {
		t.Fatal("Expected GroupingSearch to be created")
	}

	if gs.GetGroupField() != "category" {
		t.Errorf("Expected group field 'category', got '%s'", gs.GetGroupField())
	}

	if gs.GetGroupLimit() != 10 {
		t.Errorf("Expected default group limit 10, got %d", gs.GetGroupLimit())
	}

	if gs.GetDocLimit() != 1 {
		t.Errorf("Expected default doc limit 1, got %d", gs.GetDocLimit())
	}
}

func TestGroupingSearchSetters(t *testing.T) {
	gs := NewGroupingSearch("category")

	// Test SetGroupSort
	sortField := search.NewSortField("name", search.SortFieldTypeString)
	gs.SetGroupSort(*search.NewSort(sortField))
	if gs.GetGroupSort().Fields == nil {
		t.Error("Expected group sort to be set")
	}

	// Test SetGroupSortByField
	gs.SetGroupSortByField("date", true)
	if gs.GetGroupSort().Fields == nil {
		t.Error("Expected group sort by field to be set")
	}

	// Test SetDocSort
	docSortField := search.NewSortField("score", search.SortFieldTypeScore)
	docSortField.Reverse = true
	gs.SetDocSort(*search.NewSort(docSortField))
	if gs.GetDocSort().Fields == nil {
		t.Error("Expected doc sort to be set")
	}

	// Test SetDocSortByField
	gs.SetDocSortByField("relevance", false)
	if gs.GetDocSort().Fields == nil {
		t.Error("Expected doc sort by field to be set")
	}

	// Test SetGroupOffset
	gs.SetGroupOffset(5)
	if gs.GetGroupOffset() != 5 {
		t.Errorf("Expected group offset 5, got %d", gs.GetGroupOffset())
	}

	// Test SetGroupLimit
	gs.SetGroupLimit(20)
	if gs.GetGroupLimit() != 20 {
		t.Errorf("Expected group limit 20, got %d", gs.GetGroupLimit())
	}

	// Test SetDocOffset
	gs.SetDocOffset(2)
	if gs.GetDocOffset() != 2 {
		t.Errorf("Expected doc offset 2, got %d", gs.GetDocOffset())
	}

	// Test SetDocLimit
	gs.SetDocLimit(5)
	if gs.GetDocLimit() != 5 {
		t.Errorf("Expected doc limit 5, got %d", gs.GetDocLimit())
	}
}

func TestGroupingSearchChaining(t *testing.T) {
	gs := NewGroupingSearch("category").
		SetGroupLimit(50).
		SetDocLimit(10).
		SetGroupOffset(0).
		SetDocOffset(0)

	if gs.GetGroupLimit() != 50 {
		t.Errorf("Expected group limit 50, got %d", gs.GetGroupLimit())
	}

	if gs.GetDocLimit() != 10 {
		t.Errorf("Expected doc limit 10, got %d", gs.GetDocLimit())
	}
}

func TestGroupingSearchString(t *testing.T) {
	gs := NewGroupingSearch("category")
	str := gs.String()

	if str == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestNewGroupDocs(t *testing.T) {
	gd := NewGroupDocs("electronics", 1.5)

	if gd == nil {
		t.Fatal("Expected GroupDocs to be created")
	}

	if gd.GroupValue != "electronics" {
		t.Errorf("Expected group value 'electronics', got %v", gd.GroupValue)
	}

	if gd.Score != 1.5 {
		t.Errorf("Expected score 1.5, got %f", gd.Score)
	}

	if len(gd.ScoreDocs) != 0 {
		t.Errorf("Expected empty score docs, got %d", len(gd.ScoreDocs))
	}
}

func TestGroupDocsAddScoreDoc(t *testing.T) {
	gd := NewGroupDocs("test", 1.0)
	sd := &search.ScoreDoc{Doc: 1, Score: 1.0}

	gd.AddScoreDoc(sd)

	if gd.GetScoreDocCount() != 1 {
		t.Errorf("Expected 1 score doc, got %d", gd.GetScoreDocCount())
	}

	retrieved := gd.GetScoreDoc(0)
	if retrieved != sd {
		t.Error("Expected retrieved score doc to match")
	}

	// Out of bounds
	if gd.GetScoreDoc(1) != nil {
		t.Error("Expected nil for out of bounds")
	}
}
