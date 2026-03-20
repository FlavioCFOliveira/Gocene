// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"testing"
)

func TestNewSpatialTestSuite(t *testing.T) {
	suite := NewSpatialTestSuite()
	if suite == nil {
		t.Error("NewSpatialTestSuite() returned nil")
		return
	}
	if suite.GetContext() == nil {
		t.Error("GetContext() returned nil")
	}
}

func TestNewSpatialTestSuiteWithContext(t *testing.T) {
	ctx := NewSpatialContextCartesian(0, 0, 100, 100)
	suite := NewSpatialTestSuiteWithContext(ctx)
	if suite == nil {
		t.Error("NewSpatialTestSuiteWithContext() returned nil")
		return
	}
	if suite.GetContext() != ctx {
		t.Error("GetContext() returned wrong context")
	}
}

func TestSpatialTestSuite_TestShapes(t *testing.T) {
	suite := NewSpatialTestSuite()
	err := suite.TestShapes(t)
	if err != nil {
		t.Errorf("TestShapes() error = %v", err)
	}
}

func TestSpatialTestSuite_TestStrategies(t *testing.T) {
	suite := NewSpatialTestSuite()
	err := suite.TestStrategies(t)
	if err != nil {
		t.Errorf("TestStrategies() error = %v", err)
	}
}

func TestSpatialTestSuite_TestQueries(t *testing.T) {
	suite := NewSpatialTestSuite()
	err := suite.TestQueries(t)
	if err != nil {
		t.Errorf("TestQueries() error = %v", err)
	}
}

func TestSpatialTestSuite_TestOperations(t *testing.T) {
	suite := NewSpatialTestSuite()
	err := suite.TestOperations(t)
	if err != nil {
		t.Errorf("TestOperations() error = %v", err)
	}
}

func TestSpatialTestSuite_TestDistanceCalculations(t *testing.T) {
	suite := NewSpatialTestSuite()
	err := suite.TestDistanceCalculations(t)
	if err != nil {
		t.Errorf("TestDistanceCalculations() error = %v", err)
	}
}

func TestSpatialTestSuite_TestPointVectorIndexing(t *testing.T) {
	suite := NewSpatialTestSuite()
	err := suite.TestPointVectorIndexing(t)
	if err != nil {
		t.Errorf("TestPointVectorIndexing() error = %v", err)
	}
}

func TestSpatialTestSuite_TestBBoxIndexing(t *testing.T) {
	suite := NewSpatialTestSuite()
	err := suite.TestBBoxIndexing(t)
	if err != nil {
		t.Errorf("TestBBoxIndexing() error = %v", err)
	}
}

func TestSpatialTestSuite_TestPrefixTreeIndexing(t *testing.T) {
	suite := NewSpatialTestSuite()
	err := suite.TestPrefixTreeIndexing(t)
	if err != nil {
		t.Errorf("TestPrefixTreeIndexing() error = %v", err)
	}
}

func TestSpatialTestSuite_TestSpatialArgs(t *testing.T) {
	suite := NewSpatialTestSuite()
	err := suite.TestSpatialArgs(t)
	if err != nil {
		t.Errorf("TestSpatialArgs() error = %v", err)
	}
}

func TestSpatialTestSuite_TestSpatialIndexWriter(t *testing.T) {
	suite := NewSpatialTestSuite()
	err := suite.TestSpatialIndexWriter(t)
	if err != nil {
		t.Errorf("TestSpatialIndexWriter() error = %v", err)
	}
}

func TestSpatialTestSuite_TestSpatialIndexReader(t *testing.T) {
	suite := NewSpatialTestSuite()
	err := suite.TestSpatialIndexReader(t)
	if err != nil {
		t.Errorf("TestSpatialIndexReader() error = %v", err)
	}
}

func TestSpatialTestSuite_TestSpatialIndexFormat(t *testing.T) {
	suite := NewSpatialTestSuite()
	err := suite.TestSpatialIndexFormat(t)
	if err != nil {
		t.Errorf("TestSpatialIndexFormat() error = %v", err)
	}
}

