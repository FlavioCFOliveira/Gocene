// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"testing"
)

func TestNewIntersectsPrefixTreeQuery(t *testing.T) {
	// Create a quad tree for testing
	quadTree, err := NewQuadPrefixTree(8)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	// Create a test shape (rectangle around London)
	shape := NewRectangle(-0.5, 51.0, 0.5, 52.0)

	// Create the query
	query := NewIntersectsPrefixTreeQuery("location_grid", shape, quadTree, 6)
	if query == nil {
		t.Fatal("NewIntersectsPrefixTreeQuery() returned nil")
	}

	// Verify query properties
	if query.GetFieldName() != "location_grid" {
		t.Errorf("GetFieldName() = %s, want location_grid", query.GetFieldName())
	}

	if query.GetQueryShape() != shape {
		t.Error("GetQueryShape() returned wrong shape")
	}

	// Test Clone
	cloned := query.Clone()
	if cloned == nil {
		t.Error("Clone() returned nil")
	}

	// Test Equals
	if !query.Equals(cloned) {
		t.Error("Equals() returned false for identical query")
	}

	// Test String
	s := query.String()
	if s == "" {
		t.Error("String() returned empty string")
	}

	// Test HashCode
	hash := query.HashCode()
	if hash == 0 {
		t.Error("HashCode() returned 0")
	}
}

func TestNewIsWithinPrefixTreeQuery(t *testing.T) {
	// Create a quad tree for testing
	quadTree, err := NewQuadPrefixTree(8)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	// Create a test shape (rectangle around London)
	shape := NewRectangle(-0.5, 51.0, 0.5, 52.0)

	// Create the query
	query := NewIsWithinPrefixTreeQuery("location_grid", shape, quadTree, 6)
	if query == nil {
		t.Fatal("NewIsWithinPrefixTreeQuery() returned nil")
	}

	// Verify query properties
	if query.GetFieldName() != "location_grid" {
		t.Errorf("GetFieldName() = %s, want location_grid", query.GetFieldName())
	}

	// Test Clone
	cloned := query.Clone()
	if cloned == nil {
		t.Error("Clone() returned nil")
	}

	// Test Equals
	if !query.Equals(cloned) {
		t.Error("Equals() returned false for identical query")
	}

	// Test String
	s := query.String()
	if s == "" {
		t.Error("String() returned empty string")
	}

	// Test HashCode
	hash := query.HashCode()
	if hash == 0 {
		t.Error("HashCode() returned 0")
	}
}

func TestNewContainsPrefixTreeQuery(t *testing.T) {
	// Create a quad tree for testing
	quadTree, err := NewQuadPrefixTree(8)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	// Create a test shape (rectangle around London)
	shape := NewRectangle(-0.5, 51.0, 0.5, 52.0)

	// Create the query
	query := NewContainsPrefixTreeQuery("location_grid", shape, quadTree, 6)
	if query == nil {
		t.Fatal("NewContainsPrefixTreeQuery() returned nil")
	}

	// Verify query properties
	if query.GetFieldName() != "location_grid" {
		t.Errorf("GetFieldName() = %s, want location_grid", query.GetFieldName())
	}

	// Test Clone
	cloned := query.Clone()
	if cloned == nil {
		t.Error("Clone() returned nil")
	}

	// Test Equals
	if !query.Equals(cloned) {
		t.Error("Equals() returned false for identical query")
	}

	// Test String
	s := query.String()
	if s == "" {
		t.Error("String() returned empty string")
	}

	// Test HashCode
	hash := query.HashCode()
	if hash == 0 {
		t.Error("HashCode() returned 0")
	}
}

func TestNewDistanceQuery(t *testing.T) {
	// Create a quad tree for testing
	quadTree, err := NewQuadPrefixTree(8)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	// Create a test point (London)
	center := Point{X: -0.1257, Y: 51.5085}
	distance := 1.0 // 1 degree
	calculator := &HaversineCalculator{}

	// Create the query
	query := NewDistanceQuery("location_grid", center, distance, quadTree, 6, calculator)
	if query == nil {
		t.Fatal("NewDistanceQuery() returned nil")
	}

	// Verify query properties
	if query.GetFieldName() != "location_grid" {
		t.Errorf("GetFieldName() = %s, want location_grid", query.GetFieldName())
	}

	if query.GetCenter() != center {
		t.Error("GetCenter() returned wrong center")
	}

	if query.GetDistance() != distance {
		t.Errorf("GetDistance() = %f, want %f", query.GetDistance(), distance)
	}

	// Test Clone
	cloned := query.Clone()
	if cloned == nil {
		t.Error("Clone() returned nil")
	}

	// Test Equals
	if !query.Equals(cloned) {
		t.Error("Equals() returned false for identical query")
	}

	// Test String
	s := query.String()
	if s == "" {
		t.Error("String() returned empty string")
	}

	// Test HashCode
	hash := query.HashCode()
	if hash == 0 {
		t.Error("HashCode() returned 0")
	}
}

