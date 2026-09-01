package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spi"
)

func TestForceMergePolicy(t *testing.T) {
	base := NewBaseMergePolicy(DefaultNoCFSRatio, DefaultMaxCFSSegmentSize)
	policy := NewForceMergePolicy(base)

	infos := &spi.SegmentInfos{}
	mc := NewBaseMergeContext()

	spec, err := policy.FindMerges(nil, infos, mc)
	if err != nil {
		t.Fatalf("FindMerges failed: %v", err)
	}
	if spec != nil {
		t.Errorf("expected nil MergeSpecification, got %v", spec)
	}
}

func TestMergeOnFlushMergePolicy(t *testing.T) {
	base := NewBaseMergePolicy(DefaultNoCFSRatio, DefaultMaxCFSSegmentSize)
	policy := NewMergeOnFlushMergePolicy(base)
	policy.SetSmallSegmentThresholdMB(1.0) // 1MB

	// Mock SegmentCommitInfos
	// Note: In a real test, we'd need to setup Directory and files to get real sizes.
	// For now, we'll mock the size by using a dummy directory if possible,
	// or we can just test the logic if we can control SizeInBytes.
	// Since SizeInBytes reads from directory, we need a real directory.
}
