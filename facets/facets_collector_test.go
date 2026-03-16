package facets

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestNewFacetsCollector(t *testing.T) {
	fc := NewFacetsCollector()

	if fc == nil {
		t.Fatal("Expected FacetsCollector to be created")
	}

	if fc.matchingDocs == nil {
		t.Error("Expected matchingDocs to be initialized")
	}

	if fc.totalHits != 0 {
		t.Errorf("Expected totalHits to be 0, got %d", fc.totalHits)
	}

	if fc.keepScores {
		t.Error("Expected keepScores to be false by default")
	}
}

func TestNewFacetsCollectorWithScores(t *testing.T) {
	fc := NewFacetsCollectorWithScores()

	if fc == nil {
		t.Fatal("Expected FacetsCollector to be created")
	}

	if !fc.keepScores {
		t.Error("Expected keepScores to be true")
	}
}

func TestFacetsCollectorGetTotalHits(t *testing.T) {
	fc := NewFacetsCollector()

	if fc.GetTotalHits() != 0 {
		t.Errorf("Expected GetTotalHits to be 0, got %d", fc.GetTotalHits())
	}
}

func TestFacetsCollectorGetMatchingDocs(t *testing.T) {
	fc := NewFacetsCollector()

	docs := fc.GetMatchingDocs()
	if docs == nil {
		t.Error("Expected GetMatchingDocs to return non-nil slice")
	}

	if len(docs) != 0 {
		t.Errorf("Expected 0 matching docs, got %d", len(docs))
	}
}

func TestFacetsCollectorGetScore(t *testing.T) {
	// Without scores
	fc := NewFacetsCollector()
	if fc.GetScore(0) != 0 {
		t.Error("Expected GetScore to return 0 when not keeping scores")
	}

	// With scores
	fcWithScores := NewFacetsCollectorWithScores()
	fcWithScores.scores[0] = 1.5
	if fcWithScores.GetScore(0) != 1.5 {
		t.Errorf("Expected GetScore to return 1.5, got %f", fcWithScores.GetScore(0))
	}

	// Non-existent doc
	if fcWithScores.GetScore(999) != 0 {
		t.Error("Expected GetScore to return 0 for non-existent doc")
	}
}

func TestFacetsCollectorReset(t *testing.T) {
	fc := NewFacetsCollector()
	fc.totalHits = 10
	fc.scores[0] = 1.0

	fc.Reset()

	if fc.totalHits != 0 {
		t.Errorf("Expected totalHits to be 0 after reset, got %d", fc.totalHits)
	}

	if len(fc.scores) != 0 {
		t.Errorf("Expected scores to be empty after reset, got %d", len(fc.scores))
	}
}

func TestFacetsCollectorScoreMode(t *testing.T) {
	// Without scores
	fc := NewFacetsCollector()
	if fc.ScoreMode() != search.COMPLETE_NO_SCORES {
		t.Errorf("Expected ScoreMode to be COMPLETE_NO_SCORES, got %v", fc.ScoreMode())
	}

	// With scores
	fcWithScores := NewFacetsCollectorWithScores()
	if fcWithScores.ScoreMode() != search.COMPLETE {
		t.Errorf("Expected ScoreMode to be COMPLETE, got %v", fcWithScores.ScoreMode())
	}
}

func TestFacetsCollectorGetLeafCollector(t *testing.T) {
	fc := NewFacetsCollector()

	// Create a mock leaf reader
	segmentInfo := index.NewSegmentInfo("test", 10, nil)
	leafReader := index.NewLeafReader(segmentInfo)

	lc, err := fc.GetLeafCollector(leafReader)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if lc == nil {
		t.Fatal("Expected LeafCollector to be returned")
	}
}

func TestFacetsLeafCollectorCollect(t *testing.T) {
	fc := NewFacetsCollector()

	// Create a mock leaf reader context
	segmentInfo := index.NewSegmentInfo("test", 10, nil)
	leafReader := index.NewLeafReader(segmentInfo)
	ctx := index.NewLeafReaderContext(leafReader, nil, 0, 0)

	flc := &facetsLeafCollector{
		parent:  fc,
		context: ctx,
		docs:    make([]int, 0),
		scores:  make(map[int]float32),
	}

	// Collect some documents
	err := flc.Collect(0)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = flc.Collect(5)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if fc.totalHits != 2 {
		t.Errorf("Expected totalHits to be 2, got %d", fc.totalHits)
	}

	if len(flc.docs) != 2 {
		t.Errorf("Expected 2 docs in leaf collector, got %d", len(flc.docs))
	}
}

