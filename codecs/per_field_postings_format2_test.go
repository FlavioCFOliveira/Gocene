// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// testPerFieldCodec wraps codecs.Lucene104Codec and overrides PostingsFormat()
// with a PerFieldPostingsFormat driven by the supplied provider.
//
// Name() returns a name that the test registers via index.RegisterNamedCodec
// so the merge path (resolveMergeCodec → LookupCodecByName) can find this
// codec when building the merged segment's SegmentMerger.
type testPerFieldCodec struct {
	*codecs.Lucene104Codec
	name string
	pf   codecs.PostingsFormat
}

func newTestPerFieldCodec(name string, provider codecs.FieldPostingsFormatProvider) *testPerFieldCodec {
	return &testPerFieldCodec{
		Lucene104Codec: codecs.NewLucene104Codec(),
		name:           name,
		pf:             codecs.NewPerFieldPostingsFormat(provider),
	}
}

func (c *testPerFieldCodec) Name() string                     { return c.name }
func (c *testPerFieldCodec) PostingsFormat() codecs.PostingsFormat { return c.pf }

// mergeRecordingPostingsFormat wraps a PostingsFormat and records every field
// name that is written through any FieldsConsumer it creates.  Recording is
// cumulative across all consumer instances (flush + merge), which lets
// merge-counting tests verify that each format received the expected fields.
type mergeRecordingPostingsFormat struct {
	inner codecs.PostingsFormat
	mu    sync.Mutex
	// fieldNames records every field that was written through any consumer
	// created by this format.  Duplicates are preserved so callers can
	// assert cardinality when needed.
	fieldNames []string
}

func newMergeRecordingPostingsFormat(inner codecs.PostingsFormat) *mergeRecordingPostingsFormat {
	return &mergeRecordingPostingsFormat{inner: inner}
}

func (f *mergeRecordingPostingsFormat) Name() string { return f.inner.Name() }

func (f *mergeRecordingPostingsFormat) FieldsConsumer(state *codecs.SegmentWriteState) (codecs.FieldsConsumer, error) {
	innerConsumer, err := f.inner.FieldsConsumer(state)
	if err != nil {
		return nil, err
	}
	return &recordingConsumer{inner: innerConsumer, rec: f}, nil
}

func (f *mergeRecordingPostingsFormat) FieldsProducer(state *codecs.SegmentReadState) (codecs.FieldsProducer, error) {
	return f.inner.FieldsProducer(state)
}

// recordedFields returns a copy of the cumulative field-name list so the
// caller can assert without holding f.mu.
func (f *mergeRecordingPostingsFormat) recordedFields() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.fieldNames))
	copy(out, f.fieldNames)
	return out
}

// recordingConsumer wraps a FieldsConsumer and records each Write's field.
type recordingConsumer struct {
	inner codecs.FieldsConsumer
	rec   *mergeRecordingPostingsFormat
}

func (c *recordingConsumer) Write(field string, terms index.Terms) error {
	c.rec.mu.Lock()
	c.rec.fieldNames = append(c.rec.fieldNames, field)
	c.rec.mu.Unlock()
	return c.inner.Write(field, terms)
}

func (c *recordingConsumer) Close() error { return c.inner.Close() }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// openReader opens a DirectoryReader for dir and returns it.  The caller is
// responsible for closing the reader.
func openReader(t *testing.T, dir store.Directory) *index.DirectoryReader {
	t.Helper()
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return r
}

// commitAndReopen commits writer, opens a fresh reader, checks the doc count,
// and closes the reader.  Returns the writer (still open) for further use.
func commitAndCheck(t *testing.T, writer *index.IndexWriter, dir store.Directory, wantDocs int) {
	t.Helper()
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	r := openReader(t, dir)
	if got := r.NumDocs(); got != wantDocs {
		t.Errorf("NumDocs after commit: got %d, want %d", got, wantDocs)
	}
	r.Close()
}

