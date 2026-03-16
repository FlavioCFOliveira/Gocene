package grouping

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestNewTopGroups(t *testing.T) {
	groupSort := *search.NewSortByScore()
	docSort := *search.NewSortByScore()

	tg := NewTopGroups(groupSort, docSort, 0, 10)

	if tg == nil {
		t.Fatal("Expected TopGroups to be created")
	}

	if tg.GetGroupCount() != 0 {
		t.Errorf("Expected 0 groups, got %d", tg.GetGroupCount())
	}

	if tg.GetTotalGroupCount() != -1 {
		t.Errorf("Expected total group count -1 (unknown), got %d", tg.GetTotalGroupCount())
	}

	if tg.GetGroupOffset() != 0 {
		t.Errorf("Expected group offset 0, got %d", tg.GetGroupOffset())
	}

	if tg.GetGroupLimit() != 10 {
		t.Errorf("Expected group limit 10, got %d", tg.GetGroupLimit())
	}
}

func TestTopGroupsAddGroup(t *testing.T) {
	tg := NewTopGroups(*search.NewSortByScore(), *search.NewSortByScore(), 0, 10)
	gd := NewGroupDocs("test", 1.0)

	tg.AddGroup(gd)

	if tg.GetGroupCount() != 1 {
		t.Errorf("Expected 1 group, got %d", tg.GetGroupCount())
	}

	retrieved := tg.GetGroup(0)
	if retrieved != gd {
		t.Error("Expected retrieved group to match")
	}

	// Out of bounds
	if tg.GetGroup(1) != nil {
		t.Error("Expected nil for out of bounds")
	}
}

func TestTopGroupsSetTotalGroupCount(t *testing.T) {
	tg := NewTopGroups(*search.NewSortByScore(), *search.NewSortByScore(), 0, 10)

	tg.SetTotalGroupCount(100)

	if tg.GetTotalGroupCount() != 100 {
		t.Errorf("Expected total group count 100, got %d", tg.GetTotalGroupCount())
	}

	if !tg.IsTotalGroupCountExact() {
		t.Error("Expected total group count to be exact")
	}
}

func TestTopGroupsSetTotalHitCount(t *testing.T) {
	tg := NewTopGroups(*search.NewSortByScore(), *search.NewSortByScore(), 0, 10)

	tg.SetTotalHitCount(500)

	if tg.GetTotalHitCount() != 500 {
		t.Errorf("Expected total hit count 500, got %d", tg.GetTotalHitCount())
	}
}

func TestTopGroupsMerge(t *testing.T) {
	tg1 := NewTopGroups(*search.NewSortByScore(), *search.NewSortByScore(), 0, 10)
	tg1.AddGroup(NewGroupDocs("a", 1.0))
	tg1.SetTotalGroupCount(5)
	tg1.SetTotalHitCount(50)

	tg2 := NewTopGroups(*search.NewSortByScore(), *search.NewSortByScore(), 0, 10)
	tg2.AddGroup(NewGroupDocs("b", 2.0))
	tg2.SetTotalGroupCount(3)
	tg2.SetTotalHitCount(30)

	err := tg1.Merge(tg2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if tg1.GetGroupCount() != 2 {
		t.Errorf("Expected 2 groups after merge, got %d", tg1.GetGroupCount())
	}

	if tg1.GetTotalGroupCount() != 8 {
		t.Errorf("Expected total group count 8, got %d", tg1.GetTotalGroupCount())
	}

	if tg1.GetTotalHitCount() != 80 {
		t.Errorf("Expected total hit count 80, got %d", tg1.GetTotalHitCount())
	}
}

func TestTopGroupsMergeNil(t *testing.T) {
	tg := NewTopGroups(*search.NewSortByScore(), *search.NewSortByScore(), 0, 10)

	err := tg.Merge(nil)
	if err != nil {
		t.Errorf("Expected no error for nil merge, got %v", err)
	}
}

func TestTopGroupsCopy(t *testing.T) {
	original := NewTopGroups(*search.NewSortByScore(), *search.NewSortByScore(), 0, 10)
	original.AddGroup(NewGroupDocs("test", 1.0))
	original.SetTotalGroupCount(10)
	original.SetTotalHitCount(100)

	copy := NewTopGroups(*search.NewSortByScore(), *search.NewSortByScore(), 0, 10)
	copy.Copy(original)

	if copy.GetGroupCount() != 1 {
		t.Errorf("Expected 1 group in copy, got %d", copy.GetGroupCount())
	}

	if copy.GetTotalGroupCount() != 10 {
		t.Errorf("Expected total group count 10 in copy, got %d", copy.GetTotalGroupCount())
	}

	if copy.GetTotalHitCount() != 100 {
		t.Errorf("Expected total hit count 100 in copy, got %d", copy.GetTotalHitCount())
	}
}

func TestTopGroupsString(t *testing.T) {
	tg := NewTopGroups(*search.NewSortByScore(), *search.NewSortByScore(), 0, 10)
	tg.AddGroup(NewGroupDocs("a", 1.0))
	tg.SetTotalGroupCount(10)
	tg.SetTotalHitCount(100)

	str := tg.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestNewTopGroupsMerger(t *testing.T) {
	merger := NewTopGroupsMerger(*search.NewSortByScore(), *search.NewSortByScore(), 0, 10)

	if merger == nil {
		t.Fatal("Expected TopGroupsMerger to be created")
	}
}

func TestTopGroupsMergerMerge(t *testing.T) {
	merger := NewTopGroupsMerger(*search.NewSortByScore(), *search.NewSortByScore(), 0, 10)

	tg1 := NewTopGroups(*search.NewSortByScore(), *search.NewSortByScore(), 0, 10)
	tg1.AddGroup(NewGroupDocs("a", 1.0))

	tg2 := NewTopGroups(*search.NewSortByScore(), *search.NewSortByScore(), 0, 10)
	tg2.AddGroup(NewGroupDocs("b", 2.0))

	result, err := merger.Merge([]*TopGroups{tg1, tg2})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.GetGroupCount() != 2 {
		t.Errorf("Expected 2 groups after merge, got %d", result.GetGroupCount())
	}
}

func TestTopGroupsMergerMergeEmpty(t *testing.T) {
	merger := NewTopGroupsMerger(*search.NewSortByScore(), *search.NewSortByScore(), 0, 10)

	result, err := merger.Merge([]*TopGroups{})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.GetGroupCount() != 0 {
		t.Errorf("Expected 0 groups, got %d", result.GetGroupCount())
	}
}

func TestNewGroupingCollector(t *testing.T) {
	gc := NewGroupingCollector("category", *search.NewSortByScore(), 10)

	if gc == nil {
		t.Fatal("Expected GroupingCollector to be created")
	}

	if gc.GetGroupCount() != 0 {
		t.Errorf("Expected 0 groups, got %d", gc.GetGroupCount())
	}
}

func TestGroupingCollectorGetGroup(t *testing.T) {
	gc := NewGroupingCollector("category", *search.NewSortByScore(), 10)

	// Non-existent group
	if gc.GetGroup("nonexistent") != nil {
		t.Error("Expected nil for non-existent group")
	}
}

func TestGroupingCollectorGetAllGroups(t *testing.T) {
	gc := NewGroupingCollector("category", *search.NewSortByScore(), 10)

	groups := gc.GetAllGroups()
	if len(groups) != 0 {
		t.Errorf("Expected 0 groups, got %d", len(groups))
	}
}
