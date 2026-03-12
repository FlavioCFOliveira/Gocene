// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for merge policy implementations.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestTieredMergePolicy
// and related test files:
//   - TestTieredMergePolicy.java
//   - TestMergePolicy.java
//
// GC-116: Index Tests - IndexWriter Merging
package index_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestTieredMergePolicy tests the TieredMergePolicy implementation.
// Ported from: TestTieredMergePolicy.java
func TestTieredMergePolicy(t *testing.T) {
	t.Run("new tiered merge policy has defaults", func(t *testing.T) {
		policy := index.NewTieredMergePolicy()
		if policy == nil {
			t.Fatal("NewTieredMergePolicy() returned nil")
		}

		// Verify default values
		if policy.GetMaxMergeAtOnce() != 10 {
			t.Errorf("GetMaxMergeAtOnce() = %d, want 10", policy.GetMaxMergeAtOnce())
		}
		if policy.GetMaxMergeAtOnceExplicit() != 30 {
			t.Errorf("GetMaxMergeAtOnceExplicit() = %d, want 30", policy.GetMaxMergeAtOnceExplicit())
		}
		if policy.GetMaxMergedSegmentMB() != 5120 {
			t.Errorf("GetMaxMergedSegmentMB() = %d, want 5120", policy.GetMaxMergedSegmentMB())
		}
		if policy.GetMaxMergeDocs() != math.MaxInt32 {
			t.Errorf("GetMaxMergeDocs() = %d, want MaxInt32", policy.GetMaxMergeDocs())
		}
	})

	t.Run("set and get max merge at once", func(t *testing.T) {
		policy := index.NewTieredMergePolicy()
		policy.SetMaxMergeAtOnce(5)
		if policy.GetMaxMergeAtOnce() != 5 {
			t.Errorf("GetMaxMergeAtOnce() = %d, want 5", policy.GetMaxMergeAtOnce())
		}
	})

	t.Run("set and get max merge at once explicit", func(t *testing.T) {
		policy := index.NewTieredMergePolicy()
		policy.SetMaxMergeAtOnceExplicit(20)
		if policy.GetMaxMergeAtOnceExplicit() != 20 {
			t.Errorf("GetMaxMergeAtOnceExplicit() = %d, want 20", policy.GetMaxMergeAtOnceExplicit())
		}
	})

	t.Run("set and get max merged segment MB", func(t *testing.T) {
		policy := index.NewTieredMergePolicy()
		policy.SetMaxMergedSegmentMB(1024)
		if policy.GetMaxMergedSegmentMB() != 1024 {
			t.Errorf("GetMaxMergedSegmentMB() = %d, want 1024", policy.GetMaxMergedSegmentMB())
		}
	})

	t.Run("set and get max merge docs", func(t *testing.T) {
		policy := index.NewTieredMergePolicy()
		policy.SetMaxMergeDocs(1000000)
		if policy.GetMaxMergeDocs() != 1000000 {
			t.Errorf("GetMaxMergeDocs() = %d, want 1000000", policy.GetMaxMergeDocs())
		}
	})

	t.Run("set and get max merged segment bytes", func(t *testing.T) {
		policy := index.NewTieredMergePolicy()
		policy.SetMaxMergedSegmentBytes(1024 * 1024 * 1024) // 1GB
		if policy.GetMaxMergedSegmentBytes() != 1024*1024*1024 {
			t.Errorf("GetMaxMergedSegmentBytes() = %d, want 1GB", policy.GetMaxMergedSegmentBytes())
		}
	})
}

// TestMergeSpecification tests the MergeSpecification implementation.
func TestMergeSpecification(t *testing.T) {
	t.Run("new merge specification is empty", func(t *testing.T) {
		spec := index.NewMergeSpecification()
		if spec == nil {
			t.Fatal("NewMergeSpecification() returned nil")
		}
		if spec.Size() != 0 {
			t.Errorf("Size() = %d, want 0", spec.Size())
		}
	})

	t.Run("add merge to specification", func(t *testing.T) {
		spec := index.NewMergeSpecification()

		// Create a mock merge
		segments := []*index.SegmentCommitInfo{}
		merge := index.NewOneMerge(segments)

		spec.Add(merge)
		if spec.Size() != 1 {
			t.Errorf("Size() = %d, want 1", spec.Size())
		}
	})

	t.Run("string representation", func(t *testing.T) {
		spec := index.NewMergeSpecification()
		s := spec.String()
		if s == "" {
			t.Error("String() returned empty string")
		}
		t.Logf("MergeSpecification string: %s", s)
	})
}

