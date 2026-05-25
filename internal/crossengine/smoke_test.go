package crossengine_test

import (
	"os"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/internal/crossengine"
)

// expectedFixtureFiles are the files that must be present in a valid fixture set.
var expectedFixtureFiles = []string{
	"segments_1",
	"_0.si",
	"_0.cfs",
	"_0.cfe",
}

// indexFileExtensions that carry the Lucene magic header.
var magicFileExtensions = []string{".si", ".cfs", ".cfe"}

func TestSmokeFixtureFilesPresent(t *testing.T) {
	crossengine.SkipIfNoFixtures(t)
	dir := crossengine.FixturesDir()
	for _, name := range expectedFixtureFiles {
		path := dir + "/" + name
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected fixture file %q not found: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("fixture file %q is empty", name)
		}
	}
}

func TestSmokeDirectoryListAll(t *testing.T) {
	d := crossengine.OpenFixturesDir(t)
	files, err := d.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}

	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f] = true
	}

	for _, want := range expectedFixtureFiles {
		if !fileSet[want] {
			t.Errorf("expected fixture %q not listed by Directory.ListAll()", want)
		}
	}
}

func TestSmokeMagicHeaders(t *testing.T) {
	crossengine.SkipIfNoFixtures(t)
	dir := crossengine.FixturesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	checked := 0
	for _, e := range entries {
		name := e.Name()
		hasMagicExt := false
		for _, ext := range magicFileExtensions {
			if strings.HasSuffix(name, ext) {
				hasMagicExt = true
				break
			}
		}
		// segments_N also carries the magic
		if strings.HasPrefix(name, "segments_") {
			hasMagicExt = true
		}
		if !hasMagicExt {
			continue
		}

		path := dir + "/" + name
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %q: %v", name, err)
			continue
		}
		if !crossengine.HasMagic(data) {
			if len(data) >= 4 {
				t.Errorf("file %q: expected Lucene magic 0x3fd76c17, got 0x%02x%02x%02x%02x",
					name, data[0], data[1], data[2], data[3])
			} else {
				t.Errorf("file %q: too short (%d bytes) to contain magic header", name, len(data))
			}
			continue
		}
		checked++
	}

	if checked == 0 {
		t.Error("no index files with magic headers found — fixtures may be incomplete")
	}
	t.Logf("verified Lucene magic header in %d fixture files", checked)
}

func TestSmokeSegmentsFileNonEmpty(t *testing.T) {
	crossengine.SkipIfNoFixtures(t)
	dir := crossengine.FixturesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "segments_") {
			info, _ := e.Info()
			if info.Size() < 10 {
				t.Errorf("segments file %q is suspiciously small (%d bytes)", e.Name(), info.Size())
			}
			return
		}
	}
	t.Error("no segments_N file found in fixtures")
}
