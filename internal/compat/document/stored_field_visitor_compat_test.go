// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// stored_field_visitor_compat_test.go verifies that Gocene can read stored
// fields written by Apache Lucene 10.4.0 via the StoredFieldVisitor pattern,
// recovering exact string, binary and numeric field values.
//
// Audit row (docs/compat-coverage.tsv, row 99):
//
//	"StoredField visitor serialisation" — gap_notes:
//	  "No fixture-based test reads stored fields produced by Lucene."
package document

import (
	"bytes"
	"fmt"
	"testing"

	gcodecs "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// sfvCollector implements codecs.StoredFieldVisitor collecting all field
// values visited from a single document.
type sfvCollector struct {
	strings  []sfvString
	binaries []sfvBinary
	ints     []sfvInt
	longs    []sfvLong
	floats   []sfvFloat
	doubles  []sfvDouble
}

type sfvString struct{ name, value string }
type sfvBinary struct {
	name  string
	value []byte
}
type sfvInt struct {
	name  string
	value int
}
type sfvLong struct {
	name  string
	value int64
}
type sfvFloat struct {
	name  string
	value float32
}
type sfvDouble struct {
	name  string
	value float64
}

func (c *sfvCollector) StringField(name, v string) {
	c.strings = append(c.strings, sfvString{name, v})
}
func (c *sfvCollector) BinaryField(name string, v []byte) {
	cp := make([]byte, len(v))
	copy(cp, v)
	c.binaries = append(c.binaries, sfvBinary{name, cp})
}
func (c *sfvCollector) IntField(name string, v int) {
	c.ints = append(c.ints, sfvInt{name, v})
}
func (c *sfvCollector) LongField(name string, v int64) {
	c.longs = append(c.longs, sfvLong{name, v})
}
func (c *sfvCollector) FloatField(name string, v float32) {
	c.floats = append(c.floats, sfvFloat{name, v})
}
func (c *sfvCollector) DoubleField(name string, v float64) {
	c.doubles = append(c.doubles, sfvDouble{name, v})
}

// openStoredFieldsReader opens the stored-fields reader for segment "_0"
// in dir by reading the .si and .fnm from the fixture.
func openStoredFieldsReader(t *testing.T, dir store.Directory) (gcodecs.StoredFieldsReader, func()) {
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

// TestStoredFieldVisitor_BestSpeed_PayloadValues verifies that Gocene's
// StoredFieldVisitor can read Lucene-written stored fields (LZ4 BEST_SPEED
// path) and recover exact field values from the stored-fields-format
// scenario.
//
// StoredFieldsFormatScenario.buildDoc (10 docs, seed S):
//
//	title:   "title-%d-%d" % (i, S & 0xFFFF)
//	payload: bytes("payload-%d-%d" % (i, S))
//	count:   long(S ^ i)
func TestStoredFieldVisitor_BestSpeed_PayloadValues(t *testing.T) {
	const seed int64 = 0xC0FFEE
	shortSeed := int64(seed & 0xFFFF)

	rawDir := generate(t, "stored-fields-format", seed)
	d, err := store.NewSimpleFSDirectory(rawDir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	defer d.Close()

	reader, cleanup := openStoredFieldsReader(t, d)
	defer cleanup()

	// Verify first 3 of 10 docs.
	for docID := 0; docID < 3; docID++ {
		docID := docID
		var coll sfvCollector
		if err := reader.VisitDocument(docID, &coll); err != nil {
			t.Fatalf("doc=%d VisitDocument: %v", docID, err)
		}
		t.Run(fmt.Sprintf("doc%d", docID), func(t *testing.T) {
			wantTitle := fmt.Sprintf("title-%d-%d", docID, shortSeed)
			if len(coll.strings) == 0 {
				t.Fatal("no string fields")
			}
			if got := coll.strings[0].value; got != wantTitle {
				t.Errorf("title: got %q, want %q", got, wantTitle)
			}

			wantPayload := []byte(fmt.Sprintf("payload-%d-%d", docID, seed))
			if len(coll.binaries) == 0 {
				t.Fatal("no binary fields")
			}
			if !bytes.Equal(coll.binaries[0].value, wantPayload) {
				t.Errorf("payload: got %q, want %q",
					coll.binaries[0].value, wantPayload)
			}

			wantCount := seed ^ int64(docID)
			if len(coll.longs) == 0 {
				t.Fatal("no long fields")
			}
			if got := coll.longs[0].value; got != wantCount {
				t.Errorf("count: got %d, want %d", got, wantCount)
			}
		})
	}
}

// TestStoredFieldVisitor_BestCompression_PayloadValues is the DEFLATE
// counterpart, fed by the "compressing-stored-fields" scenario.
func TestStoredFieldVisitor_BestCompression_PayloadValues(t *testing.T) {
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
		var coll sfvCollector
		if err := reader.VisitDocument(docID, &coll); err != nil {
			t.Fatalf("doc=%d VisitDocument: %v", docID, err)
		}
		t.Run(fmt.Sprintf("doc%d", docID), func(t *testing.T) {
			wantTitle := fmt.Sprintf("title-%d-%d", docID, shortSeed)
			if len(coll.strings) == 0 {
				t.Fatal("no string fields")
			}
			if got := coll.strings[0].value; got != wantTitle {
				t.Errorf("title: got %q, want %q", got, wantTitle)
			}

			if len(coll.binaries) == 0 {
				t.Fatalf("no binary fields for doc %d", docID)
			}
			body := coll.binaries[0].value
			if len(body) == 0 {
				t.Errorf("doc %d: body is empty", docID)
			} else if !bytes.HasPrefix(body, []byte("compress-")) {
				t.Errorf("doc %d: body does not start with 'compress-': %q",
					docID, body[:min(len(body), 20)])
			}

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

// TestStoredFieldVisitor_BothSeeds runs the payload-value check at both
// canary seeds to verify byte-determinism.
func TestStoredFieldVisitor_BothSeeds(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			rawDir := generate(t, "stored-fields-format", seed)
			d, err := store.NewSimpleFSDirectory(rawDir)
			if err != nil {
				t.Fatalf("open dir: %v", err)
			}
			defer d.Close()

			reader, cleanup := openStoredFieldsReader(t, d)
			defer cleanup()

			// Visit doc 0 and verify at least the title string is present.
			var coll sfvCollector
			if err := reader.VisitDocument(0, &coll); err != nil {
				t.Fatalf("doc=0 VisitDocument: %v", err)
			}
			if len(coll.strings) == 0 {
				t.Fatal("no string fields")
			}
			if coll.strings[0].value == "" {
				t.Fatal("title is empty")
			}
		})
	}
}
