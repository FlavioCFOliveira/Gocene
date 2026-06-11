package crossengine

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestDebugSegmentRead(t *testing.T) {
	fixturesDir := FixturesDir()
	dir, err := store.NewMMapDirectory(fixturesDir)
	if err != nil {
		t.Fatalf("NewMMapDirectory: %v", err)
	}
	defer dir.Close()

	// Try manually
	in, err := dir.OpenInput("segments_1", store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()
	t.Logf("File length: %d", in.Length())

	magic, err := store.ReadInt32(in)
	if err != nil {
		t.Fatalf("ReadInt32: %v", err)
	}
	t.Logf("Magic: 0x%x", magic)

	// Try ReadSegmentInfos
	_, err = spi.ReadSegmentInfos(dir)
	t.Logf("ReadSegmentInfos error: %v", err)
}
