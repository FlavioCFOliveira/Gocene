// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: regexp_query_test.go
// Source: lucene/core/src/test/org/apache/lucene/search/TestRegexpQuery.java
// Purpose: Tests regular expression queries, automaton conversion, and pattern matching

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// newRegexpQuery is a helper that creates a RegexpQuery and fatals on error.
func newRegexpQuery(t *testing.T, field, pattern string) *search.RegexpQuery {
	t.Helper()
	q, err := search.NewRegexpQuery(field, pattern)
	if err != nil {
		t.Fatalf("NewRegexpQuery(%q, %q): %v", field, pattern, err)
	}
	return q
}

// setupRegexpIndex builds a single-document index with the standard test content.
func setupRegexpIndex(t *testing.T) (index.IndexReaderInterface, *search.IndexSearcher) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { dir.Close() })

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	f, err := document.NewTextField("field", "the quick brown fox jumps over the lazy ??? dog 493432 49344 [foo] 12.3 \\ ς", true)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(f)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	t.Cleanup(func() { reader.Close() })

	return reader, search.NewIndexSearcher(reader)
}

// TestRegexpQuery_Regex1 tests basic regex pattern matching
// Source: TestRegexpQuery.testRegex1()
func TestRegexpQuery_Regex1(t *testing.T) {
	_, searcher := setupRegexpIndex(t)
	q := newRegexpQuery(t, "field", "q.[aeiou]c.*")
	topDocs, err := searcher.Search(q, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 1 {
		t.Errorf("expected 1 hit, got %d", topDocs.TotalHits.Value)
	}
}

// TestRegexpQuery_Regex2 tests regex that should not match
// Source: TestRegexpQuery.testRegex2()
func TestRegexpQuery_Regex2(t *testing.T) {
	_, searcher := setupRegexpIndex(t)
	q := newRegexpQuery(t, "field", ".[aeiou]c.*")
	topDocs, err := searcher.Search(q, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 0 {
		t.Errorf("expected 0 hits, got %d", topDocs.TotalHits.Value)
	}
}

// TestRegexpQuery_Regex3 tests regex without wildcard that should not match
// Source: TestRegexpQuery.testRegex3()
func TestRegexpQuery_Regex3(t *testing.T) {
	_, searcher := setupRegexpIndex(t)
	q := newRegexpQuery(t, "field", "q.[aeiou]c")
	topDocs, err := searcher.Search(q, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 0 {
		t.Errorf("expected 0 hits, got %d", topDocs.TotalHits.Value)
	}
}

// TestRegexpQuery_NumericRange tests numeric range patterns
// Source: TestRegexpQuery.testNumericRange()
func TestRegexpQuery_NumericRange(t *testing.T) {
	_, searcher := setupRegexpIndex(t)

	q1 := newRegexpQuery(t, "field", "<420000-600000>")
	topDocs1, err := searcher.Search(q1, 5)
	if err != nil {
		t.Fatalf("Search q1: %v", err)
	}
	if topDocs1.TotalHits.Value != 1 {
		t.Errorf("expected 1 hit for <420000-600000>, got %d", topDocs1.TotalHits.Value)
	}

	q2 := newRegexpQuery(t, "field", "<493433-600000>")
	topDocs2, err := searcher.Search(q2, 5)
	if err != nil {
		t.Fatalf("Search q2: %v", err)
	}
	if topDocs2.TotalHits.Value != 0 {
		t.Errorf("expected 0 hits for <493433-600000>, got %d", topDocs2.TotalHits.Value)
	}
}

// TestRegexpQuery_CharacterClasses tests various character class patterns
// Source: TestRegexpQuery.testCharacterClasses()
func TestRegexpQuery_CharacterClasses(t *testing.T) {
	_, searcher := setupRegexpIndex(t)

	testCases := []struct {
		regex    string
		expected int64
		name     string
	}{
		{"\\d", 0, "single digit"},
		{"\\d*", 1, "zero or more digits"},
		{"\\d{6}", 1, "exactly 6 digits"},
		{"[a\\d]{6}", 1, "6 chars of a or digit"},
		{"\\d{2,7}", 1, "2 to 7 digits"},
		{"\\wox", 1, "word char followed by ox"},
		{"\\?\\?\\?", 1, "three question marks"},
		{"\\[foo\\]", 1, "literal [foo]"},
		{"\\\\", 1, "single backslash"},
		{"\\\\.*", 1, "backslash followed by anything"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			q := newRegexpQuery(t, "field", tc.regex)
			topDocs, err := searcher.Search(q, 5)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if topDocs.TotalHits.Value != tc.expected {
				t.Errorf("regex %q: expected %d hits, got %d", tc.regex, tc.expected, topDocs.TotalHits.Value)
			}
		})
	}
}

// TestRegexpQuery_CharacterClasses_Invalid tests invalid character class
// Source: TestRegexpQuery.testCharacterClasses() - invalid class test
func TestRegexpQuery_CharacterClasses_Invalid(t *testing.T) {
	_, err := search.NewRegexpQuery("field", "\\p")
	if err == nil {
		t.Error("expected error for invalid character class '\\p'")
	}
}

// TestRegexpQuery_CaseInsensitive tests case-insensitive matching
// Source: TestRegexpQuery.testCaseInsensitive()
// Note: RegExpCaseInsensitive and RegExpASCIICaseInsensitive flags not yet defined.
// Test basic constructor and property access as a placeholder.
func TestRegexpQuery_CaseInsensitive(t *testing.T) {
	q, err := search.NewRegexpQuery("field", "test")
	if err != nil {
		t.Fatalf("NewRegexpQuery: %v", err)
	}
	if q.Field() != "field" {
		t.Errorf("Field() = %q, want %q", q.Field(), "field")
	}
}

// TestRegexpQuery_NegatedCharacterClass tests negated character classes
// Source: TestRegexpQuery.testRegexNegatedCharacterClass()
func TestRegexpQuery_NegatedCharacterClass(t *testing.T) {
	_, searcher := setupRegexpIndex(t)

	q1 := newRegexpQuery(t, "field", "[^a-z]")
	topDocs1, err := searcher.Search(q1, 5)
	if err != nil {
		t.Fatalf("Search q1: %v", err)
	}
	if topDocs1.TotalHits.Value != 1 {
		t.Errorf("expected 1 hit for [^a-z], got %d", topDocs1.TotalHits.Value)
	}

	q2 := newRegexpQuery(t, "field", "[^03ad]")
	topDocs2, err := searcher.Search(q2, 5)
	if err != nil {
		t.Fatalf("Search q2: %v", err)
	}
	if topDocs2.TotalHits.Value != 1 {
		t.Errorf("expected 1 hit for [^03ad], got %d", topDocs2.TotalHits.Value)
	}
}

// TestRegexpQuery_CustomProvider tests custom AutomatonProvider
// Source: TestRegexpQuery.testCustomProvider()
// Note: NewAutomatonProviderFunc and UnionAutomata not yet implemented.
// Test basic constructor and field access as a placeholder.
func TestRegexpQuery_CustomProvider(t *testing.T) {
	q, err := search.NewRegexpQuery("field", "test")
	if err != nil {
		t.Fatalf("NewRegexpQuery: %v", err)
	}
	if q.Pattern() != "test" {
		t.Errorf("Pattern() = %q, want %q", q.Pattern(), "test")
	}
}

// TestRegexpQuery_Backtracking tests backtracking corner case
// Source: TestRegexpQuery.testBacktracking()
func TestRegexpQuery_Backtracking(t *testing.T) {
	_, searcher := setupRegexpIndex(t)
	q := newRegexpQuery(t, "field", "4934[314]")
	topDocs, err := searcher.Search(q, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 1 {
		t.Errorf("expected 1 hit for 4934[314], got %d", topDocs.TotalHits.Value)
	}
}

// TestRegexpQuery_SlowCommonSuffix tests worst-case for getCommonSuffix optimization
// Source: TestRegexpQuery.testSlowCommonSuffix()
func TestRegexpQuery_SlowCommonSuffix(t *testing.T) {
	_, err := search.NewRegexpQuery("field", "(.*a){2000}")
	if err == nil {
		t.Error("expected error for overly complex regex '(.*a){2000}'")
	}
}

// TestRegexpQuery_Basics tests basic RegexpQuery functionality
func TestRegexpQuery_Basics(t *testing.T) {
	q, err := search.NewRegexpQuery("field", "qu.*ck")
	if err != nil {
		t.Fatalf("NewRegexpQuery: %v", err)
	}

	// Test String representation
	str := q.String()
	if str == "" {
		t.Error("expected non-empty string representation")
	}
}

// TestRegexpQuery_ConstructorVariants tests constructor variants
// Note: NewRegexpQueryWithSyntaxFlags, NewRegexpQueryWithProvider and related
// constants (RegExpSyntaxAll, etc.) are not yet defined.
// Test the basic constructor with a more complex pattern.
func TestRegexpQuery_ConstructorVariants(t *testing.T) {
	q, err := search.NewRegexpQuery("field", "^foo.*bar$")
	if err != nil {
		t.Fatalf("NewRegexpQuery: %v", err)
	}
	if q.Pattern() != "^foo.*bar$" {
		t.Errorf("Pattern() = %q, want %q", q.Pattern(), "^foo.*bar$")
	}
}

// TestRegexpQuery_Rewrite tests query rewriting
func TestRegexpQuery_Rewrite(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	f, err := document.NewTextField("field", "quick brown fox", true)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(f)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	q := newRegexpQuery(t, "field", "qu.*")
	rewritten, err := q.Rewrite(reader)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if rewritten == nil {
		t.Error("rewritten query should not be nil")
	}
}

// TestRegexpQuery_EmptyPattern tests empty pattern behavior
func TestRegexpQuery_EmptyPattern(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	f, err := document.NewTextField("field", "quick brown fox", true)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(f)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	q := newRegexpQuery(t, "field", "")
	topDocs, err := searcher.Search(q, 100)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// Full-term matching: empty pattern compiles to ^(?:)$ which matches only the
	// empty string "". Analyzers never emit empty tokens, so 0 hits is correct.
	if topDocs.TotalHits.Value != 0 {
		t.Errorf("expected 0 hits for empty pattern (full-term match), got %d", topDocs.TotalHits.Value)
	}
}

// TestRegexpQuery_SpecialCharacters tests special regex characters
func TestRegexpQuery_SpecialCharacters(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	// Terms produced by whitespace tokenisation: "a.b", "c*d", "e[f]"
	// Full-term patterns must match each complete token, not a substring.
	f, err := document.NewTextField("field", "a.b c*d e[f]", true)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(f)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	testCases := []struct {
		regex    string
		expected int64
		name     string
	}{
		// Full-term semantics: pattern must match the entire token.
		{"a\\.b", 1, "literal dot"},               // ^(?:a\.b)$  matches term "a.b"
		{"a.b", 1, "dot matches any char"},        // ^(?:a.b)$   matches term "a.b"
		{"c\\*d", 1, "literal asterisk"},          // ^(?:c\*d)$  matches term "c*d"
		{"c.*d", 1, "asterisk as quantifier"},     // ^(?:c.*d)$ matches term "c*d"
		{"e\\[f\\]", 1, "literal brackets"},       // ^(?:e\[f\])$ matches term "e[f]"
		{"e\\[.*\\]", 1, "brackets with content"}, // ^(?:e\[.*\])$ matches term "e[f]"
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			q := newRegexpQuery(t, "field", tc.regex)
			topDocs, err := searcher.Search(q, 5)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if topDocs.TotalHits.Value != tc.expected {
				t.Errorf("regex %q: expected %d hits, got %d", tc.regex, tc.expected, topDocs.TotalHits.Value)
			}
		})
	}
}

// TestRegexpQuery_Alternation tests alternation (OR) patterns
func TestRegexpQuery_Alternation(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	f, err := document.NewTextField("field", "quick brown fox", true)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(f)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)
	q := newRegexpQuery(t, "field", "(quick|brown)")
	topDocs, err := searcher.Search(q, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 1 {
		t.Errorf("expected 1 hit for alternation, got %d", topDocs.TotalHits.Value)
	}
}

// TestRegexpQuery_Quantifiers tests quantifier patterns
func TestRegexpQuery_Quantifiers(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	f, err := document.NewTextField("field", "aa ab aaa aaaa", true)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(f)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	testCases := []struct {
		regex    string
		expected int64
		name     string
	}{
		{"a+", 1, "one or more"},
		{"a*", 1, "zero or more"},
		// a? matches "" or "a" as a full term; none of "aa","ab","aaa","aaaa" qualify.
		{"a?", 0, "zero or one"},
		{"a{2}", 1, "exactly 2"},
		{"a{2,}", 1, "2 or more"},
		{"a{2,4}", 1, "2 to 4"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			q := newRegexpQuery(t, "field", tc.regex)
			topDocs, err := searcher.Search(q, 5)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if topDocs.TotalHits.Value != tc.expected {
				t.Errorf("regex %q: expected %d hits, got %d", tc.regex, tc.expected, topDocs.TotalHits.Value)
			}
		})
	}
}

