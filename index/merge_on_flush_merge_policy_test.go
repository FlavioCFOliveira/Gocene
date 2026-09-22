package index

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestMergeOnFlushMergePolicy(t *testing.T) {
	base := NewBaseMergePolicy(DefaultNoCFSRatio, DefaultMaxCFSSegmentSize)
	policy := NewMergeOnFlushMergePolicy(base)
	policy.SetSmallSegmentThresholdMB(1.0) // 1MB

	dir := store.NewMemDirectory()
	infos := &spi.SegmentInfos{}

	// Create 3 small segments
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("_%d", i)
		seg := spi.NewSegmentInfo(name, 10, dir)
		// Create a dummy file to give it size
		fileName := name + ".dat"
		dir.CreateFile(fileName)
		dir.WriteFile(fileName, make([]byte, 100*1024)) // 100KB
		seg.AddFile(fileName)

		sci := spi.NewSegmentCommitInfo(seg, 0, 0)
		infos.segments = append(infos.segments, sci)
	}

	mc := NewBaseMergeContext()
	spec, err := policy.FindFullFlushMerges(nil, infos, mc)
	if err != nil {
		t.Fatalf("FindFullFlushMerges failed: %v", err)
	}
	if spec == nil {
		t.Fatal("expected MergeSpecification, got nil")
	}
	if len(spec.Merges) != 1 {
		t.Errorf("expected 1 merge, got %d", len(spec.Merges))
	}
	if len(spec.Merges[0].Segments) != 3 {
		t.Errorf("expected 3 segments in merge, got %d", len(spec.Merges[0].Segments))
	}
}