func TestSpatialTestSuite_TestEdgeCases(t *testing.T) {
	suite := NewSpatialTestSuite()
	err := suite.TestEdgeCases(t)
	if err != nil {
		t.Errorf("TestEdgeCases() error = %v", err)
	}
}

func TestSpatialTestSuite_TestPerformance(t *testing.T) {
	suite := NewSpatialTestSuite()
	err := suite.TestPerformance(t)
	if err != nil {
		t.Errorf("TestPerformance() error = %v", err)
	}
}

func TestSpatialTestSuite_RunAllTests(t *testing.T) {
	suite := NewSpatialTestSuite()
	err := suite.RunAllTests(t)
	if err != nil {
		t.Errorf("RunAllTests() error = %v", err)
	}
}

func TestSpatialTestResult(t *testing.T) {
	result := NewSpatialTestResult()
	if result == nil {
		t.Error("NewSpatialTestResult() returned nil")
		return
	}

	// Test initial state
	if result.Passed != 0 {
		t.Errorf("Passed = %d, want 0", result.Passed)
	}
	if result.Failed != 0 {
		t.Errorf("Failed = %d, want 0", result.Failed)
	}
	if result.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", result.Skipped)
	}

	// Test recording passes
	result.RecordPass()
	result.RecordPass()
	if result.Passed != 2 {
		t.Errorf("Passed = %d, want 2", result.Passed)
	}

	// Test recording failures
	result.RecordFail("test failure 1")
	result.RecordFail("test failure 2")
	if result.Failed != 2 {
		t.Errorf("Failed = %d, want 2", result.Failed)
	}
	if len(result.Errors) != 2 {
		t.Errorf("len(Errors) = %d, want 2", len(result.Errors))
	}

	// Test recording skips
	result.RecordSkip()
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}

	// Test String method
	str := result.String()
	expected := "Passed: 2, Failed: 2, Skipped: 1"
	if str != expected {
		t.Errorf("String() = %s, want %s", str, expected)
	}
}

func TestSpatialTestSuite_Integration(t *testing.T) {
	// This test runs the entire test suite to ensure all components work together
	suite := NewSpatialTestSuite()

	// Run all tests
	err := suite.RunAllTests(t)
	if err != nil {
		t.Fatalf("RunAllTests() failed: %v", err)
	}

	// Run individual test categories
	tests := []struct {
		name string
		fn   func(*testing.T) error
	}{
		{"Shapes", suite.TestShapes},
		{"Strategies", suite.TestStrategies},
		{"Queries", suite.TestQueries},
		{"Operations", suite.TestOperations},
		{"DistanceCalculations", suite.TestDistanceCalculations},
		{"PointVectorIndexing", suite.TestPointVectorIndexing},
		{"BBoxIndexing", suite.TestBBoxIndexing},
		{"PrefixTreeIndexing", suite.TestPrefixTreeIndexing},
		{"SpatialArgs", suite.TestSpatialArgs},
		{"EdgeCases", suite.TestEdgeCases},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn(t)
			if err != nil {
				t.Errorf("%s test failed: %v", tc.name, err)
			}
		})
	}
}

func TestSpatialTestSuite_WithCartesianContext(t *testing.T) {
	// Test with Cartesian context
	ctx := NewSpatialContextCartesian(0, 0, 1000, 1000)
	suite := NewSpatialTestSuiteWithContext(ctx)

	// Test shapes in Cartesian space
	err := suite.TestShapes(t)
	if err != nil {
		t.Errorf("TestShapes() with Cartesian context error = %v", err)
	}

	// Test distance calculations in Cartesian space
	err = suite.TestDistanceCalculations(t)
	if err != nil {
		t.Errorf("TestDistanceCalculations() with Cartesian context error = %v", err)
	}
}

func TestSpatialTestSuite_Concurrent(t *testing.T) {
	// Test concurrent access to the test suite
	suite := NewSpatialTestSuite()

	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			suite.TestShapes(t)
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}
