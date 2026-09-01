package components

import (
	"testing"
)

func TestAnalysisTabOperator(t *testing.T) {
	pane := NewAnalyzerPane()
	pane.SetAnalyzerByType("StandardAnalyzer")
}

func TestSimilarityPane(t *testing.T) {
	pane := NewSimilarityPane()
	pane.SetK1(1.5)
	pane.SetB(0.8)
	pane.SetUseClassicSimilarity(true)
	pane.SetDiscountOverlaps(false)

	config := pane.GetConfig()
	if config.K1 != 1.5 {
		t.Errorf("expected k1 1.5, got %f", config.K1)
	}
	if config.B != 0.8 {
		t.Errorf("expected b 0.8, got %f", config.B)
	}
	if !config.UseClassicSimilarity {
		t.Error("expected useClassicSimilarity true")
	}
	if config.DiscountOverlaps {
		t.Error("expected discountOverlaps false")
	}
}

func TestFieldValuesPane(t *testing.T) {
	pane := NewFieldValuesPane()
	fields := []string{"title", "body"}
	pane.SetFields(fields)

	loaded := pane.GetFieldsToLoad()
	if len(loaded) != 2 || loaded[0] != "title" || loaded[1] != "body" {
		t.Errorf("unexpected fields: %v", loaded)
	}
}

func TestURLLabel(t *testing.T) {
	label, err := NewURLLabel("https://lucene.apache.org")
	if err != nil {
		t.Fatalf("failed to create URLLabel: %v", err)
	}
	if label.URL.String() != "https://lucene.apache.org" {
		t.Errorf("unexpected URL: %s", label.URL.String())
	}
}

func TestLukeWindow(t *testing.T) {
	win := NewLukeWindow()
	win.SetColorTheme(DarkTheme)
	if win.theme != DarkTheme {
		t.Error("failed to set theme")
	}
}
