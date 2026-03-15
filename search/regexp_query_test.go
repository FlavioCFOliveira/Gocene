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

// TestRegexpQuery_Regex1 tests basic regex pattern matching
// Source: TestRegexpQuery.testRegex1()
// Purpose: Tests regex pattern "q.[aeiou]c.*" which should match "quick"
func TestRegexpQuery_Regex1(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "the quick brown fox jumps over the lazy ??? dog 493432 49344 [foo] 12.3 \\ \u03C2", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Test regex "q.[aeiou]c.*" - should match "quick"
	query := search.NewRegexpQuery(index.NewTerm("field", "q.[aeiou]c.*"))
	topDocs, err := searcher.Search(query, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocs.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for regex 'q.[aeiou]c.*', got %d", topDocs.TotalHits.Value)
	}
}

// TestRegexpQuery_Regex2 tests regex that should not match
// Source: TestRegexpQuery.testRegex2()
// Purpose: Tests regex pattern ".[aeiou]c.*" which should not match
func TestRegexpQuery_Regex2(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "the quick brown fox jumps over the lazy ??? dog 493432 49344 [foo] 12.3 \\ \u03C2", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Test regex ".[aeiou]c.*" - should not match
	query := search.NewRegexpQuery(index.NewTerm("field", ".[aeiou]c.*"))
	topDocs, err := searcher.Search(query, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocs.TotalHits.Value != 0 {
		t.Errorf("Expected 0 hits for regex '.[aeiou]c.*', got %d", topDocs.TotalHits.Value)
	}
}

// TestRegexpQuery_Regex3 tests regex without wildcard that should not match
// Source: TestRegexpQuery.testRegex3()
// Purpose: Tests regex pattern "q.[aeiou]c" which should not match (no wildcard at end)
func TestRegexpQuery_Regex3(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "the quick brown fox jumps over the lazy ??? dog 493432 49344 [foo] 12.3 \\ \u03C2", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Test regex "q.[aeiou]c" - should not match (needs exact match)
	query := search.NewRegexpQuery(index.NewTerm("field", "q.[aeiou]c"))
	topDocs, err := searcher.Search(query, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if topDocs.TotalHits.Value != 0 {
		t.Errorf("Expected 0 hits for regex 'q.[aeiou]c', got %d", topDocs.TotalHits.Value)
	}
}

// TestRegexpQuery_NumericRange tests numeric range patterns
// Source: TestRegexpQuery.testNumericRange()
// Purpose: Tests numeric range syntax "<420000-600000>" and "<493433-600000>"
func TestRegexpQuery_NumericRange(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "the quick brown fox jumps over the lazy ??? dog 493432 49344 [foo] 12.3 \\ \u03C2", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Test numeric range "<420000-600000>" - should match 493432
	query1 := search.NewRegexpQuery(index.NewTerm("field", "<420000-600000>"))
	topDocs1, err := searcher.Search(query1, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if topDocs1.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for numeric range '<420000-600000>', got %d", topDocs1.TotalHits.Value)
	}

	// Test numeric range "<493433-600000>" - should not match (493432 is just below)
	query2 := search.NewRegexpQuery(index.NewTerm("field", "<493433-600000>"))
	topDocs2, err := searcher.Search(query2, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if topDocs2.TotalHits.Value != 0 {
		t.Errorf("Expected 0 hits for numeric range '<493433-600000>', got %d", topDocs2.TotalHits.Value)
	}
}

// TestRegexpQuery_CharacterClasses tests various character class patterns
// Source: TestRegexpQuery.testCharacterClasses()
// Purpose: Tests \d, \w, \W, \S, and other character class patterns
func TestRegexpQuery_CharacterClasses(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "the quick brown fox jumps over the lazy ??? dog 493432 49344 [foo] 12.3 \\ \u03C2", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Test cases: regex -> expected hits
	testCases := []struct {
		regex     string
		expected  int64
		name      string
	}{
		{"\\d", 0, "single digit"},                    // No single digit tokens
		{"\\d*", 1, "zero or more digits"},           // Matches empty or numeric tokens
		{"\\d{6}", 1, "exactly 6 digits"},            // Matches 493432
		{"[a\\d]{6}", 1, "6 chars of a or digit"},    // Matches 493432
		{"\\d{2,7}", 1, "2 to 7 digits"},             // Matches 493432
		{"\\d{4}", 0, "exactly 4 digits"},            // No 4-digit token
		{"\\dog", 0, "digit followed by 'og'"},       // No such token
		{"493\\d32", 1, "493 followed by digit and 32"}, // Matches 493432
		{"\\wox", 1, "word char followed by 'ox'"}, // Matches fox
		{"493\\w32", 1, "493, word char, 32"},        // Matches 493432
		{"\\?\\?\\?", 1, "three question marks"},    // Matches ???
		{"\\?\\W\\?", 1, "question, non-word, question"}, // Matches ???
		{"\\?\\S\\?", 1, "question, non-space, question"}, // Matches ???
		{"\\[foo\\]", 1, "literal [foo]"},            // Matches [foo]
		{"\\[\\w{3}\\]", 1, "bracket with 3 word chars"}, // Matches [foo]
		{"\\s.*", 0, "whitespace followed by anything"}, // No matches (whitespace stripped)
		{"\\S*ck", 1, "non-space chars ending in 'ck'"}, // Matches quick
		{"[\\d\\.]{3,10}", 1, "3-10 digits or dots"}, // Matches 12.3
		{"\\d{1,3}(\\.(\\d{1,2}))+", 1, "decimal number pattern"}, // Matches 12.3
		{"\\\\", 1, "single backslash"},            // Matches \
		{"\\\\.*", 1, "backslash followed by anything"}, // Matches \
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query := search.NewRegexpQuery(index.NewTerm("field", tc.regex))
			topDocs, err := searcher.Search(query, 5)
			if err != nil {
				t.Fatalf("Search failed for regex '%s': %v", tc.regex, err)
			}
			if topDocs.TotalHits.Value != tc.expected {
				t.Errorf("Regex '%s': expected %d hits, got %d", tc.regex, tc.expected, topDocs.TotalHits.Value)
			}
		})
	}
}

// TestRegexpQuery_CharacterClasses_Invalid tests invalid character class
// Source: TestRegexpQuery.testCharacterClasses() - invalid character class test
// Purpose: Tests that invalid character class "\p" throws IllegalArgumentException
func TestRegexpQuery_CharacterClasses_Invalid(t *testing.T) {
	// Test invalid character class "\p" - should panic or return error
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid character class '\\p'")
		}
	}()

	// This should panic due to invalid character class
	_ = search.NewRegexpQuery(index.NewTerm("field", "\\p"))
}

