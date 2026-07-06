// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

package smoke

import (
	"fmt"
	"os"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/internal/compat"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestCompat_QuickAudit_AllFive(t *testing.T) {
	const seed int64 = 0xC0FFEE

	results := make(map[string]scenarioResult)

	results["postings-terms"] = testPostingsTerms(t, seed)
	results["stored-fields"] = testStoredFields(t, seed)
	results["term-vectors"] = testTermVectors(t, seed)
	results["docvalues-numeric"] = testDocValuesNumeric(t, seed)
	results["multi-segment"] = testMultiSegment(t, seed)

	t.Log("=== COMPAT QUICK AUDIT RESULTS ===")
	for name, r := range results {
		status := "PASS"
		if !r.ok {
			status = "FAIL"
		}
		t.Logf("SCENARIO=%s STATUS=%s OK=%v ERROR=%q COMPONENT=%q DETAIL=%q",
			name, status, r.ok, r.errMsg, r.component, r.detail)
	}
}

type scenarioResult struct {
	ok        bool
	errMsg    string
	component string
	detail    string
}

func requireHarness(t *testing.T) {
	t.Helper()
	if _, err := compat.Locate(); err != nil {
		if os.IsNotExist(err) || err == compat.ErrHarnessMissing {
			t.Fatalf("compat harness missing: %v", err)
		}
		t.Fatalf("locate harness: %v", err)
	}
}

func generateDir(t *testing.T, scenario string, seed int64) string {
	t.Helper()
	requireHarness(t)
	dir := t.TempDir()
	if err := compat.GenerateInto(scenario, seed, dir); err != nil {
		t.Fatalf("harness gen %s seed=%d: %v", scenario, seed, err)
	}
	return dir
}

// ------------------------------------------------------------------
// 1. Postings + Terms
// ------------------------------------------------------------------
func testPostingsTerms(t *testing.T, seed int64) scenarioResult {
	var r scenarioResult
	defer func() {
		if rec := recover(); rec != nil {
			r.ok = false
			r.errMsg = fmt.Sprintf("panic: %v", rec)
		}
	}()

	dirPath := generateDir(t, "postings-format", seed)
	dir, err := store.NewSimpleFSDirectory(dirPath)
	if err != nil {
		r.component = "store.NewSimpleFSDirectory"
		r.errMsg = err.Error()
		return r
	}
	defer dir.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		r.component = "index.OpenDirectoryReader"
		r.errMsg = err.Error()
		return r
	}
	defer reader.Close()

	segs := reader.GetSegmentReaders()
	if len(segs) == 0 {
		r.component = "SegmentReader"
		r.errMsg = "no segments"
		return r
	}
	seg := segs[0]

	cr := seg.GetCoreReaders()
	if cr == nil {
		r.component = "SegmentCoreReaders"
		r.errMsg = "coreReaders is nil"
		return r
	}
	fields := cr.GetFields()
	if fields == nil {
		r.component = "FieldsProducer"
		r.errMsg = "fields is nil"
		return r
	}

	terms, err := fields.Terms("body")
	if err != nil {
		r.component = "Fields.Terms"
		r.errMsg = err.Error()
		return r
	}
	if terms == nil {
		r.component = "Fields.Terms"
		r.errMsg = "terms for 'body' is nil"
		return r
	}

	te, err := terms.GetIterator()
	if err != nil {
		r.component = "Terms.GetIterator"
		r.errMsg = err.Error()
		return r
	}
	if te == nil {
		r.component = "Terms.GetIterator"
		r.errMsg = "term enum is nil"
		return r
	}
	term, err := te.Next()
	if err != nil {
		r.component = "TermsEnum.Next"
		r.errMsg = err.Error()
		return r
	}
	if term == nil {
		r.component = "TermsEnum.Next"
		r.errMsg = "no terms in field"
		return r
	}

	r.ok = true
	r.component = "postings-terms"
	r.detail = fmt.Sprintf("first_term=%s", term.Text())
	return r
}

// ------------------------------------------------------------------
// 2. Stored fields read-back
// ------------------------------------------------------------------
type auditStoredFieldVisitor struct {
	fields int
}

