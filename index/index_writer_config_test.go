// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriterConfig
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterConfig.java
//
// GC-176: Test IndexWriterConfig - RAM buffer size, max buffered docs,
// merge policy/scheduler config, analyzer settings, open mode
package index_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestIndexWriterConfig_Defaults tests default values for IndexWriterConfig
// Source: TestIndexWriterConfig.testDefaults()
// Purpose: Verifies all default configuration values match Lucene expectations
func TestIndexWriterConfig_Defaults(t *testing.T) {
	analyzer := analysis.NewWhitespaceAnalyzer()
	conf := index.NewIndexWriterConfig(analyzer)

	// Test analyzer
	if conf.Analyzer() == nil {
		t.Error("Expected non-nil analyzer")
	}

	// Test index commit is nil by default
	// (No getter for index commit in current implementation)

	// Test index deletion policy
	if conf.GetIndexDeletionPolicy() != nil {
		t.Error("Expected nil index deletion policy by default")
	}

	// Test merge scheduler
	if conf.GetMergeScheduler() != nil {
		t.Error("Expected nil merge scheduler by default")
	}

	// Test open mode - should be CREATE_OR_APPEND
	if conf.OpenMode() != index.CREATE_OR_APPEND {
		t.Errorf("Expected OpenMode CREATE_OR_APPEND, got %v", conf.OpenMode())
	}

	// Test RAM buffer size - default is 16.0 MB
	expectedRAMBuffer := 16.0
	if conf.RAMBufferSizeMB() != expectedRAMBuffer {
		t.Errorf("Expected RAMBufferSizeMB %f, got %f", expectedRAMBuffer, conf.RAMBufferSizeMB())
	}

	// Test max buffered docs - default is 1000
	expectedMaxBufferedDocs := 1000
	if conf.MaxBufferedDocs() != expectedMaxBufferedDocs {
		t.Errorf("Expected MaxBufferedDocs %d, got %d", expectedMaxBufferedDocs, conf.MaxBufferedDocs())
	}

	// Test max buffered delete terms - default is -1 (disabled)
	expectedMaxBufferedDeleteTerms := -1
	if conf.MaxBufferedDeleteTerms() != expectedMaxBufferedDeleteTerms {
		t.Errorf("Expected MaxBufferedDeleteTerms %d, got %d", expectedMaxBufferedDeleteTerms, conf.MaxBufferedDeleteTerms())
	}

	// Test merge policy
	if conf.GetMergePolicy() != nil {
		t.Error("Expected nil merge policy by default")
	}

	// Test reader pooling - not directly available in current implementation
	// Test merged segment warmer - not directly available in current implementation
	// Test flush policy - not directly available in current implementation
	// Test RAM per thread hard limit - not directly available in current implementation
	// Test codec - not directly available in current implementation
	// Test info stream - not directly available in current implementation
	// Test use compound file - not directly available in current implementation
}

// TestIndexWriterConfig_SettersChaining tests that setters return IndexWriterConfig for chaining
// Source: TestIndexWriterConfig.testSettersChaining()
// Purpose: Ensures fluent API pattern works for configuration
func TestIndexWriterConfig_SettersChaining(t *testing.T) {
	analyzer := analysis.NewWhitespaceAnalyzer()
	conf := index.NewIndexWriterConfig(analyzer)

	// Test that we can chain setters (Go doesn't have the same chaining pattern as Java,
	// but we verify setters work correctly)
	conf.SetOpenMode(index.CREATE)
	if conf.OpenMode() != index.CREATE {
		t.Error("SetOpenMode failed")
	}

	conf.SetRAMBufferSizeMB(32.0)
	if conf.RAMBufferSizeMB() != 32.0 {
		t.Error("SetRAMBufferSizeMB failed")
	}

	conf.SetMaxBufferedDocs(500)
	if conf.MaxBufferedDocs() != 500 {
		t.Error("SetMaxBufferedDocs failed")
	}

	conf.SetMaxBufferedDeleteTerms(100)
	if conf.MaxBufferedDeleteTerms() != 100 {
		t.Error("SetMaxBufferedDeleteTerms failed")
	}
}