func TestIntersectsPrefixTreeQuery_NotEquals(t *testing.T) {
	// Create a quad tree for testing
	quadTree, err := NewQuadPrefixTree(8)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	// Create two different shapes
	shape1 := NewRectangle(-0.5, 51.0, 0.5, 52.0)
	shape2 := NewRectangle(-1.0, 50.0, 1.0, 53.0)

	// Create two queries with different shapes
	query1 := NewIntersectsPrefixTreeQuery("location_grid", shape1, quadTree, 6)
	query2 := NewIntersectsPrefixTreeQuery("location_grid", shape2, quadTree, 6)

	// Queries should not be equal
	if query1.Equals(query2) {
		t.Error("Equals() returned true for different queries")
	}
}

func TestIsWithinPrefixTreeQuery_NotEquals(t *testing.T) {
	// Create a quad tree for testing
	quadTree, err := NewQuadPrefixTree(8)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	// Create two different shapes
	shape1 := NewRectangle(-0.5, 51.0, 0.5, 52.0)
	shape2 := NewRectangle(-1.0, 50.0, 1.0, 53.0)

	// Create two queries with different shapes
	query1 := NewIsWithinPrefixTreeQuery("location_grid", shape1, quadTree, 6)
	query2 := NewIsWithinPrefixTreeQuery("location_grid", shape2, quadTree, 6)

	// Queries should not be equal
	if query1.Equals(query2) {
		t.Error("Equals() returned true for different queries")
	}
}

func TestContainsPrefixTreeQuery_NotEquals(t *testing.T) {
	// Create a quad tree for testing
	quadTree, err := NewQuadPrefixTree(8)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	// Create two different shapes
	shape1 := NewRectangle(-0.5, 51.0, 0.5, 52.0)
	shape2 := NewRectangle(-1.0, 50.0, 1.0, 53.0)

	// Create two queries with different shapes
	query1 := NewContainsPrefixTreeQuery("location_grid", shape1, quadTree, 6)
	query2 := NewContainsPrefixTreeQuery("location_grid", shape2, quadTree, 6)

	// Queries should not be equal
	if query1.Equals(query2) {
		t.Error("Equals() returned true for different queries")
	}
}

func TestDistanceQuery_NotEquals(t *testing.T) {
	// Create a quad tree for testing
	quadTree, err := NewQuadPrefixTree(8)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	// Create two different centers
	center1 := Point{X: -0.1257, Y: 51.5085}
	center2 := Point{X: -74.0, Y: 40.7}
	calculator := &HaversineCalculator{}

	// Create two queries with different centers
	query1 := NewDistanceQuery("location_grid", center1, 1.0, quadTree, 6, calculator)
	query2 := NewDistanceQuery("location_grid", center2, 1.0, quadTree, 6, calculator)

	// Queries should not be equal
	if query1.Equals(query2) {
		t.Error("Equals() returned true for different queries")
	}
}

func TestHashCode_Consistency(t *testing.T) {
	// Create a quad tree for testing
	quadTree, err := NewQuadPrefixTree(8)
	if err != nil {
		t.Fatalf("NewQuadPrefixTree() error = %v", err)
	}

	// Create a test shape
	shape := NewRectangle(-0.5, 51.0, 0.5, 52.0)

	// Create the same query twice
	query1 := NewIntersectsPrefixTreeQuery("location_grid", shape, quadTree, 6)
	query2 := NewIntersectsPrefixTreeQuery("location_grid", shape, quadTree, 6)

	// Hash codes should be equal for equal queries
	hash1 := query1.HashCode()
	hash2 := query2.HashCode()
	if hash1 != hash2 {
		t.Errorf("HashCode() returned different values for equal queries: %d vs %d", hash1, hash2)
	}
}