// TestRegexpQuery_CaseInsensitive tests case-insensitive matching
// Source: TestRegexpQuery.testCaseInsensitive()
// Purpose: Tests case-insensitive regex matching with ASCII and Unicode
func TestRegexpQuery_CaseInsensitive(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "the quick brown fox jumps over the lazy ??? dog 493432 49344 [foo] 12.3 \\ \u03C2", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Test case-sensitive "Quick" - should not match (lowercase "quick" in doc)
	query1 := search.NewRegexpQuery(index.NewTerm("field", "Quick"))
	topDocs1, err := searcher.Search(query1, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if topDocs1.TotalHits.Value != 0 {
		t.Errorf("Expected 0 hits for case-sensitive 'Quick', got %d", topDocs1.TotalHits.Value)
	}

	// Test case-insensitive "Quick" - should match
	query2 := search.NewRegexpQueryWithFlags(
		index.NewTerm("field", "Quick"),
		search.RegExpSyntaxAll,
		search.RegExpCaseInsensitive|search.RegExpASCIICaseInsensitive,
	)
	topDocs2, err := searcher.Search(query2, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if topDocs2.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for case-insensitive 'Quick', got %d", topDocs2.TotalHits.Value)
	}

	// Test case-insensitive Greek sigma "\u03A3" (uppercase) - should match "\u03C2" (lowercase)
	query3 := search.NewRegexpQueryWithFlags(
		index.NewTerm("field", "\u03A3"),
		search.RegExpSyntaxAll,
		search.RegExpCaseInsensitive|search.RegExpASCIICaseInsensitive,
	)
	topDocs3, err := searcher.Search(query3, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if topDocs3.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for case-insensitive Greek sigma, got %d", topDocs3.TotalHits.Value)
	}

	// Test case-insensitive Greek sigma "\u03C3" (lowercase) - should match "\u03C2" (final sigma)
	query4 := search.NewRegexpQueryWithFlags(
		index.NewTerm("field", "\u03C3"),
		search.RegExpSyntaxAll,
		search.RegExpCaseInsensitive|search.RegExpASCIICaseInsensitive,
	)
	topDocs4, err := searcher.Search(query4, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if topDocs4.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for case-insensitive lowercase sigma, got %d", topDocs4.TotalHits.Value)
	}
}

// TestRegexpQuery_NegatedCharacterClass tests negated character classes
// Source: TestRegexpQuery.testRegexNegatedCharacterClass()
// Purpose: Tests patterns like "[^a-z]" and "[^03ad]"
func TestRegexpQuery_NegatedCharacterClass(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "the quick brown fox jumps over the lazy ??? dog 493432 49344 [foo] 12.3 \\ \u03C2", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Test negated character class "[^a-z]" - should match non-lowercase tokens
	query1 := search.NewRegexpQuery(index.NewTerm("field", "[^a-z]"))
	topDocs1, err := searcher.Search(query1, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if topDocs1.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for '[^a-z]', got %d", topDocs1.TotalHits.Value)
	}

	// Test negated character class "[^03ad]" - should match tokens not containing 0,3,a,d
	query2 := search.NewRegexpQuery(index.NewTerm("field", "[^03ad]"))
	topDocs2, err := searcher.Search(query2, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if topDocs2.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for '[^03ad]', got %d", topDocs2.TotalHits.Value)
	}
}

// TestRegexpQuery_CustomProvider tests custom AutomatonProvider
// Source: TestRegexpQuery.testCustomProvider()
// Purpose: Tests named automata via custom AutomatonProvider
func TestRegexpQuery_CustomProvider(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "the quick brown fox jumps over the lazy ??? dog 493432 49344 [foo] 12.3 \\ \u03C2", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Create custom automaton provider that provides "quickBrown" automaton
	provider := search.NewAutomatonProviderFunc(func(name string) search.Automaton {
		if name == "quickBrown" {
			// Return automaton that matches "quick", "brown", or "bob"
			return search.UnionAutomata([]string{"quick", "brown", "bob"})
		}
		return nil
	})

	// Test query with custom provider using <quickBrown> syntax
	query := search.NewRegexpQueryWithProvider(
		index.NewTerm("field", "<quickBrown>"),
		search.RegExpSyntaxAll,
		provider,
	)
	topDocs, err := searcher.Search(query, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if topDocs.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for custom provider '<quickBrown>', got %d", topDocs.TotalHits.Value)
	}
}

// TestRegexpQuery_Backtracking tests backtracking corner case
// Source: TestRegexpQuery.testBacktracking()
// Purpose: Tests backtracking when term dictionary has 493432 followed by 49344
func TestRegexpQuery_Backtracking(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "the quick brown fox jumps over the lazy ??? dog 493432 49344 [foo] 12.3 \\ \u03C2", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Test backtracking pattern "4934[314]"
	// When backtracking from 49343... to 4934, need to test that 4934 itself is ok
	query := search.NewRegexpQuery(index.NewTerm("field", "4934[314]"))
	topDocs, err := searcher.Search(query, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if topDocs.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for backtracking pattern '4934[314]', got %d", topDocs.TotalHits.Value)
	}
}

// TestRegexpQuery_SlowCommonSuffix tests worst-case for getCommonSuffix optimization
// Source: TestRegexpQuery.testSlowCommonSuffix()
// Purpose: Tests that overly complex regex throws TooComplexToDeterminizeException
func TestRegexpQuery_SlowCommonSuffix(t *testing.T) {
	// Test that overly complex regex "(.*a){2000}" throws TooComplexToDeterminizeException
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for overly complex regex '(.*a){2000}'")
		}
	}()

	// This should panic due to complexity
	_ = search.NewRegexpQuery(index.NewTerm("field", "(.*a){2000}"))
}