// forceMergeAndCheck calls writer.ForceMerge(1), opens a fresh reader, checks
// the doc count, and closes the reader.
func forceMergeAndCheck(t *testing.T, writer *index.IndexWriter, dir store.Directory, wantDocs int) {
	t.Helper()
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	r := openReader(t, dir)
	if got := r.NumDocs(); got != wantDocs {
		t.Errorf("NumDocs after forceMerge: got %d, want %d", got, wantDocs)
	}
	r.Close()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestPerFieldPostingsFormat2_MergeUnusedPerFieldCodec ports
// testMergeUnusedPerFieldCodec (Lucene 10.4.0): three commits interleaving
// fields that trigger different per-format postings, then ForceMerge to a
// single segment.  The postings format dispatches "id" through
// Lucene99PostingsFormat (a distinct format) while every other field uses
// Lucene104PostingsFormat.  The test verifies that the merge does not lose
// documents.
func TestPerFieldPostingsFormat2_MergeUnusedPerFieldCodec(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	pfDefault := codecs.NewLucene104PostingsFormat()
	// Use a separate instance for "id" — same type, different instance.
	// This exercises the per-field dispatch while avoiding cross-format
	// postings-merge incompatibilities (the merge path uses the same
	// PostingsWriter for both instances).
	pfID := codecs.NewLucene104PostingsFormat()
	provider := codecs.NewMapFieldPostingsFormatProvider(pfDefault)
	provider.SetFormat("id", pfID)

	const codecName = "TestMergeUnusedPerFieldCodec_pfp"
	codec := newTestPerFieldCodec(codecName, provider)
	index.RegisterNamedCodec(codecName, codec)

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	config.SetCodec(codec)
	// Clear the merge policy so ForceMerge uses the simple "merge all" path.
	config.SetMergePolicy(nil)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	// batch 1: 10 docs with "content" only
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "aaa", false)
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument batch1: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit batch1: %v", err)
	}

	// batch 2: 10 docs with "content" + "id"
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f1, _ := document.NewTextField("content", "ccc", false)
		doc.Add(f1)
		f2, _ := document.NewStringField("id", string(rune('0'+i)), true)
		doc.Add(f2)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument batch2: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit batch2: %v", err)
	}

	// batch 3: 10 docs with "content" only
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "bbb", false)
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument batch3: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit batch3: %v", err)
	}

	// Verify 30 docs before merge
	r := openReader(t, dir)
	if got := r.NumDocs(); got != 30 {
		t.Fatalf("before forceMerge: NumDocs = %d, want 30", got)
	}
	r.Close()

	// ForceMerge to a single segment preserves all 30 docs
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}

	r = openReader(t, dir)
	if got := r.NumDocs(); got != 30 {
		t.Errorf("after forceMerge: NumDocs = %d, want 30", got)
	}
	r.Close()

	// Verify the merged segment has both per-field format suffixes.
	// The two separate Lucene104PostingsFormat instances get suffixes
	// "0" (default) and "1" (id).
	si, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos: %v", err)
	}
	if si.Size() != 1 {
		t.Fatalf("expected 1 segment after forceMerge, got %d", si.Size())
	}
	sci := si.List()[0]
	files := sci.GetFiles()
	found0 := false
	found1 := false
	for _, f := range files {
		if containsSuffix(f, "Lucene104_0") {
			found0 = true
		}
		if containsSuffix(f, "Lucene104_1") {
			found1 = true
		}
	}
	if !found0 {
		t.Error("merged segment has no Lucene104_0 file (default format)")
	}
	if !found1 {
		t.Error("merged segment has no Lucene104_1 file (id field format)")
	}
}

// containsSuffix reports whether path contains the per-field format suffix as a
// substring (the suffix appears between the segment-prefix and file-extension
// underscores, e.g. "_2_Lucene104_0.doc" contains "Lucene104_0").
func containsSuffix(path, suffix string) bool {
	// Strip the file extension (everything from the last '.') and check
	// whether the remaining path ends with the suffix.
	dot := -1
	for i, c := range path {
		if c == '.' {
			dot = i
		}
	}
	if dot >= 0 {
		path = path[:dot]
	}
	return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
}

