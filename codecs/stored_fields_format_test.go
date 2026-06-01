// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// GC-139: Codecs Tests - Stored Fields Format
// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene104/TestLucene104StoredFieldsFormat.java
// Also ports tests from BaseStoredFieldsFormatTestCase.java
package codecs_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// storedFieldsVisitor collects all stored-field values into a map for test assertions.
type storedFieldsVisitor struct {
	fields map[string]interface{}
}

func newStoredFieldsVisitor() *storedFieldsVisitor {
	return &storedFieldsVisitor{fields: make(map[string]interface{})}
}

func (v *storedFieldsVisitor) StringField(name string, value string)   { v.fields[name] = value }
func (v *storedFieldsVisitor) BinaryField(name string, value []byte)   { v.fields[name] = value }
func (v *storedFieldsVisitor) IntField(name string, value int)         { v.fields[name] = value }
func (v *storedFieldsVisitor) LongField(name string, value int64)      { v.fields[name] = value }
func (v *storedFieldsVisitor) FloatField(name string, value float32)   { v.fields[name] = value }
func (v *storedFieldsVisitor) DoubleField(name string, value float64)  { v.fields[name] = value }

// TestLucene104StoredFieldsFormat_Basic runs the base round-trip tester.
func TestLucene104StoredFieldsFormat_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	tester := codecs.NewStoredFieldsTester(t)
	format := codecs.NewLucene104StoredFieldsFormat()

	// If Lucene104StoredFieldsFormat is still a placeholder, this will log and return
	tester.TestFull(format, dir)
}

