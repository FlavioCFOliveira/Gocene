package crossengine_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/internal/crossengine"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestCrossEngine_ReadSegmentInfos verifies that index.ReadSegmentInfos can
// parse the segments_1 file from the committed Lucene 10.4.0 golden fixtures.
func TestCrossEngine_ReadSegmentInfos(t *testing.T) {
	crossengine.SkipIfNoFixtures(t)
	dir, err := store.NewMMapDirectory(crossengine.FixturesDir())
	if err != nil {
		t.Fatalf("NewMMapDirectory: %v", err)
	}
	defer dir.Close()

	sis, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos: %v", err)
	}
	if sis == nil {
		t.Fatal("ReadSegmentInfos returned nil")
	}
	if sis.Generation() < 1 {
		t.Errorf("expected generation >= 1, got %d", sis.Generation())
	}
	segments := sis.List()
	if len(segments) < 1 {
		t.Errorf("expected at least 1 segment, got %d", len(segments))
	}
	t.Logf("ReadSegmentInfos: generation=%d, segments=%d", sis.Generation(), len(segments))
}

// TestCrossEngine_ReadSegmentInfo_si verifies that codecs.Lucene99SegmentInfoFormat
// can parse the _0.si file from the committed Lucene 10.4.0 golden fixtures.
// The fixture has docCount=20 (20 docs written by FixtureGen.java) and
// uses a compound file (.cfs), so isCompoundFile must be true.
func TestCrossEngine_ReadSegmentInfo_si(t *testing.T) {
	crossengine.SkipIfNoFixtures(t)
	dir, err := store.NewMMapDirectory(crossengine.FixturesDir())
	if err != nil {
		t.Fatalf("NewMMapDirectory: %v", err)
	}
	defer dir.Close()

	// Read segments_1 first to get the segment ID.
	sis, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos: %v", err)
	}
	segments := sis.List()
	if len(segments) == 0 {
		t.Fatal("no segments in fixture")
	}
	seg0 := segments[0]
	segName := seg0.Name()
	segID := seg0.SegmentInfo().GetID()

	t.Logf("Reading .si for segment %q (ID len=%d)", segName, len(segID))

	f := codecs.NewLucene99SegmentInfoFormat()
	si, err := f.Read(dir, segName, segID, store.IOContextRead)
	if err != nil {
		t.Fatalf("Lucene99SegmentInfoFormat.Read: %v", err)
	}

	if si.DocCount() != 20 {
		t.Errorf("expected docCount=20, got %d", si.DocCount())
	}
	if !si.IsCompoundFile() {
		t.Errorf("expected isCompoundFile=true (fixture uses .cfs)")
	}
	t.Logf("SegmentInfo: name=%q, docCount=%d, compoundFile=%v, version=%s",
		si.Name(), si.DocCount(), si.IsCompoundFile(), si.Version())
}