// TestOneMerge tests the OneMerge implementation.
func TestOneMerge(t *testing.T) {
	t.Run("new one merge with segments", func(t *testing.T) {
		// Create segment commit info list
		segments := []*index.SegmentCommitInfo{}

		merge := index.NewOneMerge(segments)
		if merge == nil {
			t.Fatal("NewOneMerge() returned nil")
		}

		if merge.SegmentsSize() != 0 {
			t.Errorf("SegmentsSize() = %d, want 0", merge.SegmentsSize())
		}
	})

	t.Run("string representation", func(t *testing.T) {
		segments := []*index.SegmentCommitInfo{}
		merge := index.NewOneMerge(segments)

		s := merge.String()
		if s == "" {
			t.Error("String() returned empty string")
		}
		t.Logf("OneMerge string: %s", s)
	})
}

// TestTieredMergePolicyFindMerges tests merge finding logic.
// Ported from: TestTieredMergePolicy.testForceMerge()
func TestTieredMergePolicyFindMerges(t *testing.T) {
	t.Run("find merges with empty segments", func(t *testing.T) {
		policy := index.NewTieredMergePolicy()

		// Create empty SegmentInfos
		infos := index.NewSegmentInfos()

		spec, err := policy.FindMerges(index.SEGMENT_FLUSH, infos)
		if err != nil {
			t.Errorf("FindMerges() error = %v", err)
		}

		// With no segments, should return nil (no merges needed)
		// or empty specification depending on implementation
		if spec != nil && spec.Size() != 0 {
			t.Errorf("Expected nil or empty spec for empty segments, got %v", spec)
		}
	})

	t.Run("find forced merges with empty segments", func(t *testing.T) {
		policy := index.NewTieredMergePolicy()
		infos := index.NewSegmentInfos()

		spec, err := policy.FindForcedMerges(infos, 1)
		if err != nil {
			t.Errorf("FindForcedMerges() error = %v", err)
		}

		// With no segments, should return nil or empty
		if spec != nil && spec.Size() != 0 {
			t.Errorf("Expected nil or empty spec for empty segments, got %v", spec)
		}
	})

	t.Run("find forced deletes merges with empty segments", func(t *testing.T) {
		policy := index.NewTieredMergePolicy()
		infos := index.NewSegmentInfos()

		spec, err := policy.FindForcedDeletesMerges(infos)
		if err != nil {
			t.Errorf("FindForcedDeletesMerges() error = %v", err)
		}

		// With no segments, should return nil or empty
		if spec != nil && spec.Size() != 0 {
			t.Errorf("Expected nil or empty spec for empty segments, got %v", spec)
		}
	})
}

// TestMergePolicyConfig tests the merge policy configuration.
func TestMergePolicyConfig(t *testing.T) {
	t.Run("default merge policy config", func(t *testing.T) {
		config := index.DefaultMergePolicyConfig()

		if config.MaxMergeAtOnce != 10 {
			t.Errorf("MaxMergeAtOnce = %d, want 10", config.MaxMergeAtOnce)
		}
		if config.MaxMergeAtOnceExplicit != 30 {
			t.Errorf("MaxMergeAtOnceExplicit = %d, want 30", config.MaxMergeAtOnceExplicit)
		}
		if config.MaxMergedSegmentMB != 5120 {
			t.Errorf("MaxMergedSegmentMB = %d, want 5120", config.MaxMergedSegmentMB)
		}
		if config.FloorSegmentMB != 2 {
			t.Errorf("FloorSegmentMB = %d, want 2", config.FloorSegmentMB)
		}
	})
}