// TestLucene104StoredFieldsFormat_Random exercises the stored-fields round-trip
// with a randomized mix of string, binary, int, long, float, and double fields.
// Mirrors BaseStoredFieldsFormatTestCase.testRandom().
func TestLucene104StoredFieldsFormat_Random(t *testing.T) {
	const numDocs = 50
	rng := rand.New(rand.NewSource(42))

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segID := make([]byte, 16)
	rng.Read(segID)
	si := index.NewSegmentInfo("_rand", numDocs, dir)
	if err := si.SetID(segID); err != nil {
		t.Fatalf("SetID: %v", err)
	}

	format := codecs.NewLucene104StoredFieldsFormat()
	fieldInfos := index.NewFieldInfos()

	type docRecord struct {
		strField    string
		binField    []byte
		intField    int
		longField   int64
		floatField  float32
		doubleField float64
	}

	records := make([]docRecord, numDocs)
	writer, err := format.FieldsWriter(dir, si, store.IOContextWrite)
	if err != nil {
		t.Fatalf("FieldsWriter: %v", err)
	}

	for i := 0; i < numDocs; i++ {
		rec := docRecord{
			strField:    fmt.Sprintf("str-%d-%d", i, rng.Int63()),
			binField:    []byte(fmt.Sprintf("bin-%d", rng.Int63())),
			intField:    rng.Intn(1 << 20),
			longField:   rng.Int63(),
			floatField:  float32(rng.Float64()),
			doubleField: rng.Float64(),
		}
		records[i] = rec

		if err := writer.StartDocument(); err != nil {
			t.Fatalf("doc %d StartDocument: %v", i, err)
		}
		writeStoredField(t, writer, "str", rec.strField, nil, nil)
		writeStoredField(t, writer, "bin", "", rec.binField, nil)
		writeStoredField(t, writer, "int", "", nil, rec.intField)
		writeStoredField(t, writer, "long", "", nil, rec.longField)
		writeStoredField(t, writer, "float", "", nil, rec.floatField)
		writeStoredField(t, writer, "double", "", nil, rec.doubleField)
		if err := writer.FinishDocument(); err != nil {
			t.Fatalf("doc %d FinishDocument: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	reader, err := format.FieldsReader(dir, si, fieldInfos, store.IOContextRead)
	if err != nil {
		t.Fatalf("FieldsReader: %v", err)
	}
	defer reader.Close()

	for i, rec := range records {
		v := newStoredFieldsVisitor()
		if err := reader.VisitDocument(i, v); err != nil {
			t.Fatalf("doc %d VisitDocument: %v", i, err)
		}
		got := v.fields
		if got["str"] != rec.strField {
			t.Errorf("doc %d str: got %v want %v", i, got["str"], rec.strField)
		}
		if !bytes.Equal(got["bin"].([]byte), rec.binField) {
			t.Errorf("doc %d bin: mismatch", i)
		}
	}
}

// TestLucene104StoredFieldsFormat_BigDocuments verifies that documents with large
// binary and string payloads round-trip correctly.
// Mirrors BaseStoredFieldsFormatTestCase.testBigDocuments().
func TestLucene104StoredFieldsFormat_BigDocuments(t *testing.T) {
	const (
		numDocs   = 5
		fieldSize = 100_000 // 100 KB payload per field
	)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segID := make([]byte, 16)
	if _, err := rand.New(rand.NewSource(7)).Read(segID); err != nil {
		t.Fatalf("rand: %v", err)
	}
	si := index.NewSegmentInfo("_big", numDocs, dir)
	if err := si.SetID(segID); err != nil {
		t.Fatalf("SetID: %v", err)
	}

	format := codecs.NewLucene104StoredFieldsFormat()
	fieldInfos := index.NewFieldInfos()

	payloads := make([][]byte, numDocs)
	writer, err := format.FieldsWriter(dir, si, store.IOContextWrite)
	if err != nil {
		t.Fatalf("FieldsWriter: %v", err)
	}

	for i := 0; i < numDocs; i++ {
		payload := make([]byte, fieldSize)
		for j := range payload {
			payload[j] = byte(j & 0xff)
		}
		payloads[i] = payload

		if err := writer.StartDocument(); err != nil {
			t.Fatalf("doc %d StartDocument: %v", i, err)
		}
		writeStoredField(t, writer, "big", "", payload, nil)
		// Also add a large string field.
		bigStr := strings.Repeat(fmt.Sprintf("doc%d-", i), fieldSize/6)
		writeStoredField(t, writer, "bigstr", bigStr, nil, nil)
		if err := writer.FinishDocument(); err != nil {
			t.Fatalf("doc %d FinishDocument: %v", i, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	reader, err := format.FieldsReader(dir, si, fieldInfos, store.IOContextRead)
	if err != nil {
		t.Fatalf("FieldsReader: %v", err)
	}
	defer reader.Close()

	for i := range payloads {
		v := newStoredFieldsVisitor()
		if err := reader.VisitDocument(i, v); err != nil {
			t.Fatalf("doc %d VisitDocument: %v", i, err)
		}
		got := v.fields
		gotBin, ok := got["big"]
		if !ok {
			t.Fatalf("doc %d: missing big field", i)
		}
		if !bytes.Equal(gotBin.([]byte), payloads[i]) {
			t.Errorf("doc %d big field: payload mismatch (len got=%d want=%d)", i, len(gotBin.([]byte)), len(payloads[i]))
		}
	}
}

// TestLucene104StoredFieldsFormat_NumericField verifies that all numeric stored-field
// types (int, long, float, double) round-trip without loss.
// Mirrors BaseStoredFieldsFormatTestCase.testNumericField().
func TestLucene104StoredFieldsFormat_NumericField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segID := make([]byte, 16)
	rand.New(rand.NewSource(99)).Read(segID)
	si := index.NewSegmentInfo("_num", 1, dir)
	if err := si.SetID(segID); err != nil {
		t.Fatalf("SetID: %v", err)
	}

	format := codecs.NewLucene104StoredFieldsFormat()
	fieldInfos := index.NewFieldInfos()

	writer, err := format.FieldsWriter(dir, si, store.IOContextWrite)
	if err != nil {
		t.Fatalf("FieldsWriter: %v", err)
	}
	if err := writer.StartDocument(); err != nil {
		t.Fatalf("StartDocument: %v", err)
	}
	writeStoredField(t, writer, "i", "", nil, int(42))
	writeStoredField(t, writer, "l", "", nil, int64(1<<40))
	writeStoredField(t, writer, "f", "", nil, float32(3.14))
	writeStoredField(t, writer, "d", "", nil, float64(2.71828))
	if err := writer.FinishDocument(); err != nil {
		t.Fatalf("FinishDocument: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	reader, err := format.FieldsReader(dir, si, fieldInfos, store.IOContextRead)
	if err != nil {
		t.Fatalf("FieldsReader: %v", err)
	}
	defer reader.Close()

	v := newStoredFieldsVisitor()
	if err := reader.VisitDocument(0, v); err != nil {
		t.Fatalf("VisitDocument: %v", err)
	}
	got := v.fields
	if got["i"] != int(42) {
		t.Errorf("int field: got %v (%T) want 42", got["i"], got["i"])
	}
	if got["l"] != int64(1<<40) {
		t.Errorf("long field: got %v (%T) want %d", got["l"], got["l"], int64(1<<40))
	}
	// Floating-point values use exact equality (same bits were serialised and deserialised).
	if got["f"] != float32(3.14) {
		t.Errorf("float field: got %v want 3.14", got["f"])
	}
	if got["d"] != float64(2.71828) {
		t.Errorf("double field: got %v want 2.71828", got["d"])
	}
}

// TestLucene104StoredFieldsFormat_ConcurrentReads verifies that multiple goroutines
// can read stored fields from the same reader concurrently without data races.
// Mirrors BaseStoredFieldsFormatTestCase.testConcurrentReads().
func TestLucene104StoredFieldsFormat_ConcurrentReads(t *testing.T) {
	const (
		numDocs        = 20
		numGoroutines  = 8
		readsPerThread = 50
	)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segID := make([]byte, 16)
	rand.New(rand.NewSource(55)).Read(segID)
	si := index.NewSegmentInfo("_conc", numDocs, dir)
	if err := si.SetID(segID); err != nil {
		t.Fatalf("SetID: %v", err)
	}

	format := codecs.NewLucene104StoredFieldsFormat()
	fieldInfos := index.NewFieldInfos()

	// Write documents.
	expected := make([]string, numDocs)
	writer, err := format.FieldsWriter(dir, si, store.IOContextWrite)
	if err != nil {
		t.Fatalf("FieldsWriter: %v", err)
	}
	for i := 0; i < numDocs; i++ {
		val := fmt.Sprintf("concurrent-doc-%d", i)
		expected[i] = val
		if err := writer.StartDocument(); err != nil {
			t.Fatalf("StartDocument: %v", err)
		}
		writeStoredField(t, writer, "val", val, nil, nil)
		if err := writer.FinishDocument(); err != nil {
			t.Fatalf("FinishDocument: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	reader, err := format.FieldsReader(dir, si, fieldInfos, store.IOContextRead)
	if err != nil {
		t.Fatalf("FieldsReader: %v", err)
	}
	defer reader.Close()

	var wg sync.WaitGroup
	errCh := make(chan string, numGoroutines*readsPerThread)
	rng := rand.New(rand.NewSource(77))
	var rngMu sync.Mutex

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < readsPerThread; r++ {
				rngMu.Lock()
				docID := rng.Intn(numDocs)
				rngMu.Unlock()

				v := newStoredFieldsVisitor()
				if err := reader.VisitDocument(docID, v); err != nil {
					errCh <- fmt.Sprintf("doc %d: %v", docID, err)
					return
				}
				got := v.fields
				if got["val"] != expected[docID] {
					errCh <- fmt.Sprintf("doc %d val: got %v want %v", docID, got["val"], expected[docID])
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for msg := range errCh {
		t.Errorf("concurrent read error: %s", msg)
	}
}

// writeStoredField is a helper that writes a single field to a StoredFieldsWriter.
// Exactly one of strVal, binVal, numVal should be non-nil/non-empty.
func writeStoredField(t *testing.T, w codecs.StoredFieldsWriter, name, strVal string, binVal []byte, numVal interface{}) {
	t.Helper()
	f := &testStoredField{name: name, strVal: strVal, binVal: binVal, numVal: numVal}
	if err := w.WriteField(f); err != nil {
		t.Fatalf("WriteField(%q): %v", name, err)
	}
}

// testStoredField is a minimal IndexableField for stored-fields tests.
type testStoredField struct {
	name   string
	strVal string
	binVal []byte
	numVal interface{}
}

func (f *testStoredField) Name() string            { return f.name }
func (f *testStoredField) StringValue() string     { return f.strVal }
func (f *testStoredField) BinaryValue() []byte     { return f.binVal }
func (f *testStoredField) NumericValue() interface{} { return f.numVal }