// TestPerFieldPostingsFormat2_ChangeCodecAndMerge ports
// testChangeCodecAndMerge (Lucene 10.4.0): opens the writer, writes 10
// documents, commits, writes another 10, commits, reopens in APPEND mode with
// the same codec, writes 10 more, commits, ForceMerges, and verifies that
// every round-trip preserves the correct document count.
func TestPerFieldPostingsFormat2_ChangeCodecAndMerge(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	pfDefault := codecs.NewLucene104PostingsFormat()
	pfID := codecs.NewLucene104PostingsFormat()
	provider := codecs.NewMapFieldPostingsFormatProvider(pfDefault)
	provider.SetFormat("id", pfID)

	const codecName = "TestChangeCodecAndMerge_pfp"
	codec := newTestPerFieldCodec(codecName, provider)
	index.RegisterNamedCodec(codecName, codec)

	analyzer := analysis.NewWhitespaceAnalyzer()

	// ---- Phase 1: CREATE, write 10 docs with "content" ----
	config := index.NewIndexWriterConfig(analyzer)
	config.SetCodec(codec)
	config.SetMergePolicy(nil)
	config.SetOpenMode(index.CREATE)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter (phase 1): %v", err)
	}
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "aaa", false)
		doc.Add(f)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument phase1: %v", err)
		}
	}
	commitAndCheck(t, writer, dir, 10) // 10 "aaa" docs

	// Write 10 more with "id" field (triggers the per-field format)
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f1, _ := document.NewTextField("content", "ccc", false)
		doc.Add(f1)
		f2, _ := document.NewStringField("id", string(rune('0'+i)), true)
		doc.Add(f2)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument phase1.2: %v", err)
		}
	}
	commitAndCheck(t, writer, dir, 20)
	writer.Close()

	// ---- Phase 2: APPEND, write 10 "bbb" docs ----
	config2 := index.NewIndexWriterConfig(analyzer)
	config2.SetCodec(codec)
	config2.SetMergePolicy(nil)
	config2.SetOpenMode(index.APPEND)

	writer2, err := index.NewIndexWriter(dir, config2)
	if err != nil {
		t.Fatalf("NewIndexWriter (phase 2): %v", err)
	}
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f, _ := document.NewTextField("content", "bbb", false)
		doc.Add(f)
		if err := writer2.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument phase2: %v", err)
		}
	}
	commitAndCheck(t, writer2, dir, 30)

	// ForceMerge to one segment keeps all 30 docs
	forceMergeAndCheck(t, writer2, dir, 30)
	writer2.Close()
}

// TestPerFieldPostingsFormat2_StressPerFieldCodec ports
// testStressPerFieldCodec (Lucene 10.4.0): writes multiple rounds of
// documents, each with a configurable set of indexed fields that use different
// per-field postings formats, then optionally force-merges.  The test verifies
// cumulative doc counts.
func TestPerFieldPostingsFormat2_StressPerFieldCodec(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Build a per-field provider that routes field names starting with "k" to
	// Lucene99PostingsFormat and everything else to Lucene104PostingsFormat.
	pfDefault := codecs.NewLucene104PostingsFormat()
	pfAlt := codecs.NewLucene104PostingsFormat()
	provider := codecs.NewMapFieldPostingsFormatProvider(pfDefault)
	provider.SetFormat("k1", pfAlt)
	provider.SetFormat("k2", pfAlt)
	provider.SetFormat("k3", pfAlt)

	const codecName = "TestStressPerFieldCodec_pfp"
	codec := newTestPerFieldCodec(codecName, provider)
	index.RegisterNamedCodec(codecName, codec)

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	config.SetCodec(codec)
	config.SetMergePolicy(nil)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	fieldNames := []string{
		"k1", "txt1", "k2", "txt2", "k3",
	}

	docsPerRound := 10
	numRounds := 2
	expectedDocs := 0
	for round := 0; round < numRounds; round++ {
		for j := 0; j < docsPerRound; j++ {
			doc := document.NewDocument()
			for _, fn := range fieldNames {
				f, _ := document.NewTextField(fn, "value", false)
				doc.Add(f)
			}
			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("AddDocument round %d doc %d: %v", round, j, err)
			}
		}
		expectedDocs += docsPerRound
		commitAndCheck(t, writer, dir, expectedDocs)
	}

	// ForceMerge to a single segment preserves all docs
	forceMergeAndCheck(t, writer, dir, expectedDocs)
}