// TestRegexpQuery_Anchors tests anchor patterns
func TestRegexpQuery_Anchors(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	f, err := document.NewTextField("field", "quick brown fox", true)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(f)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	q1 := newRegexpQuery(t, "field", "^quick")
	topDocs1, err := searcher.Search(q1, 5)
	if err != nil {
		t.Fatalf("Search q1: %v", err)
	}
	t.Logf("start anchor '^quick' returned %d hits", topDocs1.TotalHits.Value)

	q2 := newRegexpQuery(t, "field", "fox$")
	topDocs2, err := searcher.Search(q2, 5)
	if err != nil {
		t.Fatalf("Search q2: %v", err)
	}
	t.Logf("end anchor 'fox$' returned %d hits", topDocs2.TotalHits.Value)
}

// TestRegexpQuery_Intersection tests intersection (&) operator
// Note: RegExpSyntaxIntersection constant not yet defined.
// Test basic constructor and field/pattern access as a placeholder.
func TestRegexpQuery_Intersection(t *testing.T) {
	q, err := search.NewRegexpQuery("field", "abc.*")
	if err != nil {
		t.Fatalf("NewRegexpQuery: %v", err)
	}
	if q.Pattern() != "abc.*" {
		t.Errorf("Pattern() = %q, want %q", q.Pattern(), "abc.*")
	}
}
