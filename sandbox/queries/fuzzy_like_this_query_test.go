package queries

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestFuzzyLikeThisQuery_Rewrite(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	iw, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	f, _ := document.NewTextField("field", "apple banana cherry", true)
	doc.Add(f)
	if err := iw.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := iw.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := iw.Close(); err != nil {
		t.Fatalf("writer Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	flt := NewFuzzyLikeThisQuery(10, analyzer)
	flt.AddTerms("aple banana", "field", 1, 1)

	rewritten, err := flt.Rewrite(reader)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}

	bq, ok := rewritten.(*search.BooleanQuery)
	if !ok {
		t.Fatalf("expected *BooleanQuery, got %T", rewritten)
	}
	if len(bq.Clauses()) == 0 {
		t.Fatal("expected at least one clause")
	}
	for _, c := range bq.Clauses() {
		if c.Occur != search.SHOULD {
			t.Errorf("expected SHOULD, got %v", c.Occur)
		}
		_, isFuzzy := c.Query.(*search.FuzzyQuery)
		_, isCSQ := c.Query.(*search.ConstantScoreQuery)
		if !isFuzzy && !isCSQ {
			t.Errorf("expected FuzzyQuery or ConstantScoreQuery(FuzzyQuery), got %T", c.Query)
		}
	}
}

func TestFuzzyLikeThisQuery_NonExistingField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	iw, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	f, _ := document.NewTextField("field", "apple banana", true)
	doc.Add(f)
	if err := iw.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := iw.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := iw.Close(); err != nil {
		t.Fatalf("writer Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	flt := NewFuzzyLikeThisQuery(10, analyzer)
	flt.AddTerms("aple", "nonexistent", 1, 1)

	rewritten, err := flt.Rewrite(reader)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	_, isMatchNoDocs := rewritten.(*search.MatchNoDocsQuery)
	if !isMatchNoDocs {
		t.Fatalf("expected MatchNoDocsQuery for non-existing field, got %T", rewritten)
	}
}

func TestFuzzyLikeThisQuery_Equals(t *testing.T) {
	analyzer := analysis.NewWhitespaceAnalyzer()
	flt1 := NewFuzzyLikeThisQuery(10, analyzer)
	flt1.AddTerms("hello", "field", 1, 1)
	flt1.SetIgnoreTF(true)

	flt2 := NewFuzzyLikeThisQuery(10, analyzer)
	flt2.AddTerms("hello", "field", 1, 1)
	flt2.SetIgnoreTF(true)

	if !flt1.Equals(flt2) {
		t.Fatal("expected identical FLT queries to be equal")
	}

	flt3 := NewFuzzyLikeThisQuery(10, analyzer)
	flt3.AddTerms("hello", "field", 2, 1)
	flt3.SetIgnoreTF(true)

	if flt1.Equals(flt3) {
		t.Fatal("expected different FLT queries not to be equal")
	}
}

func TestFuzzyLikeThisQuery_NoMatchFirstWord(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	iw, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	f, _ := document.NewTextField("field", "apple banana cherry", true)
	doc.Add(f)
	if err := iw.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := iw.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := iw.Close(); err != nil {
		t.Fatalf("writer Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	flt := NewFuzzyLikeThisQuery(10, analyzer)
	// "xyz" does not match anything; "banana" matches exactly.
	flt.AddTerms("xyz banana", "field", 0, 0)

	hits, err := searcher.Search(flt, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if hits.TotalHits.Value != 1 {
		t.Fatalf("expected 1 hit, got %d", hits.TotalHits.Value)
	}
}
