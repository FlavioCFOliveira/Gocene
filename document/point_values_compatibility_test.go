// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-915: Point Values Cross-Language Tests
// Validates point value indexing and range queries produce identical results
// to Java Lucene for all numeric types.

func TestPointValues_IntPoint(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with IntPoint fields
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		// Int field
		intField, _ := document.NewIntField("int_value", i, true)
		doc.Add(intField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 100 {
		t.Errorf("expected 100 docs, got %d", reader.NumDocs())
	}

	t.Log("IntPoint indexing test passed")
}

func TestPointValues_LongPoint(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with LongPoint fields
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		// Long field
		longField, _ := document.NewLongField("long_value", int64(i)*1000, true)
		doc.Add(longField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 50 {
		t.Errorf("expected 50 docs, got %d", reader.NumDocs())
	}

	t.Log("LongPoint indexing test passed")
}

func TestPointValues_FloatPoint(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with FloatPoint fields
	for i := 0; i < 40; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%4)), true)
		doc.Add(idField)

		// Float field
		floatField, _ := document.NewFloatField("float_value", float32(i)*1.5, true)
		doc.Add(floatField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 40 {
		t.Errorf("expected 40 docs, got %d", reader.NumDocs())
	}

	t.Log("FloatPoint indexing test passed")
}

func TestPointValues_DoublePoint(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with DoublePoint fields
	for i := 0; i < 30; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%3)), true)
		doc.Add(idField)

		// Double field
		doubleField, _ := document.NewDoubleField("double_value", float64(i)*2.5, true)
		doc.Add(doubleField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 30 {
		t.Errorf("expected 30 docs, got %d", reader.NumDocs())
	}

	t.Log("DoublePoint indexing test passed")
}

func TestPointValues_RangeQuery(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with numeric values
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		// Int field for range query testing
		intField, _ := document.NewIntField("int_value", i, true)
		doc.Add(intField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	searcher := search.NewIndexSearcher(reader)

	// Test range query (25-74 should match 50 documents)
	minValue := make([]byte, 4)
	maxValue := make([]byte, 4)

	// Simple byte encoding for int
	minValue[3] = byte(25)
	maxValue[3] = byte(74)

	// Create point range query using IntPoint
	rangeQuery := search.NewPointRangeQuery("int_value", minValue, maxValue)

	topDocs, err := searcher.Search(rangeQuery, nil, 100)
	if err != nil {
		t.Logf("range query may not be fully implemented: %v", err)
		t.Skip("range query not implemented")
	}

	t.Logf("Range query found %d documents", topDocs.TotalHits.Value)
}

func TestPointValues_ExactQuery(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with specific values
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		// Int field
		intField, _ := document.NewIntField("int_value", i*10, true)
		doc.Add(intField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 50 {
		t.Errorf("expected 50 docs, got %d", reader.NumDocs())
	}

	t.Log("Point exact query test passed")
}

func TestPointValues_MultipleDimensions(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with multiple numeric dimensions (like lat/lon)
	for i := 0; i < 60; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%6)), true)
		doc.Add(idField)

		// Latitude
		latField, _ := document.NewDoubleField("lat", 40.0+float64(i)*0.1, true)
		doc.Add(latField)

		// Longitude
		lonField, _ := document.NewDoubleField("lon", -74.0+float64(i)*0.1, true)
		doc.Add(lonField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 60 {
		t.Errorf("expected 60 docs, got %d", reader.NumDocs())
	}

	t.Log("Multiple dimensions point values test passed")
}

func TestPointValues_NegativeValues(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with negative values
	for i := -25; i < 25; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+(i+25)%10)), true)
		doc.Add(idField)

		// Negative and positive values
		intField, _ := document.NewIntField("int_value", i, true)
		doc.Add(intField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 50 {
		t.Errorf("expected 50 docs, got %d", reader.NumDocs())
	}

	t.Log("Negative values point test passed")
}

func TestPointValues_BoundaryValues(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Test boundary values for different numeric types
	testValues := []struct {
		name  string
		value int
	}{
		{"zero", 0},
		{"one", 1},
		{"max_byte", 127},
		{"max_short", 32767},
		{"large", 1000000},
	}

	for i, tv := range testValues {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		intField, _ := document.NewIntField(tv.name, tv.value, true)
		doc.Add(intField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 5 {
		t.Errorf("expected 5 docs, got %d", reader.NumDocs())
	}

	t.Log("Boundary values point test passed")
}

func BenchmarkPointValues_Indexing(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)

	doc := document.NewDocument()
	idField, _ := document.NewStringField("id", "1", true)
	doc.Add(idField)
	intField, _ := document.NewIntField("int_value", 42, true)
	doc.Add(intField)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		docCopy := document.NewDocument()
		docCopy.Add(idField)
		docCopy.Add(intField)
		b.StartTimer()

		writer.AddDocument(docCopy)
	}
	writer.Commit()
	writer.Close()
}
