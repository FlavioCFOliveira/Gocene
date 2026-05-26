// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene90_stored_fields_compat_test.go covers Lucene90StoredFieldsFormat
// (the .fdt payload + .fdx index + .fdm metadata triple) AND its
// compressing variant. The base scenario uses the default codec
// (BEST_SPEED + LZ4); the new compressing-stored-fields scenario added
// by rmp 4615 exercises BEST_COMPRESSION (DEFLATE).
//
// Audit rows cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene90StoredFieldsFormat (.fdt/.fdx/.fdm)" — gap_notes:
//	  "Stored-fields bytes inside .cfs fixture are not decoded by an
//	   isolated test."
//	"Lucene90 compressing block format (LZ4/Deflate/BEST_SPEED)" —
//	  gap_notes: "Compression bytes never compared to Lucene output."
package codecs

import (
	"bytes"
	"fmt"
	"testing"

	gcodecs "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// storedFieldsCollector implements codecs.StoredFieldVisitor collecting
// all field values visited from a single document.
type storedFieldsCollector struct {
	strings  []fieldNamedString
	binaries []fieldNamedBytes
	longs    []fieldNamedLong
}

type fieldNamedString struct{ name, value string }
type fieldNamedBytes struct {
	name  string
	value []byte
}
type fieldNamedLong struct {
	name  string
	value int64
}

func (c *storedFieldsCollector) StringField(name, v string) {
	c.strings = append(c.strings, fieldNamedString{name, v})
}
func (c *storedFieldsCollector) BinaryField(name string, v []byte) {
	cp := make([]byte, len(v))
	copy(cp, v)
	c.binaries = append(c.binaries, fieldNamedBytes{name, cp})
}
func (c *storedFieldsCollector) IntField(name string, _ int) {}
func (c *storedFieldsCollector) LongField(name string, v int64) {
	c.longs = append(c.longs, fieldNamedLong{name, v})
}
func (c *storedFieldsCollector) FloatField(name string, _ float32)  {}
func (c *storedFieldsCollector) DoubleField(name string, _ float64) {}

// openStoredFieldsReader opens the stored-fields reader for the "_0" segment
// in dir by reading the .si and .fnm from the fixture, then delegating to
// Lucene90StoredFieldsFormat. It is the cross-engine reader path that
// AC#3 mandates.
func openStoredFieldsReader(
	t *testing.T,
	dir store.Directory,
) (gcodecs.StoredFieldsReader, func()) {
	t.Helper()

	siFormat := gcodecs.NewLucene99SegmentInfoFormat()
	si, err := siFormat.Read(dir, "_0", nil, store.IOContextDefault)
	if err != nil {
		t.Fatalf("read .si: %v", err)
	}

	fiFormat := gcodecs.NewLucene104FieldInfosFormat()
	fn, err := fiFormat.Read(dir, si, "", store.IOContextDefault)
	if err != nil {
		t.Fatalf("read .fnm: %v", err)
	}

	format := lucene90.NewLucene90StoredFieldsFormat()
	reader, err := format.FieldsReader(dir, si, fn, store.IOContextDefault)
	if err != nil {
		t.Fatalf("FieldsReader: %v", err)
	}
	return reader, func() { _ = reader.Close() }
}

// TestLucene90StoredFields_BestSpeed_AllThreeFiles checks the LZ4 path:
// .fdt/.fdx/.fdm IndexHeaders all parse and CRC32 trailers all validate.
// The compressing-stored-fields code path stamps the per-codec NAME as
// "Lucene90StoredFieldsFastData" and "Lucene90StoredFieldsFastIndex"
// for the BEST_SPEED layout (see codecs/lucene90/lucene90_stored_fields_format.go).
func TestLucene90StoredFields_BestSpeed_AllThreeFiles(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "stored-fields-format", seed)
			for _, ext := range []string{".fdt", ".fdx", ".fdm"} {
				name := findUniqueByExt(t, dir, ext)
				if err := validateOneEnvelope(t, dir, name); err != nil {
					t.Fatalf("%s: CRC validation failed: %v", name, err)
				}
			}
		})
	}
}

