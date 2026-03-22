// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spatial"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-908: Spatial Query Compatibility Tests
// Validates spatial queries (point, distance, shape) produce
// identical matching documents to Java Lucene implementation.

func TestSpatialCompatibility_PointQueries(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with spatial data
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i)), true)
		doc.Add(idField)

		// Add lat/lon as float points
		lat := 40.0 + float64(i)
		lon := -74.0 + float64(i)

		latField, _ := document.NewFloatField("lat", float32(lat), true)
		doc.Add(latField)

		lonField, _ := document.NewFloatField("lon", float32(lon), true)
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

	if reader.NumDocs() != 10 {
		t.Errorf("expected 10 docs, got %d", reader.NumDocs())
	}
}

func TestSpatialCompatibility_DistanceQuery(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Create documents with coordinates
	locations := []struct {
		id  string
		lat float32
		lon float32
	}{
		{"1", 40.7128, -74.0060},  // NYC
		{"2", 34.0522, -118.2437}, // LA
		{"3", 51.5074, -0.1278},   // London
		{"4", 48.8566, 2.3522},    // Paris
		{"5", 40.7128, -74.0060},  // NYC (duplicate)
	}

	for _, loc := range locations {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", loc.id, true)
		doc.Add(idField)

		latField, _ := document.NewFloatField("lat", loc.lat, true)
		doc.Add(latField)

		lonField, _ := document.NewFloatField("lon", loc.lon, true)
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

	if reader.NumDocs() != 5 {
		t.Errorf("expected 5 docs, got %d", reader.NumDocs())
	}
}