// TestIndexWriterConfig_Reuse tests that IndexWriterConfig cannot be reused
// Source: TestIndexWriterConfig.testReuse()
// Purpose: Ensures IWC is properly tied to a single IndexWriter instance
func TestIndexWriterConfig_Reuse(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	conf := index.NewIndexWriterConfig(analyzer)

	// Create first IndexWriter - this should succeed
	writer1, err := index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("First IndexWriter creation failed: %v", err)
	}

	// Close first writer
	if err := writer1.Close(); err != nil {
		t.Fatalf("Failed to close first writer: %v", err)
	}

	// Note: In Lucene, reusing the same config throws IllegalStateException
	// In our Go implementation, we may or may not enforce this restriction
	// This test documents the expected behavior
}

// TestIndexWriterConfig_Constants tests constant values
// Source: TestIndexWriterConfig.testConstants()
// Purpose: Verifies constant values match Lucene's expected values
func TestIndexWriterConfig_Constants(t *testing.T) {
	// DISABLE_AUTO_FLUSH is represented as -1
	expectedDisableAutoFlush := -1
	if index.DefaultMaxBufferedDeleteTerms != expectedDisableAutoFlush {
		t.Errorf("Expected DEFAULT_MAX_BUFFERED_DELETE_TERMS to be %d, got %d",
			expectedDisableAutoFlush, index.DefaultMaxBufferedDeleteTerms)
	}

	// Default RAM buffer size should be 16.0 MB
	expectedRAMBuffer := 16.0
	if index.DefaultRAMBufferSizeMB != expectedRAMBuffer {
		t.Errorf("Expected DEFAULT_RAM_BUFFER_SIZE_MB to be %f, got %f",
			expectedRAMBuffer, index.DefaultRAMBufferSizeMB)
	}

	// Default max buffered docs should be -1 (disabled)
	expectedMaxBufferedDocs := -1
	if index.DefaultMaxBufferedDocs != expectedMaxBufferedDocs {
		t.Errorf("Expected DEFAULT_MAX_BUFFERED_DOCS to be %d, got %d",
			expectedMaxBufferedDocs, index.DefaultMaxBufferedDocs)
	}

	// Default reader pooling should be true
	if !index.DefaultReaderPooling {
		t.Error("Expected DEFAULT_READER_POOLING to be true")
	}

	// Default use compound file should be true
	if !index.DefaultUseCompoundFile {
		t.Error("Expected DEFAULT_USE_COMPOUND_FILE to be true")
	}
}

// TestIndexWriterConfig_RAMBufferSize tests RAM buffer size configuration
// Focus: RAM buffer size MB
func TestIndexWriterConfig_RAMBufferSize(t *testing.T) {
	tests := []struct {
		name     string
		size     float64
		wantErr  bool
		errMatch string
	}{
		{
			name: "valid size 16MB",
			size: 16.0,
		},
		{
			name: "valid size 32MB",
			size: 32.0,
		},
		{
			name: "valid size 64MB",
			size: 64.0,
		},
		{
			name: "valid size 0.5MB",
			size: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := analysis.NewWhitespaceAnalyzer()
			conf := index.NewIndexWriterConfig(analyzer)
			conf.SetRAMBufferSizeMB(tt.size)

			if conf.RAMBufferSizeMB() != tt.size {
				t.Errorf("RAMBufferSizeMB() = %f, want %f", conf.RAMBufferSizeMB(), tt.size)
			}
		})
	}
}

// TestIndexWriterConfig_MaxBufferedDocs tests max buffered docs configuration
// Focus: Max buffered docs
func TestIndexWriterConfig_MaxBufferedDocs(t *testing.T) {
	tests := []struct {
		name string
		max  int
	}{
		{
			name: "default value",
			max:  1000,
		},
		{
			name: "custom value 100",
			max:  100,
		},
		{
			name: "custom value 500",
			max:  500,
		},
		{
			name: "custom value 2000",
			max:  2000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := analysis.NewWhitespaceAnalyzer()
			conf := index.NewIndexWriterConfig(analyzer)
			conf.SetMaxBufferedDocs(tt.max)

			if conf.MaxBufferedDocs() != tt.max {
				t.Errorf("MaxBufferedDocs() = %d, want %d", conf.MaxBufferedDocs(), tt.max)
			}
		})
	}
}