func (v *auditStoredFieldVisitor) StringField(field string, value string)   { v.fields++ }
func (v *auditStoredFieldVisitor) BinaryField(field string, value []byte)    { v.fields++ }
func (v *auditStoredFieldVisitor) IntField(field string, value int)         { v.fields++ }
func (v *auditStoredFieldVisitor) LongField(field string, value int64)      { v.fields++ }
func (v *auditStoredFieldVisitor) FloatField(field string, value float32)   { v.fields++ }
func (v *auditStoredFieldVisitor) DoubleField(field string, value float64)  { v.fields++ }

func testStoredFields(t *testing.T, seed int64) scenarioResult {
	var r scenarioResult
	defer func() {
		if rec := recover(); rec != nil {
			r.ok = false
			r.errMsg = fmt.Sprintf("panic: %v", rec)
		}
	}()

	dirPath := generateDir(t, "stored-fields-format", seed)
	dir, err := store.NewSimpleFSDirectory(dirPath)
	if err != nil {
		r.component = "store.NewSimpleFSDirectory"
		r.errMsg = err.Error()
		return r
	}
	defer dir.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		r.component = "index.OpenDirectoryReader"
		r.errMsg = err.Error()
		return r
	}
	defer reader.Close()

	segs := reader.GetSegmentReaders()
	if len(segs) == 0 {
		r.component = "SegmentReader"
		r.errMsg = "no segments"
		return r
	}
	seg := segs[0]

	sf, err := seg.StoredFields()
	if err != nil {
		r.component = "SegmentReader.StoredFields"
		r.errMsg = err.Error()
		return r
	}
	if sf == nil {
		r.component = "SegmentReader.StoredFields"
		r.errMsg = "StoredFields is nil"
		return r
	}

	visitor := &auditStoredFieldVisitor{}
	if err := sf.Document(0, visitor); err != nil {
		r.component = "StoredFields.Document"
		r.errMsg = err.Error()
		return r
	}
	if visitor.fields == 0 {
		r.component = "StoredFields.Document"
		r.errMsg = "document has no fields"
		return r
	}

	r.ok = true
	r.component = "stored-fields"
	r.detail = fmt.Sprintf("doc0_fields=%d", visitor.fields)
	return r
}

// ------------------------------------------------------------------
// 3. TermVectors read-back
// ------------------------------------------------------------------
func testTermVectors(t *testing.T, seed int64) scenarioResult {
	var r scenarioResult
	defer func() {
		if rec := recover(); rec != nil {
			r.ok = false
			r.errMsg = fmt.Sprintf("panic: %v", rec)
		}
	}()

	dirPath := generateDir(t, "term-vectors-format", seed)
	dir, err := store.NewSimpleFSDirectory(dirPath)
	if err != nil {
		r.component = "store.NewSimpleFSDirectory"
		r.errMsg = err.Error()
		return r
	}
	defer dir.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		r.component = "index.OpenDirectoryReader"
		r.errMsg = err.Error()
		return r
	}
	defer reader.Close()

	segs := reader.GetSegmentReaders()
	if len(segs) == 0 {
		r.component = "SegmentReader"
		r.errMsg = "no segments"
		return r
	}
	seg := segs[0]

	// Use TermVectors() interface method (not GetTermVectors on LeafReader)
	tv, err := seg.TermVectors()
	if err != nil {
		r.component = "SegmentReader.TermVectors"
		r.errMsg = err.Error()
		return r
	}
	if tv == nil {
		r.component = "SegmentReader.TermVectors"
		r.errMsg = "TermVectors is nil"
		return r
	}

	tvFields, err := tv.Get(0)
	if err != nil {
		r.component = "TermVectors.Get"
		r.errMsg = err.Error()
		return r
	}
	if tvFields == nil {
		r.component = "TermVectors.Get"
		r.errMsg = "Get returned nil fields"
		return r
	}

	it, err := tvFields.Iterator()
	if err != nil {
		r.component = "Fields.Iterator"
		r.errMsg = err.Error()
		return r
	}
	if it == nil {
		r.component = "Fields.Iterator"
		r.errMsg = "fields iterator is nil"
		return r
	}
	fieldName, err := it.Next()
	if err != nil {
		r.component = "FieldIterator.Next"
		r.errMsg = err.Error()
		return r
	}
	if fieldName == "" {
		r.component = "FieldIterator.Next"
		r.errMsg = "no fields in term vectors"
		return r
	}

	r.ok = true
	r.component = "term-vectors"
	r.detail = fmt.Sprintf("first_field=%s", fieldName)
	return r
}

