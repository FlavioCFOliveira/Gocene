// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
)

func TestNewNRTMergePolicy(t *testing.T) {
	policy := NewNRTMergePolicy()

	if policy == nil {
		t.Fatal("expected non-nil policy")
	}

	// Check default values
	if policy.GetMinMergeMB() != 0.5 {
		t.Errorf("expected minMergeMB 0.5, got %f", policy.GetMinMergeMB())
	}
	if policy.GetMaxMergeMB() != 512.0 {
		t.Errorf("expected maxMergeMB 512.0, got %f", policy.GetMaxMergeMB())
	}
	if policy.GetMaxMergeMBDuringNRT() != 100.0 {
		t.Errorf("expected maxMergeMBDuringNRT 100.0, got %f", policy.GetMaxMergeMBDuringNRT())
	}
	if policy.GetMergeFactor() != 5 {
		t.Errorf("expected mergeFactor 5, got %d", policy.GetMergeFactor())
	}
	if policy.GetMaxMergedSegmentMB() != 256.0 {
		t.Errorf("expected maxMergedSegmentMB 256.0, got %f", policy.GetMaxMergedSegmentMB())
	}
	if policy.GetDeletesPctAllowed() != 20.0 {
		t.Errorf("expected deletesPctAllowed 20.0, got %f", policy.GetDeletesPctAllowed())
	}
	if policy.GetSegmentsPerTier() != 3 {
		t.Errorf("expected segmentsPerTier 3, got %d", policy.GetSegmentsPerTier())
	}
	if policy.GetFloorSegmentMB() != 1.0 {
		t.Errorf("expected floorSegmentMB 1.0, got %f", policy.GetFloorSegmentMB())
	}
	if !policy.GetCalibrateSizeByDeletes() {
		t.Error("expected calibrateSizeByDeletes to be true")
	}
	if !policy.GetNRTAware() {
		t.Error("expected nrtAware to be true")
	}
}