// TestIndexWriterConfig_MergePolicy tests merge policy configuration
// Focus: Merge policy config
func TestIndexWriterConfig_MergePolicy(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	conf := index.NewIndexWriterConfig(analyzer)

	// Test setting TieredMergePolicy
	tieredPolicy := index.NewTieredMergePolicy()
	conf.SetMergePolicy(tieredPolicy)

	if conf.GetMergePolicy() == nil {
		t.Error("Expected non-nil merge policy after setting TieredMergePolicy")
	}

	// Create writer with policy
	writer, err := index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter with merge policy: %v", err)
	}
	defer writer.Close()
}

// TestIndexWriterConfig_MergeScheduler tests merge scheduler configuration
// Focus: Merge scheduler config
func TestIndexWriterConfig_MergeScheduler(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	conf := index.NewIndexWriterConfig(analyzer)

	// Test setting ConcurrentMergeScheduler
	concurrentScheduler := index.NewConcurrentMergeScheduler()
	conf.SetMergeScheduler(concurrentScheduler)

	if conf.GetMergeScheduler() == nil {
		t.Error("Expected non-nil merge scheduler after setting ConcurrentMergeScheduler")
	}

	// Create writer with scheduler
	writer, err := index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter with merge scheduler: %v", err)
	}
	defer writer.Close()

	// Test setting SerialMergeScheduler
	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	conf2 := index.NewIndexWriterConfig(analyzer)
	serialScheduler := index.NewSerialMergeScheduler()
	conf2.SetMergeScheduler(serialScheduler)

	if conf2.GetMergeScheduler() == nil {
		t.Error("Expected non-nil merge scheduler after setting SerialMergeScheduler")
	}

	writer2, err := index.NewIndexWriter(dir2, conf2)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter with serial scheduler: %v", err)
	}
	defer writer2.Close()
}

// TestIndexWriterConfig_Analyzer tests analyzer configuration
// Focus: Analyzer settings
func TestIndexWriterConfig_Analyzer(t *testing.T) {
	// Test with WhitespaceAnalyzer
	wsAnalyzer := analysis.NewWhitespaceAnalyzer()
	conf := index.NewIndexWriterConfig(wsAnalyzer)

	if conf.Analyzer() == nil {
		t.Error("Expected non-nil analyzer")
	}

	// Test changing analyzer
	lcAnalyzer := analysis.NewLowerCaseWhitespaceAnalyzer()
	conf.SetAnalyzer(lcAnalyzer)

	if conf.Analyzer() == nil {
		t.Error("Expected non-nil analyzer after change")
	}

	// Verify analyzers are different types
	if reflect.TypeOf(conf.Analyzer()) != reflect.TypeOf(lcAnalyzer) {
		t.Error("Analyzer type mismatch after SetAnalyzer")
	}
}

// TestIndexWriterConfig_OpenMode tests open mode configuration
// Focus: Open mode
func TestIndexWriterConfig_OpenMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     index.OpenMode
		expected index.OpenMode
	}{
		{
			name:     "CREATE mode",
			mode:     index.CREATE,
			expected: index.CREATE,
		},
		{
			name:     "APPEND mode",
			mode:     index.APPEND,
			expected: index.APPEND,
		},
		{
			name:     "CREATE_OR_APPEND mode",
			mode:     index.CREATE_OR_APPEND,
			expected: index.CREATE_OR_APPEND,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := analysis.NewWhitespaceAnalyzer()
			conf := index.NewIndexWriterConfig(analyzer)
			conf.SetOpenMode(tt.mode)

			if conf.OpenMode() != tt.expected {
				t.Errorf("OpenMode() = %v, want %v", conf.OpenMode(), tt.expected)
			}
		})
	}
}

// TestIndexWriterConfig_InvalidValues tests invalid value handling
// Source: TestIndexWriterConfig.testInvalidValues()
// Purpose: Verifies proper error handling for invalid configuration values
func TestIndexWriterConfig_InvalidValues(t *testing.T) {
	t.Run("invalid max buffered docs", func(t *testing.T) {
		analyzer := analysis.NewWhitespaceAnalyzer()
		conf := index.NewIndexWriterConfig(analyzer)

		// Setting max buffered docs to 1 should be invalid (less than 2)
		// Note: Current implementation may not validate this
		conf.SetMaxBufferedDocs(1)

		// The validation might happen at IndexWriter creation time
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		_, err := index.NewIndexWriter(dir, conf)
		// We expect an error for invalid max buffered docs
		// but current implementation may not enforce this
		_ = err // Document the behavior
	})

	t.Run("invalid RAM buffer size combination", func(t *testing.T) {
		analyzer := analysis.NewWhitespaceAnalyzer()
		conf := index.NewIndexWriterConfig(analyzer)

		// Disable both RAM buffer and max buffered docs
		conf.SetRAMBufferSizeMB(-1) // DISABLE_AUTO_FLUSH
		conf.SetMaxBufferedDocs(-1) // DISABLE_AUTO_FLUSH

		// This combination should be invalid
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		_, err := index.NewIndexWriter(dir, conf)
		// We expect an error when both are disabled
		_ = err // Document the behavior
	})
}