// ------------------------------------------------------------------
// 4. DocValues read-back (numeric, no updates)
// ------------------------------------------------------------------
func testDocValuesNumeric(t *testing.T, seed int64) scenarioResult {
	var r scenarioResult
	defer func() {
		if rec := recover(); rec != nil {
			r.ok = false
			r.errMsg = fmt.Sprintf("panic: %v", rec)
		}
	}()

	dirPath := generateDir(t, "doc-values-format", seed)
	dir, err := store.NewSimpleFSDirectory(dirPath)
	if err != nil {
		r.component = "store.NewSimpleFSDirectory"
		r.errMsg = err.Error()
		return r
	}
	defer dir.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		r.component = "index.OpenDirectoryReader"
		r.errMsg = err.Error()
		return r
	}
	defer reader.Close()

	segs := reader.GetSegmentReaders()
	if len(segs) == 0 {
		r.component = "SegmentReader"
		r.errMsg = "no segments"
		return r
	}
	seg := segs[0]

	dv, err := seg.GetNumericDocValues("dv_num")
	if err != nil {
		r.component = "SegmentReader.GetNumericDocValues"
		r.errMsg = err.Error()
		return r
	}
	if dv == nil {
		r.component = "SegmentReader.GetNumericDocValues"
		r.errMsg = "NumericDocValues for 'dv_num' is nil"
		return r
	}

	has, err := dv.AdvanceExact(0)
	if err != nil {
		r.component = "NumericDocValues.AdvanceExact"
		r.errMsg = err.Error()
		return r
	}
	if !has {
		r.component = "NumericDocValues.AdvanceExact"
		r.errMsg = "doc 0 has no numeric value"
		return r
	}
	val, err := dv.LongValue()
	if err != nil {
		r.component = "NumericDocValues.LongValue"
		r.errMsg = err.Error()
		return r
	}

	r.ok = true
	r.component = "docvalues-numeric"
	r.detail = fmt.Sprintf("doc0_val=%d", val)
	return r
}

// ------------------------------------------------------------------
// 5. Multi-segment index open + NumDocs/MaxDoc
// ------------------------------------------------------------------
func testMultiSegment(t *testing.T, seed int64) scenarioResult {
	var r scenarioResult
	defer func() {
		if rec := recover(); rec != nil {
			r.ok = false
			r.errMsg = fmt.Sprintf("panic: %v", rec)
		}
	}()

	dirPath := generateDir(t, "combined-multi-segment-index-search", seed)
	dir, err := store.NewSimpleFSDirectory(dirPath)
	if err != nil {
		r.component = "store.NewSimpleFSDirectory"
		r.errMsg = err.Error()
		return r
	}
	defer dir.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		r.component = "index.OpenDirectoryReader"
		r.errMsg = err.Error()
		return r
	}
	defer reader.Close()

	numDocs := reader.NumDocs()
	maxDoc := reader.MaxDoc()
	segs := reader.GetSegmentReaders()

	const wantDocs = 18
	const wantSegs = 3
	if numDocs != wantDocs {
		r.component = "DirectoryReader.NumDocs"
		r.errMsg = fmt.Sprintf("NumDocs=%d, want %d", numDocs, wantDocs)
		return r
	}
	if maxDoc != wantDocs {
		r.component = "DirectoryReader.MaxDoc"
		r.errMsg = fmt.Sprintf("MaxDoc=%d, want %d", maxDoc, wantDocs)
		return r
	}
	if len(segs) != wantSegs {
		r.component = "DirectoryReader.Segments"
		r.errMsg = fmt.Sprintf("segments=%d, want %d", len(segs), wantSegs)
		return r
	}

	r.ok = true
	r.component = "multi-segment"
	r.detail = fmt.Sprintf("numDocs=%d maxDoc=%d segments=%d", numDocs, maxDoc, len(segs))
	return r
}