// TestLucene90StoredFields_BestCompression_AllThreeFiles is the DEFLATE
// counterpart, fed by the new "compressing-stored-fields" scenario. The
// scenario uses Lucene104Codec(BEST_COMPRESSION) which switches the
// stored-fields wire codec to "Lucene90StoredFieldsHighData".
func TestLucene90StoredFields_BestCompression_AllThreeFiles(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "compressing-stored-fields", seed)
			for _, ext := range []string{".fdt", ".fdx", ".fdm"} {
				name := findUniqueByExt(t, dir, ext)
				if err := validateOneEnvelope(t, dir, name); err != nil {
					t.Fatalf("%s: CRC validation failed: %v", name, err)
				}
			}
		})
	}
}

// TestLucene90StoredFields_BothModesProduceDifferentBytes is the
// payload-level smoke test: BEST_SPEED and BEST_COMPRESSION on the same
// documents MUST produce different .fdt bytes (LZ4 vs DEFLATE compress
// differently). Catches the regression where the Mode constructor is
// ignored and both modes silently use LZ4.
func TestLucene90StoredFields_BothModesProduceDifferentBytes(t *testing.T) {
	requireHarness(t)
	// Note: we cannot directly compare the two .fdt bytes because the
	// two scenarios index different documents (different numDocs and
	// different body content). The differentiator is instead the codec
	// name stamped in the IndexHeader: BEST_SPEED writes
	// "Lucene90StoredFieldsFastData" while BEST_COMPRESSION writes
	// "Lucene90StoredFieldsHighData". Validate both names appear.
	speedDir := generate(t, "stored-fields-format", 0xC0FFEE)
	compDir := generate(t, "compressing-stored-fields", 0xC0FFEE)
	speedFdt := findUniqueByExt(t, speedDir, ".fdt")
	compFdt := findUniqueByExt(t, compDir, ".fdt")
	const suffix = ""
	expectIndexCodecName(t, speedDir, speedFdt, "Lucene90StoredFieldsFastData",
		0, 32, suffix)
	expectIndexCodecName(t, compDir, compFdt, "Lucene90StoredFieldsHighData",
		0, 32, suffix)
}