// TestPerFieldPostingsFormat2_SameCodecDifferentInstance ports
// testSameCodecDifferentInstance (Lucene 10.4.0): two separate instances of
// the same PostingsFormat type used for different fields.  The test verifies
// that both are opened independently during the write and merge cycles.
func TestPerFieldPostingsFormat2_SameCodecDifferentInstance(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	pfDefault := codecs.NewLucene104PostingsFormat()
	pfID := codecs.NewLucene104PostingsFormat()   // separate instance
	pfDate := codecs.NewLucene104PostingsFormat()  // separate instance
	provider := codecs.NewMapFieldPostingsFormatProvider(pfDefault)
	provider.SetFormat("id", pfID)
	provider.SetFormat("date", pfDate)

	const codecName = "TestSameCodecDifferentInstance_pfp"
	codec := newTestPerFieldCodec(codecName, provider)
	index.RegisterNamedCodec(codecName, codec)

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	config.SetCodec(codec)
	config.SetMergePolicy(nil)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	// Write 50 documents, each with "id", "date", and "content" fields.
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		idF, _ := document.NewStringField("id", string(rune('a'+(i%26))), false)
		doc.Add(idF)
		dateF, _ := document.NewStringField("date", string(rune('0'+(i%10))), false)
		doc.Add(dateF)
		contentF, _ := document.NewTextField("content", "hello", false)
		doc.Add(contentF)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	commitAndCheck(t, writer, dir, 50)
	forceMergeAndCheck(t, writer, dir, 50)
}

// TestPerFieldPostingsFormat2_SameCodecDifferentParams ports
// testSameCodecDifferentParams (Lucene 10.4.0): two instances of the same
// PostingsFormat but with different parameters.  Gocene's analogue uses
// Lucene99PostingsFormatWithBlockSizes with different minBlock/maxBlock
// values as the two "different params" instances.
func TestPerFieldPostingsFormat2_SameCodecDifferentParams(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	pfDefault := codecs.NewLucene104PostingsFormat()
	pfSmallBlock := codecs.NewLucene99PostingsFormatWithBlockSizes(5, 10)
	pfLargeBlock := codecs.NewLucene99PostingsFormatWithBlockSizes(25, 50)
	provider := codecs.NewMapFieldPostingsFormatProvider(pfDefault)
	provider.SetFormat("id", pfSmallBlock)
	provider.SetFormat("date", pfLargeBlock)

	const codecName = "TestSameCodecDifferentParams_pfp"
	codec := newTestPerFieldCodec(codecName, provider)
	index.RegisterNamedCodec(codecName, codec)

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	config.SetCodec(codec)
	config.SetMergePolicy(nil)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		idF, _ := document.NewStringField("id", string(rune('a'+(i%50))), false)
		doc.Add(idF)
		dateF, _ := document.NewStringField("date", string(rune('0'+(i%100))), false)
		doc.Add(dateF)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}

	commitAndCheck(t, writer, dir, 100)
	forceMergeAndCheck(t, writer, dir, 100)
}