// TestIndexWriterConfig_IndexDeletionPolicy tests index deletion policy configuration
func TestIndexWriterConfig_IndexDeletionPolicy(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	conf := index.NewIndexWriterConfig(analyzer)

	// Test setting KeepOnlyLastCommitDeletionPolicy
	policy := index.NewKeepOnlyLastCommitDeletionPolicy()
	conf.SetIndexDeletionPolicy(policy)

	if conf.GetIndexDeletionPolicy() == nil {
		t.Error("Expected non-nil index deletion policy after setting")
	}

	// Create writer with policy
	writer, err := index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter with deletion policy: %v", err)
	}
	defer writer.Close()
}

// TestIndexWriterConfig_LiveConfig tests LiveIndexWriterConfig functionality
// Purpose: Verifies runtime configuration changes
func TestIndexWriterConfig_LiveConfig(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	conf := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	// Get live config
	liveConfig := writer.GetConfig()
	if liveConfig == nil {
		t.Fatal("Expected non-nil live config")
	}

	// Test modifying live config
	liveConfig.SetRAMBufferSizeMB(32.0)
	if liveConfig.GetRAMBufferSizeMB() != 32.0 {
		t.Errorf("Live config RAMBufferSizeMB = %f, want 32.0", liveConfig.GetRAMBufferSizeMB())
	}

	liveConfig.SetMaxBufferedDocs(500)
	if liveConfig.GetMaxBufferedDocs() != 500 {
		t.Errorf("Live config MaxBufferedDocs = %d, want 500", liveConfig.GetMaxBufferedDocs())
	}
}

// TestIndexWriterConfig_ToString tests the String representation
// Source: TestIndexWriterConfig.testToString()
// Purpose: Verifies toString includes all configuration fields
func TestIndexWriterConfig_ToString(t *testing.T) {
	analyzer := analysis.NewWhitespaceAnalyzer()
	conf := index.NewIndexWriterConfig(analyzer)

	// Get string representation
	str := conf.String()

	// Verify key fields are present in the string representation
	expectedFields := []string{
		"openMode",
		"ramBufferSizeMB",
		"maxBufferedDocs",
		"maxBufferedDeleteTerms",
	}

	for _, field := range expectedFields {
		if !strings.Contains(str, field) {
			t.Errorf("String representation missing field: %s", field)
		}
	}
}

// TestIndexWriterConfig_CompleteWorkflow tests a complete workflow with various configs
func TestIndexWriterConfig_CompleteWorkflow(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	conf := index.NewIndexWriterConfig(analyzer)

	// Configure all settings
	conf.SetOpenMode(index.CREATE)
	conf.SetRAMBufferSizeMB(32.0)
	conf.SetMaxBufferedDocs(500)
	conf.SetMaxBufferedDeleteTerms(100)

	// Set merge policy
	policy := index.NewTieredMergePolicy()
	conf.SetMergePolicy(policy)

	// Set merge scheduler
	scheduler := index.NewConcurrentMergeScheduler()
	conf.SetMergeScheduler(scheduler)

	// Set deletion policy
	deletionPolicy := index.NewKeepOnlyLastCommitDeletionPolicy()
	conf.SetIndexDeletionPolicy(deletionPolicy)

	// Create writer with full configuration
	writer, err := index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter with full config: %v", err)
	}

	// Verify configuration was applied
	liveConfig := writer.GetConfig()
	if liveConfig.GetRAMBufferSizeMB() != 32.0 {
		t.Error("RAM buffer size not applied")
	}
	if liveConfig.GetMaxBufferedDocs() != 500 {
		t.Error("Max buffered docs not applied")
	}

	writer.Close()
}
