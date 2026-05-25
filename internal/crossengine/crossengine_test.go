package crossengine_test

import (
	"os"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/internal/crossengine"
)

func TestFixturesDirExists(t *testing.T) {
	crossengine.SkipIfNoFixtures(t)
	dir := crossengine.FixturesDir()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("FixturesDir stat: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("FixturesDir %q is not a directory", dir)
	}
}

func TestOpenFixturesDir(t *testing.T) {
	d := crossengine.OpenFixturesDir(t)
	files, err := d.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("fixtures directory is empty")
	}
	t.Logf("fixture files: %v", files)
}

func TestSkipWhenAbsent(t *testing.T) {
	// Verify the SkipIfNoFixtures function itself is callable — if fixtures
	// exist it is a no-op; if absent the test is skipped.  Either outcome is
	// valid for CI environments that may or may not have fixtures pre-generated.
	crossengine.SkipIfNoFixtures(t)
}