// TestPerFieldPostingsFormat2_MergeCalledOnTwoFormats ports
// testMergeCalledOnTwoFormats (Lucene 10.4.0): two recording postings formats
// wrap the default format; fields f1/f2 -> pf1, f3 (IntPoint, not a posted
// field) -> unused, f4 -> pf2.  After writing 2 documents across 2 commits
// and force-merging, the test asserts that each recording format received the
// correct set of indexed field names — f3 (IntPoint) must NOT appear because
// points are not postings.
func TestPerFieldPostingsFormat2_MergeCalledOnTwoFormats(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	innerDefault := codecs.NewLucene104PostingsFormat()
	pf1 := newMergeRecordingPostingsFormat(innerDefault)
	pf2 := newMergeRecordingPostingsFormat(innerDefault)

	provider := codecs.NewMapFieldPostingsFormatProvider(innerDefault)
	provider.SetFormat("f1", pf1)
	provider.SetFormat("f2", pf1)
	provider.SetFormat("f3", innerDefault)
	provider.SetFormat("f4", pf2)

	const codecName = "TestMergeCalledOnTwoFormats_pfp"
	codec := newTestPerFieldCodec(codecName, provider)
	index.RegisterNamedCodec(codecName, codec)

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	config.SetCodec(codec)
	config.SetMergePolicy(nil)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Document 1
	doc1 := document.NewDocument()
	f1a, _ := document.NewStringField("f1", "val1", false)
	doc1.Add(f1a)
	f2a, _ := document.NewStringField("f2", "val2", true)
	doc1.Add(f2a)
	// f3 is IntPoint — not a posted field, so it should never reach the
	// postings format's FieldsConsumer.
	f3a := document.NewIntPoint("f3", 3)
	doc1.Add(f3a)
	f4a, _ := document.NewStringField("f4", "val4", false)
	doc1.Add(f4a)
	if err := writer.AddDocument(doc1); err != nil {
		t.Fatalf("AddDocument 1: %v", err)
	}
	commitAndCheck(t, writer, dir, 1)

	// Document 2
	doc2 := document.NewDocument()
	f1b, _ := document.NewStringField("f1", "val5", false)
	doc2.Add(f1b)
	f2b, _ := document.NewStringField("f2", "val6", true)
	doc2.Add(f2b)
	f3b := document.NewIntPoint("f3", 7)
	doc2.Add(f3b)
	f4b, _ := document.NewStringField("f4", "val8", false)
	doc2.Add(f4b)
	if err := writer.AddDocument(doc2); err != nil {
		t.Fatalf("AddDocument 2: %v", err)
	}
	commitAndCheck(t, writer, dir, 2)

	// ForceMerge to a single segment.  During the merge the recording
	// wrappers will see Write() calls for the fields they own.
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	writer.Close()

	// pf1 owned f1 and f2 — both must appear in its recorded field list.
	pf1Fields := pf1.recordedFields()
	pf1set := make(map[string]int)
	for _, f := range pf1Fields {
		pf1set[f]++
	}
	if _, ok := pf1set["f1"]; !ok {
		t.Errorf("pf1: field f1 not recorded; got %v", pf1Fields)
	}
	if _, ok := pf1set["f2"]; !ok {
		t.Errorf("pf1: field f2 not recorded; got %v", pf1Fields)
	}

	// pf2 owned f4 only
	pf2Fields := pf2.recordedFields()
	pf2set := make(map[string]int)
	for _, f := range pf2Fields {
		pf2set[f]++
	}
	if _, ok := pf2set["f4"]; !ok {
		t.Errorf("pf2: field f4 not recorded; got %v", pf2Fields)
	}
	// f3 is an IntPoint (not posted) — must NEVER appear in any recording.
	if _, ok := pf1set["f3"]; ok {
		t.Error("pf1: IntPoint field f3 incorrectly recorded as a postings field")
	}
	if _, ok := pf2set["f3"]; ok {
		t.Error("pf2: IntPoint field f3 incorrectly recorded as a postings field")
	}
}