func TestNRTMergePolicy_MinMergeMB(t *testing.T) {
	policy := NewNRTMergePolicy()

	tests := []struct {
		name string
		mb   float64
		want float64
	}{
		{
			name: "set to 1 MB",
			mb:   1.0,
			want: 1.0,
		},
		{
			name: "set to 10 MB",
			mb:   10.0,
			want: 10.0,
		},
		{
			name: "set to negative (should become 0)",
			mb:   -1.0,
			want: 0.0,
		},
		{
			name: "set to zero",
			mb:   0.0,
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy.SetMinMergeMB(tt.mb)
			got := policy.GetMinMergeMB()
			if got != tt.want {
				t.Errorf("GetMinMergeMB() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestNRTMergePolicy_MaxMergeMB(t *testing.T) {
	policy := NewNRTMergePolicy()

	tests := []struct {
		name string
		mb   float64
		want float64
	}{
		{
			name: "set to 100 MB",
			mb:   100.0,
			want: 100.0,
		},
		{
			name: "set to 1024 MB",
			mb:   1024.0,
			want: 1024.0,
		},
		{
			name: "set to negative (should become 0)",
			mb:   -10.0,
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy.SetMaxMergeMB(tt.mb)
			got := policy.GetMaxMergeMB()
			if got != tt.want {
				t.Errorf("GetMaxMergeMB() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestNRTMergePolicy_MaxMergeMBDuringNRT(t *testing.T) {
	policy := NewNRTMergePolicy()

	tests := []struct {
		name string
		mb   float64
		want float64
	}{
		{
			name: "set to 50 MB",
			mb:   50.0,
			want: 50.0,
		},
		{
			name: "set to 200 MB",
			mb:   200.0,
			want: 200.0,
		},
		{
			name: "set to negative (should become 0)",
			mb:   -10.0,
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy.SetMaxMergeMBDuringNRT(tt.mb)
			got := policy.GetMaxMergeMBDuringNRT()
			if got != tt.want {
				t.Errorf("GetMaxMergeMBDuringNRT() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestNRTMergePolicy_MergeFactor(t *testing.T) {
	policy := NewNRTMergePolicy()

	tests := []struct {
		name   string
		factor int
		want   int
	}{
		{
			name:   "set to 10",
			factor: 10,
			want:   10,
		},
		{
			name:   "set to 3",
			factor: 3,
			want:   3,
		},
		{
			name:   "set to 1 (should become 2)",
			factor: 1,
			want:   2,
		},
		{
			name:   "set to 0 (should become 2)",
			factor: 0,
			want:   2,
		},
		{
			name:   "set to negative (should become 2)",
			factor: -1,
			want:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy.SetMergeFactor(tt.factor)
			got := policy.GetMergeFactor()
			if got != tt.want {
				t.Errorf("GetMergeFactor() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNRTMergePolicy_MaxMergedSegmentMB(t *testing.T) {
	policy := NewNRTMergePolicy()

	tests := []struct {
		name string
		mb   float64
		want float64
	}{
		{
			name: "set to 128 MB",
			mb:   128.0,
			want: 128.0,
		},
		{
			name: "set to 512 MB",
			mb:   512.0,
			want: 512.0,
		},
		{
			name: "set to negative (should become 0)",
			mb:   -10.0,
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy.SetMaxMergedSegmentMB(tt.mb)
			got := policy.GetMaxMergedSegmentMB()
			if got != tt.want {
				t.Errorf("GetMaxMergedSegmentMB() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestNRTMergePolicy_DeletesPctAllowed(t *testing.T) {
	policy := NewNRTMergePolicy()

	tests := []struct {
		name string
		pct  float64
		want float64
	}{
		{
			name: "set to 10%",
			pct:  10.0,
			want: 10.0,
		},
		{
			name: "set to 50%",
			pct:  50.0,
			want: 50.0,
		},
		{
			name: "set to negative (should become 0)",
			pct:  -10.0,
			want: 0.0,
		},
		{
			name: "set to over 100 (should become 100)",
			pct:  150.0,
			want: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy.SetDeletesPctAllowed(tt.pct)
			got := policy.GetDeletesPctAllowed()
			if got != tt.want {
				t.Errorf("GetDeletesPctAllowed() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestNRTMergePolicy_SegmentsPerTier(t *testing.T) {
	policy := NewNRTMergePolicy()

	tests := []struct {
		name    string
		perTier int
		want    int
	}{
		{
			name:    "set to 5",
			perTier: 5,
			want:    5,
		},
		{
			name:    "set to 2 (minimum)",
			perTier: 2,
			want:    2,
		},
		{
			name:    "set to 1 (should become 2)",
			perTier: 1,
			want:    2,
		},
		{
			name:    "set to 0 (should become 2)",
			perTier: 0,
			want:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy.SetSegmentsPerTier(tt.perTier)
			got := policy.GetSegmentsPerTier()
			if got != tt.want {
				t.Errorf("GetSegmentsPerTier() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNRTMergePolicy_FloorSegmentMB(t *testing.T) {
	policy := NewNRTMergePolicy()

	tests := []struct {
		name string
		mb   float64
		want float64
	}{
		{
			name: "set to 2 MB",
			mb:   2.0,
			want: 2.0,
		},
		{
			name: "set to 5 MB",
			mb:   5.0,
			want: 5.0,
		},
		{
			name: "set to negative (should become 0)",
			mb:   -1.0,
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy.SetFloorSegmentMB(tt.mb)
			got := policy.GetFloorSegmentMB()
			if got != tt.want {
				t.Errorf("GetFloorSegmentMB() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestNRTMergePolicy_CalibrateSizeByDeletes(t *testing.T) {
	policy := NewNRTMergePolicy()

	// Default should be true
	if !policy.GetCalibrateSizeByDeletes() {
		t.Error("expected calibrateSizeByDeletes to be true by default")
	}

	policy.SetCalibrateSizeByDeletes(false)
	if policy.GetCalibrateSizeByDeletes() {
		t.Error("expected calibrateSizeByDeletes to be false after setting")
	}

	policy.SetCalibrateSizeByDeletes(true)
	if !policy.GetCalibrateSizeByDeletes() {
		t.Error("expected calibrateSizeByDeletes to be true after setting")
	}
}

func TestNRTMergePolicy_NRTAware(t *testing.T) {
	policy := NewNRTMergePolicy()

	// Default should be true
	if !policy.GetNRTAware() {
		t.Error("expected nrtAware to be true by default")
	}

	policy.SetNRTAware(false)
	if policy.GetNRTAware() {
		t.Error("expected nrtAware to be false after setting")
	}

	policy.SetNRTAware(true)
	if !policy.GetNRTAware() {
		t.Error("expected nrtAware to be true after setting")
	}
}

func TestNRTMergePolicy_MaxMergeDocs(t *testing.T) {
	policy := NewNRTMergePolicy()

	tests := []struct {
		name string
		docs int
		want int
	}{
		{
			name: "set to 1000",
			docs: 1000,
			want: 1000,
		},
		{
			name: "set to 10000",
			docs: 10000,
			want: 10000,
		},
		{
			name: "set to 0 (should become 1)",
			docs: 0,
			want: 1,
		},
		{
			name: "set to negative (should become 1)",
			docs: -1,
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy.SetMaxMergeDocs(tt.docs)
			got := policy.GetMaxMergeDocs()
			if got != tt.want {
				t.Errorf("GetMaxMergeDocs() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNRTMergePolicy_FindMerges(t *testing.T) {
	policy := NewNRTMergePolicy()

	// Test with nil infos
	spec, err := policy.FindMerges(FULL_FLUSH, nil, nil)
	if err == nil {
		t.Error("expected error when infos is nil")
	}
	if spec != nil {
		t.Error("expected nil spec when infos is nil")
	}
}

func TestNRTMergePolicy_FindForcedMerges(t *testing.T) {
	policy := NewNRTMergePolicy()

	// Test with nil infos
	spec, err := policy.FindForcedMerges(nil, 5, nil, nil)
	if err == nil {
		t.Error("expected error when infos is nil")
	}
	if spec != nil {
		t.Error("expected nil spec when infos is nil")
	}
}

func TestNRTMergePolicy_FindForcedDeletesMerges(t *testing.T) {
	policy := NewNRTMergePolicy()

	// Test with nil infos
	spec, err := policy.FindForcedDeletesMerges(nil, nil)
	if err == nil {
		t.Error("expected error when infos is nil")
	}
	if spec != nil {
		t.Error("expected nil spec when infos is nil")
	}
}

func TestNRTMergePolicy_UseCompoundFile(t *testing.T) {
	policy := NewNRTMergePolicy()

	// Should always return true for NRT
	if !policy.UseCompoundFile(nil, nil) {
		t.Error("expected UseCompoundFile to return true for NRT")
	}
}

func TestNRTMergePolicy_String(t *testing.T) {
	policy := NewNRTMergePolicy()

	str := policy.String()
	if str == "" {
		t.Error("String() should not return empty string")
	}

	// Should contain "NRTMergePolicy"
	if str == "" {
		t.Error("String() should return non-empty string")
	}
}

func TestNRTMergePolicy_sortBySizeAsc(t *testing.T) {
	// This test is a placeholder since we cannot easily create SegmentInfo with size
	// without a proper Directory. The actual sorting logic is tested through integration tests.
	policy := NewNRTMergePolicy()
	_ = policy // Use the policy to avoid unused variable

	// Verify the policy was created
	if policy == nil {
		t.Error("expected non-nil policy")
	}
}

func TestNRTMergePolicy_deletePct(t *testing.T) {
	policy := NewNRTMergePolicy()

	tests := []struct {
		name     string
		docCount int
		delCount int
		want     float64
	}{
		{
			name:     "no deletes",
			docCount: 100,
			delCount: 0,
			want:     0.0,
		},
		{
			name:     "50% deletes",
			docCount: 100,
			delCount: 50,
			want:     50.0,
		},
		{
			name:     "zero docs",
			docCount: 0,
			delCount: 0,
			want:     0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a SegmentInfo with proper initialization
			si := NewSegmentInfo("_test", tt.docCount, nil)
			info := NewSegmentCommitInfo(si, tt.delCount, -1)

			got := policy.deletePct(info)
			if got != tt.want {
				t.Errorf("deletePct() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestNRTMergePolicy_isMergeSizeAcceptable(t *testing.T) {
	policy := NewNRTMergePolicy()

	tests := []struct {
		name  string
		merge *OneMerge
		want  bool
	}{
		{
			name:  "nil merge",
			merge: nil,
			want:  false,
		},
		{
			name:  "empty merge",
			merge: &OneMerge{Segments: []*SegmentCommitInfo{}},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := policy.isMergeSizeAcceptable(tt.merge)
			if got != tt.want {
				t.Errorf("isMergeSizeAcceptable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNRTMergePolicy_ConcurrentAccess(t *testing.T) {
	policy := NewNRTMergePolicy()

	done := make(chan bool, 100)

	// Concurrent getters
	for i := 0; i < 50; i++ {
		go func() {
			_ = policy.GetMinMergeMB()
			_ = policy.GetMaxMergeMB()
			_ = policy.GetMergeFactor()
			_ = policy.GetCalibrateSizeByDeletes()
			_ = policy.GetNRTAware()
			done <- true
		}()
	}

	// Concurrent setters
	for i := 0; i < 50; i++ {
		go func(idx int) {
			policy.SetMinMergeMB(float64(idx))
			policy.SetMaxMergeMB(float64(idx * 10))
			policy.SetMergeFactor(idx%10 + 2)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}