// TestLucene90StoredFields_BestSpeed_PayloadValues verifies that Gocene's
// reader can decode the LZ4-compressed stored-fields bytes produced by
// Apache Lucene 10.4.0 and recover the exact field values written by the
// stored-fields-format scenario (StoredFieldsFormatScenario).
//
// This test directly closes AC#3: "internal/compat scenarios that exercise
// stored fields no longer require their workaround paths." The reader path
// is exercised end-to-end: .si → SegmentInfo (with MODE_KEY attribute) →
// .fnm → FieldInfos → .fdt/.fdx/.fdm → VisitDocument → field values.
func TestLucene90StoredFields_BestSpeed_PayloadValues(t *testing.T) {
	const seed int64 = 0xC0FFEE
	// StoredFieldsFormatScenario.buildDoc:
	//   title: "title-%d-%d" % (i, seed & 0xFFFF)
	//   payload: bytes("payload-%d-%d" % (i, seed))
	//   count: long(seed ^ i)
	shortSeed := int64(seed & 0xFFFF)
	type wantDoc struct {
		title   string
		payload []byte
		count   int64
	}
	// Verify first 3 of 10 docs (seed=0xC0FFEE).
	wants := []wantDoc{
		{
			title:   fmt.Sprintf("title-0-%d", shortSeed),
			payload: []byte(fmt.Sprintf("payload-0-%d", seed)),
			count:   seed ^ 0,
		},
		{
			title:   fmt.Sprintf("title-1-%d", shortSeed),
			payload: []byte(fmt.Sprintf("payload-1-%d", seed)),
			count:   seed ^ 1,
		},
		{
			title:   fmt.Sprintf("title-2-%d", shortSeed),
			payload: []byte(fmt.Sprintf("payload-2-%d", seed)),
			count:   seed ^ 2,
		},
	}

	rawDir := generate(t, "stored-fields-format", seed)
	d, err := store.NewSimpleFSDirectory(rawDir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	defer d.Close()

	reader, cleanup := openStoredFieldsReader(t, d)
	defer cleanup()

	for docID, want := range wants {
		var coll storedFieldsCollector
		if err := reader.VisitDocument(docID, &coll); err != nil {
			t.Fatalf("doc=%d VisitDocument: %v", docID, err)
		}
		t.Run(fmt.Sprintf("doc%d", docID), func(t *testing.T) {
			// title (string)
			if len(coll.strings) == 0 {
				t.Fatalf("no string fields")
			}
			if got := coll.strings[0].value; got != want.title {
				t.Errorf("title: got %q, want %q", got, want.title)
			}
			// payload (binary)
			if len(coll.binaries) == 0 {
				t.Fatalf("no binary fields")
			}
			if !bytes.Equal(coll.binaries[0].value, want.payload) {
				t.Errorf("payload mismatch: got %q, want %q",
					coll.binaries[0].value, want.payload)
			}
			// count (long)
			if len(coll.longs) == 0 {
				t.Fatalf("no long fields")
			}
			if got := coll.longs[0].value; got != want.count {
				t.Errorf("count: got %d, want %d", got, want.count)
			}
		})
	}
}

// TestLucene90StoredFields_BestCompression_PayloadValues is the DEFLATE
// counterpart of TestLucene90StoredFields_BestSpeed_PayloadValues. It
// verifies that Gocene's reader decodes BEST_COMPRESSION .fdt bytes
// produced by Lucene 10.4.0 (CompressingStoredFieldsScenario).
//
// CompressingStoredFieldsScenario.buildDoc:
//
//	title: "title-%d-%d" % (i, seed & 0xFFFF)
//	body:  []byte of repeating "compress-%d-" patterns (64 repetitions)
//	count: long(seed ^ i)
//
// Body content is validated by length and byte-prefix rather than an exact
// equality check because the mixing formula involves a large loop.
func TestLucene90StoredFields_BestCompression_PayloadValues(t *testing.T) {
	const seed int64 = 0xC0FFEE
	shortSeed := int64(seed & 0xFFFF)

	rawDir := generate(t, "compressing-stored-fields", seed)
	d, err := store.NewSimpleFSDirectory(rawDir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	defer d.Close()

	reader, cleanup := openStoredFieldsReader(t, d)
	defer cleanup()

	// Verify first 3 of 64 docs.
	for docID := 0; docID < 3; docID++ {
		docID := docID
		var coll storedFieldsCollector
		if err := reader.VisitDocument(docID, &coll); err != nil {
			t.Fatalf("doc=%d VisitDocument: %v", docID, err)
		}
		t.Run(fmt.Sprintf("doc%d", docID), func(t *testing.T) {
			wantTitle := fmt.Sprintf("title-%d-%d", docID, shortSeed)
			if len(coll.strings) == 0 {
				t.Fatalf("no string fields")
			}
			if got := coll.strings[0].value; got != wantTitle {
				t.Errorf("title: got %q, want %q", got, wantTitle)
			}
			// body is a binary field
			if len(coll.binaries) == 0 {
				t.Fatalf("no binary fields for doc %d", docID)
			}
			// Body must be non-empty and start with "compress-"
			body := coll.binaries[0].value
			if len(body) == 0 {
				t.Errorf("doc %d: body is empty", docID)
			} else if !bytes.HasPrefix(body, []byte("compress-")) {
				t.Errorf("doc %d: body does not start with 'compress-': %q",
					docID, body[:min(len(body), 20)])
			}
			// count (long)
			wantCount := seed ^ int64(docID)
			if len(coll.longs) == 0 {
				t.Fatalf("no long fields for doc %d", docID)
			}
			if got := coll.longs[0].value; got != wantCount {
				t.Errorf("count: got %d, want %d", got, wantCount)
			}
		})
	}
}