func TestFacetsLeafCollectorFinish(t *testing.T) {
	fc := NewFacetsCollector()

	// Create a mock leaf reader context
	segmentInfo := index.NewSegmentInfo("test", 10, nil)
	leafReader := index.NewLeafReader(segmentInfo)
	ctx := index.NewLeafReaderContext(leafReader, nil, 0, 0)

	flc := &facetsLeafCollector{
		parent:  fc,
		context: ctx,
		docs:    []int{0, 5, 9},
		scores:  make(map[int]float32),
	}

	err := flc.Finish()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(fc.matchingDocs) != 1 {
		t.Errorf("Expected 1 MatchingDocs, got %d", len(fc.matchingDocs))
	}

	md := fc.matchingDocs[0]
	if md.TotalHits != 3 {
		t.Errorf("Expected TotalHits to be 3, got %d", md.TotalHits)
	}
}

func TestFacetsLeafCollectorFinishEmpty(t *testing.T) {
	fc := NewFacetsCollector()

	// Create a mock leaf reader context
	segmentInfo := index.NewSegmentInfo("test", 10, nil)
	leafReader := index.NewLeafReader(segmentInfo)
	ctx := index.NewLeafReaderContext(leafReader, nil, 0, 0)

	flc := &facetsLeafCollector{
		parent:  fc,
		context: ctx,
		docs:    make([]int, 0),
		scores:  make(map[int]float32),
	}

	err := flc.Finish()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(fc.matchingDocs) != 0 {
		t.Errorf("Expected 0 MatchingDocs, got %d", len(fc.matchingDocs))
	}
}

func TestNewSearchManagerWithFacets(t *testing.T) {
	smwf := NewSearchManagerWithFacets()

	if smwf == nil {
		t.Fatal("Expected SearchManagerWithFacets to be created")
	}

	if smwf.FacetsCollector == nil {
		t.Error("Expected FacetsCollector to be initialized")
	}
}

func TestNewMultiCollector(t *testing.T) {
	fc1 := NewFacetsCollector()
	fc2 := NewFacetsCollector()

	mc := NewMultiCollector(fc1, fc2)

	if mc == nil {
		t.Fatal("Expected MultiCollector to be created")
	}

	if len(mc.collectors) != 2 {
		t.Errorf("Expected 2 collectors, got %d", len(mc.collectors))
	}
}

func TestNewMultiCollectorWithNil(t *testing.T) {
	fc := NewFacetsCollector()

	mc := NewMultiCollector(fc, nil)

	if len(mc.collectors) != 1 {
		t.Errorf("Expected 1 collector (nil filtered), got %d", len(mc.collectors))
	}
}

func TestMultiCollectorScoreMode(t *testing.T) {
	// Both without scores
	fc1 := NewFacetsCollector()
	fc2 := NewFacetsCollector()
	mc := NewMultiCollector(fc1, fc2)

	if mc.ScoreMode() != search.COMPLETE_NO_SCORES {
		t.Errorf("Expected ScoreMode to be COMPLETE_NO_SCORES, got %v", mc.ScoreMode())
	}

	// One with scores
	fcWithScores := NewFacetsCollectorWithScores()
	mc2 := NewMultiCollector(fc1, fcWithScores)

	if mc2.ScoreMode() != search.COMPLETE {
		t.Errorf("Expected ScoreMode to be COMPLETE, got %v", mc2.ScoreMode())
	}
}

func TestNewFacetsCollectorManager(t *testing.T) {
	fcm := NewFacetsCollectorManager()

	if fcm == nil {
		t.Fatal("Expected FacetsCollectorManager to be created")
	}

	if fcm.keepScores {
		t.Error("Expected keepScores to be false by default")
	}
}

func TestNewFacetsCollectorManagerWithScores(t *testing.T) {
	fcm := NewFacetsCollectorManagerWithScores()

	if fcm == nil {
		t.Fatal("Expected FacetsCollectorManager to be created")
	}

	if !fcm.keepScores {
		t.Error("Expected keepScores to be true")
	}
}

func TestFacetsCollectorManagerNewCollector(t *testing.T) {
	// Without scores
	fcm := NewFacetsCollectorManager()
	fc := fcm.NewCollector()

	if fc == nil {
		t.Fatal("Expected FacetsCollector to be created")
	}

	if fc.keepScores {
		t.Error("Expected keepScores to be false")
	}

	// With scores
	fcmWithScores := NewFacetsCollectorManagerWithScores()
	fcWithScores := fcmWithScores.NewCollector()

	if !fcWithScores.keepScores {
		t.Error("Expected keepScores to be true")
	}
}