// TestRegexpQuery_Basics tests basic RegexpQuery functionality
// Purpose: Tests query construction, cloning, equality, and string representation
func TestRegexpQuery_Basics(t *testing.T) {
	term := index.NewTerm("field", "qu.*ck")
	query := search.NewRegexpQuery(term)

	// Test GetRegexp
	if !query.GetRegexp().Equals(term) {
		t.Error("GetRegexp() should return original term")
	}

	// Test Clone
	cloned := query.Clone().(*search.RegexpQuery)
	if !cloned.GetRegexp().Equals(term) {
		t.Error("Cloned regexp should equal original")
	}

	// Test Equals
	query2 := search.NewRegexpQuery(index.NewTerm("field", "qu.*ck"))
	if !query.Equals(query2) {
		t.Error("Queries with same term should be equal")
	}

	// Test not equal - different pattern
	query3 := search.NewRegexpQuery(index.NewTerm("field", "br.*wn"))
	if query.Equals(query3) {
		t.Error("Queries with different patterns should not be equal")
	}

	// Test not equal - different field
	query4 := search.NewRegexpQuery(index.NewTerm("other", "qu.*ck"))
	if query.Equals(query4) {
		t.Error("Queries with different fields should not be equal")
	}

	// Test HashCode
	if query.HashCode() != query2.HashCode() {
		t.Error("Equal queries should have same HashCode")
	}

	// Test String representation
	str := query.String()
	if str != "field:/qu.*ck/" {
		t.Errorf("Expected 'field:/qu.*ck/', got %q", str)
	}
}