// TestMergeTrigger tests merge trigger types.
func TestMergeTrigger(t *testing.T) {
	t.Run("merge trigger string representations", func(t *testing.T) {
		triggers := []index.MergeTrigger{
			index.SEGMENT_FLUSH,
			index.CLOSED_WRITER,
			index.EXPLICIT,
			index.MERGE_FINISHED,
			index.COMMIT,
			index.FULL_FLUSH,
			index.GET_READER,
		}

		expectedNames := []string{
			"SEGMENT_FLUSH",
			"CLOSED_WRITER",
			"EXPLICIT",
			"MERGE_FINISHED",
			"COMMIT",
			"FULL_FLUSH",
			"GET_READER",
		}

		for i, trigger := range triggers {
			s := trigger.String()
			if s != expectedNames[i] {
				t.Errorf("Trigger %d String() = %s, want %s", i, s, expectedNames[i])
			}
		}
	})

	t.Run("unknown merge trigger", func(t *testing.T) {
		// Create an unknown trigger value
		unknownTrigger := index.MergeTrigger(999)
		s := unknownTrigger.String()
		if s == "" {
			t.Error("String() should not return empty for unknown trigger")
		}
		// Should contain UNKNOWN
		if s[:7] != "UNKNOWN" {
			t.Errorf("String() should start with UNKNOWN, got %s", s)
		}
	})
}

// TestBaseMergePolicy tests the base merge policy functionality.
func TestBaseMergePolicy(t *testing.T) {
	t.Run("base merge policy defaults", func(t *testing.T) {
		policy := index.NewBaseMergePolicy()

		if policy.GetMaxMergeDocs() != math.MaxInt32 {
			t.Errorf("GetMaxMergeDocs() = %d, want MaxInt32", policy.GetMaxMergeDocs())
		}

		// Default max merged segment bytes is 5GB
		expectedBytes := int64(5 * 1024 * 1024 * 1024)
		if policy.GetMaxMergedSegmentBytes() != expectedBytes {
			t.Errorf("GetMaxMergedSegmentBytes() = %d, want %d", policy.GetMaxMergedSegmentBytes(), expectedBytes)
		}
	})

	t.Run("base merge policy not implemented methods", func(t *testing.T) {
		policy := index.NewBaseMergePolicy()
		infos := index.NewSegmentInfos()

		_, err := policy.FindMerges(index.SEGMENT_FLUSH, infos)
		if err == nil {
			t.Error("FindMerges should return error for base implementation")
		}

		_, err = policy.FindForcedMerges(infos, 1)
		if err == nil {
			t.Error("FindForcedMerges should return error for base implementation")
		}

		_, err = policy.FindForcedDeletesMerges(infos)
		if err == nil {
			t.Error("FindForcedDeletesMerges should return error for base implementation")
		}
	})

	t.Run("use compound file default", func(t *testing.T) {
		policy := index.NewBaseMergePolicy()
		infos := index.NewSegmentInfos()

		// Create a dummy segment info
		si := index.NewSegmentInfo("test", 100, nil)
		useCFS := policy.UseCompoundFile(infos, si)
		if useCFS {
			t.Error("UseCompoundFile should return false by default")
		}
	})
}

// TestTieredMergePolicyAdvanced tests advanced TieredMergePolicy scenarios.
func TestTieredMergePolicyAdvanced(t *testing.T) {
	t.Run("floor segment MB setting", func(t *testing.T) {
		policy := index.NewTieredMergePolicy()
		policy.SetFloorSegmentMB(10)
		if policy.GetFloorSegmentMB() != 10 {
			t.Errorf("GetFloorSegmentMB() = %d, want 10", policy.GetFloorSegmentMB())
		}
	})

	t.Run("noCFS ratio setting", func(t *testing.T) {
		policy := index.NewTieredMergePolicy()
		policy.SetNoCFSRatio(0.5)
		if policy.GetNoCFSRatio() != 0.5 {
			t.Errorf("GetNoCFSRatio() = %f, want 0.5", policy.GetNoCFSRatio())
		}
	})

	t.Run("deletes percentage allowed setting", func(t *testing.T) {
		policy := index.NewTieredMergePolicy()
		policy.SetDeletesPctAllowed(50.0)
		if policy.GetDeletesPctAllowed() != 50.0 {
			t.Errorf("GetDeletesPctAllowed() = %f, want 50.0", policy.GetDeletesPctAllowed())
		}
	})

	t.Run("tier exponent setting", func(t *testing.T) {
		policy := index.NewTieredMergePolicy()
		policy.SetTierExponent(0.75)
		if policy.GetTierExponent() != 0.75 {
			t.Errorf("GetTierExponent() = %f, want 0.75", policy.GetTierExponent())
		}
	})
}