// TestRegexpQuery_ConstructorVariants tests different RegexpQuery constructors
// Purpose: Tests all constructor variants with different flag combinations
func TestRegexpQuery_ConstructorVariants(t *testing.T) {
	term := index.NewTerm("field", "test.*")

	// Test basic constructor
	q1 := search.NewRegexpQuery(term)
	if q1 == nil {
		t.Error("NewRegexpQuery should not return nil")
	}

	// Test constructor with syntax flags
	q2 := search.NewRegexpQueryWithSyntaxFlags(term, search.RegExpSyntaxAll)
	if q2 == nil {
		t.Error("NewRegexpQueryWithSyntaxFlags should not return nil")
	}

	// Test constructor with syntax and match flags
	q3 := search.NewRegexpQueryWithFlags(term, search.RegExpSyntaxAll, search.RegExpCaseInsensitive)
	if q3 == nil {
		t.Error("NewRegexpQueryWithFlags should not return nil")
	}

	// Test constructor with all parameters
	provider := search.NewAutomatonProviderFunc(func(name string) search.Automaton {
		return nil
	})
	q4 := search.NewRegexpQueryWithProvider(term, search.RegExpSyntaxAll, provider)
	if q4 == nil {
		t.Error("NewRegexpQueryWithProvider should not return nil")
	}
}

// TestRegexpQuery_Rewrite tests query rewriting
// Purpose: Tests that RegexpQuery rewrites correctly
func TestRegexpQuery_Rewrite(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "quick brown fox", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	query := search.NewRegexpQuery(index.NewTerm("field", "qu.*"))
	rewritten, err := query.Rewrite(reader)
	if err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}

	// RegexpQuery should rewrite to a form that can be executed
	if rewritten == nil {
		t.Error("Rewritten query should not be nil")
	}
}

// TestRegexpQuery_EmptyPattern tests empty pattern behavior
// Purpose: Tests that empty regex pattern matches all terms
func TestRegexpQuery_EmptyPattern(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "quick brown fox", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Empty pattern should match all terms
	query := search.NewRegexpQuery(index.NewTerm("field", ""))
	topDocs, err := searcher.Search(query, 100)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should match all tokens in the document
	if topDocs.TotalHits.Value < 1 {
		t.Errorf("Expected at least 1 hit for empty pattern, got %d", topDocs.TotalHits.Value)
	}
}

// TestRegexpQuery_SpecialCharacters tests special regex characters
// Purpose: Tests escaping of special regex metacharacters
func TestRegexpQuery_SpecialCharacters(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "a.b*c+d?e[f]g", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Test matching literal special characters with escaping
	testCases := []struct {
		regex    string
		expected int64
		name     string
	}{
		{"a\\.b", 1, "literal dot"},
		{"a.b", 1, "dot matches any char"},
		{"c\\*d", 1, "literal asterisk"},
		{"c.*d", 1, "asterisk as quantifier"},
		{"e\\[f\\]", 1, "literal brackets"},
		{"e\\[.*\\]", 1, "brackets with content"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query := search.NewRegexpQuery(index.NewTerm("field", tc.regex))
			topDocs, err := searcher.Search(query, 5)
			if err != nil {
				t.Fatalf("Search failed for regex '%s': %v", tc.regex, err)
			}
			if topDocs.TotalHits.Value != tc.expected {
				t.Errorf("Regex '%s': expected %d hits, got %d", tc.regex, tc.expected, topDocs.TotalHits.Value)
			}
		})
	}
}

// TestRegexpQuery_Alternation tests alternation (OR) patterns
// Source: Implicit in regex syntax
// Purpose: Tests | operator for alternation
func TestRegexpQuery_Alternation(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "quick brown fox", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Test alternation pattern
	query := search.NewRegexpQuery(index.NewTerm("field", "(quick|brown)"))
	topDocs, err := searcher.Search(query, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if topDocs.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for alternation pattern, got %d", topDocs.TotalHits.Value)
	}
}

// TestRegexpQuery_Quantifiers tests quantifier patterns
// Source: Implicit in regex syntax
// Purpose: Tests *, +, ?, {n}, {n,}, {n,m} quantifiers
func TestRegexpQuery_Quantifiers(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "aa ab aaa aaaa", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
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
		{"a?", 1, "zero or one"},
		{"a{2}", 1, "exactly 2"},
		{"a{2,}", 1, "2 or more"},
		{"a{2,4}", 1, "2 to 4"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query := search.NewRegexpQuery(index.NewTerm("field", tc.regex))
			topDocs, err := searcher.Search(query, 5)
			if err != nil {
				t.Fatalf("Search failed for regex '%s': %v", tc.regex, err)
			}
			if topDocs.TotalHits.Value != tc.expected {
				t.Errorf("Regex '%s': expected %d hits, got %d", tc.regex, tc.expected, topDocs.TotalHits.Value)
			}
		})
	}
}

// TestRegexpQuery_Anchors tests anchor patterns
// Source: Implicit in regex syntax
// Purpose: Tests ^ and $ anchors (if supported)
func TestRegexpQuery_Anchors(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "quick brown fox", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Test start anchor
	query1 := search.NewRegexpQuery(index.NewTerm("field", "^quick"))
	topDocs1, err := searcher.Search(query1, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	// Note: Anchors may or may not be supported depending on implementation
	t.Logf("Start anchor '^quick' returned %d hits", topDocs1.TotalHits.Value)

	// Test end anchor
	query2 := search.NewRegexpQuery(index.NewTerm("field", "fox$"))
	topDocs2, err := searcher.Search(query2, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	t.Logf("End anchor 'fox$' returned %d hits", topDocs2.TotalHits.Value)
}

// TestRegexpQuery_Intersection tests intersection (&) operator
// Source: Lucene RegExp specific syntax
// Purpose: Tests intersection of patterns
func TestRegexpQuery_Intersection(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	doc := document.NewDocument()
	field, _ := document.NewTextField("field", "quick brown fox", true)
	doc.Add(field)
	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Test intersection pattern (Lucene-specific syntax)
	// Matches tokens that match both patterns
	query := search.NewRegexpQueryWithSyntaxFlags(
		index.NewTerm("field", "q.* & qu.*"),
		search.RegExpSyntaxIntersection,
	)
	topDocs, err := searcher.Search(query, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if topDocs.TotalHits.Value != 1 {
		t.Errorf("Expected 1 hit for intersection pattern, got %d", topDocs.TotalHits.Value)
	}
}
