// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Round-trip coverage for the KNN vectors (HNSW) leg of the segment merge
// (rmp #14/#114): two committed segments with float vectors are merged and the
// merged segment's vectors are read back, proving each vector is preserved at
// its remapped docID.

package index_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

func collectFloatVectors(t *testing.T, fvv index.FloatVectorValues, maxDoc int) map[int][]float32 {
	t.Helper()
	out := map[int][]float32{}
	for {
		d, err := fvv.NextDoc()
		if err != nil {
			t.Fatalf("vector NextDoc: %v", err)
		}
		if d < 0 || d >= maxDoc {
			break
		}
		v, err := fvv.Get(d)
		if err != nil {
			t.Fatalf("vector Get(%d): %v", d, err)
		}
		cp := make([]float32, len(v))
		copy(cp, v)
		out[d] = cp
	}
	return out
}

func TestSegmentMerger_VectorsRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	addVec := func(v []float32) {
		doc := document.NewDocument()
		f, err := document.NewKnnFloatVectorFieldEuclidean("vec", v)
		if err != nil {
			t.Fatalf("NewKnnFloatVectorField: %v", err)
		}
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	// Segment 1: merged docs 0,1.
	addVec([]float32{1, 0})
	addVec([]float32{0, 1})
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg1: %v", err)
	}
	// Segment 2: merged docs 2,3.
	addVec([]float32{1, 1})
	addVec([]float32{0.5, 0.25})
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg2: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	segReaders := reader.GetSegmentReaders()
	if len(segReaders) < 2 {
		t.Fatalf("expected >= 2 segments, got %d", len(segReaders))
	}

	// Expected merged vectors: each segment's vectors at the concatenated docIDs.
	want := map[int][]float32{}
	base := 0
	var codecReaders []*index.CodecReader
	total := 0
	for _, sr := range segReaders {
		fvv, err := sr.GetFloatVectorValues("vec")
		if err != nil || fvv == nil {
			t.Fatalf("segment GetFloatVectorValues: fvv=%v err=%v", fvv, err)
		}
		for d, v := range collectFloatVectors(t, fvv, sr.MaxDoc()) {
			want[base+d] = v
		}
		base += sr.MaxDoc()

		cr := index.NewCodecReader(sr.GetCoreReaders(), sr.GetLiveDocs(), sr.NumDocs())
		cr.GetSegmentInfo().SetDocCount(sr.MaxDoc())
		codecReaders = append(codecReaders, cr)
		total += sr.NumDocs()
	}
	if len(want) != 4 {
		t.Fatalf("expected 4 source vectors, got %d", len(want))
	}

	mergedSI := index.NewSegmentInfo("_merged", total, dir)
	mergedSI.SetCodec(index.GetDefaultCodec().Name())
	merger, err := index.NewSegmentMerger(codecReaders, mergedSI, nil, dir, store.IOContext{Context: store.ContextMerge})
	if err != nil {
		t.Fatalf("NewSegmentMerger: %v", err)
	}
	ms, err := merger.Merge()
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}

	// Read the merged vectors back.
	codec := index.GetDefaultCodec()
	rs := &index.SegmentReadState{Directory: dir, SegmentInfo: mergedSI, FieldInfos: ms.MergeFieldInfos}
	vr, err := codec.KnnVectorsFormat().FieldsReader(rs)
	if err != nil {
		t.Fatalf("KnnVectors FieldsReader: %v", err)
	}
	defer vr.Close()

	delegate, ok := vr.(interface {
		FloatVectorValues(field string) (index.FloatVectorValues, error)
	})
	if !ok {
		t.Fatalf("KnnVectorsReader %T has no FloatVectorValues", vr)
	}
	mfvv, err := delegate.FloatVectorValues("vec")
	if err != nil || mfvv == nil {
		t.Fatalf("merged FloatVectorValues: fvv=%v err=%v", mfvv, err)
	}
	got := collectFloatVectors(t, mfvv, total)

	if len(got) != len(want) {
		t.Fatalf("merged vectors count = %d, want %d", len(got), len(want))
	}
	for d, exp := range want {
		gv, ok := got[d]
		if !ok {
			t.Errorf("merged vectors missing doc %d", d)
			continue
		}
		if len(gv) != len(exp) {
			t.Errorf("merged vector doc %d len = %d, want %d", d, len(gv), len(exp))
			continue
		}
		for k := range exp {
			if gv[k] != exp[k] {
				t.Errorf("merged vector doc %d = %v, want %v", d, gv, exp)
				break
			}
		}
	}
}

// segmentVectorFilesPresent reports, for the single merged segment, whether the
// three HNSW on-disk artefacts produced by Lucene99HnswVectorsFormat exist as
// standalone files: the raw flat vector data (.vec), the HNSW metadata (.vem)
// and the HNSW graph index (.vex). Compound files must be disabled on the
// writer for these to surface in ListAll (otherwise they are packed into .cfs).
func segmentVectorFilesPresent(t *testing.T, dir store.Directory) (vec, vem, vex bool) {
	t.Helper()
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	for _, f := range files {
		switch {
		case strings.HasSuffix(f, ".vec"):
			vec = true
		case strings.HasSuffix(f, ".vem"):
			vem = true
		case strings.HasSuffix(f, ".vex"):
			vex = true
		}
	}
	return vec, vem, vex
}

// docIDByStoredID builds a stored-id -> global-docID map over the reopened
// reader so KNN/round-trip assertions can be expressed in terms of the stable
// "id" field rather than post-merge docIDs (which are an implementation detail
// of the merge remap).
func docIDByStoredID(t *testing.T, s *search.IndexSearcher, maxDoc int) map[string]int {
	t.Helper()
	out := map[string]int{}
	for d := 0; d < maxDoc; d++ {
		doc, err := s.Doc(d)
		if err != nil {
			t.Fatalf("Doc(%d): %v", d, err)
		}
		f := doc.Get("id")
		if f == nil {
			t.Fatalf("doc %d missing stored id field", d)
		}
		out[f.StringValue()] = d
	}
	return out
}

// TestForceMerge_FloatVectorsSparseDeletedRoundTrip drives the REAL
// IndexWriter.ForceMerge path (not the low-level SegmentMerger) over two
// committed segments carrying FLOAT KnnVector fields, including:
//   - a SPARSE document (id "s") that carries no vector field at all, and
//   - a DELETED document (id "2") removed before the merge.
//
// After force-merging to a single segment it proves, against the reopened
// reader: the deleted doc is gone, the sparse doc has no vector, every
// surviving vector round-trips at its remapped docID, the .vec/.vem/.vex
// artefacts exist on disk, and a KnnFloatVectorQuery returns the correct
// nearest documents. This is the end-to-end acceptance proof for rmp #20.
func TestForceMerge_FloatVectorsSparseDeletedRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	// Disable compound files so the HNSW artefacts surface as standalone
	// files in the merged segment and the .vec/.vem/.vex check is meaningful.
	cfg.SetUseCompoundFile(false)
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// addVecDoc adds a document with a stored "id" field and, when vec != nil,
	// a float KnnVector field "fvec". A nil vec yields a SPARSE doc.
	addVecDoc := func(id string, vec []float32) {
		doc := document.NewDocument()
		idF, err := document.NewStringField("id", id, true)
		if err != nil {
			t.Fatalf("id field: %v", err)
		}
		doc.Add(idF)
		if vec != nil {
			f, err := document.NewKnnFloatVectorFieldEuclidean("fvec", vec)
			if err != nil {
				t.Fatalf("NewKnnFloatVectorFieldEuclidean: %v", err)
			}
			doc.Add(f)
		}
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %q: %v", id, err)
		}
	}

	// Segment 1: ids 1,2 (both with vectors).
	addVecDoc("1", []float32{1, 0})
	addVecDoc("2", []float32{0, 1})
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg1: %v", err)
	}
	// Segment 2: id 3 (vector), id s (SPARSE — no vector field), id 4 (vector).
	// The sparse doc sits between two vector-bearing docs so the merge must
	// correctly skip a gap in the middle of a segment's doc space.
	addVecDoc("3", []float32{1, 1})
	addVecDoc("s", nil)
	addVecDoc("4", []float32{0.5, 0.25})
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg2: %v", err)
	}

	// Delete id 2 BEFORE merging: ForceMerge must compact it out.
	if err := w.DeleteDocuments(index.NewTerm("id", "2")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}

	if c := w.GetSegmentCount(); c < 2 {
		t.Fatalf("expected multiple segments before merge, got %d", c)
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if c := w.GetSegmentCount(); c != 1 {
		t.Fatalf("segment count after ForceMerge = %d, want 1", c)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The merged segment must carry the three HNSW artefacts on disk.
	if vec, vem, vex := segmentVectorFilesPresent(t, dir); !vec || !vem || !vex {
		t.Fatalf("merged segment missing HNSW artefacts: .vec=%v .vem=%v .vex=%v", vec, vem, vex)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if got := len(reader.GetSegmentReaders()); got != 1 {
		t.Fatalf("reader segment count = %d, want 1", got)
	}
	// 4 surviving docs: ids 1,3,s,4 (id 2 deleted).
	if got := reader.NumDocs(); got != 4 {
		t.Fatalf("NumDocs after merge = %d, want 4", got)
	}
	if got := reader.MaxDoc(); got != 4 {
		t.Fatalf("MaxDoc after merge = %d, want 4 (deletes compacted)", got)
	}
	if got := reader.NumDeletedDocs(); got != 0 {
		t.Fatalf("NumDeletedDocs after merge = %d, want 0", got)
	}

	s := search.NewIndexSearcher(reader)
	byID := docIDByStoredID(t, s, reader.MaxDoc())
	for _, id := range []string{"1", "3", "s", "4"} {
		if _, ok := byID[id]; !ok {
			t.Fatalf("surviving doc id %q absent after merge", id)
		}
	}
	if _, ok := byID["2"]; ok {
		t.Fatalf("deleted doc id 2 is still present after merge")
	}

	// Read back the merged segment's float vectors keyed by remapped docID.
	sr := reader.GetSegmentReaders()[0]
	fvv, err := sr.GetFloatVectorValues("fvec")
	if err != nil || fvv == nil {
		t.Fatalf("merged GetFloatVectorValues: fvv=%v err=%v", fvv, err)
	}
	got := collectFloatVectors(t, fvv, sr.MaxDoc())

	// Exactly the three vector-bearing survivors must have a vector.
	if len(got) != 3 {
		t.Fatalf("merged float vectors count = %d, want 3", len(got))
	}
	wantByID := map[string][]float32{
		"1": {1, 0},
		"3": {1, 1},
		"4": {0.5, 0.25},
	}
	for id, exp := range wantByID {
		d := byID[id]
		gv, ok := got[d]
		if !ok {
			t.Errorf("vector for id %q (doc %d) missing after merge", id, d)
			continue
		}
		if len(gv) != len(exp) {
			t.Errorf("vector id %q len = %d, want %d", id, len(gv), len(exp))
			continue
		}
		for k := range exp {
			if gv[k] != exp[k] {
				t.Errorf("vector id %q = %v, want %v", id, gv, exp)
				break
			}
		}
	}
	// The SPARSE doc must carry no vector.
	if _, ok := got[byID["s"]]; ok {
		t.Errorf("sparse doc id s (doc %d) unexpectedly has a vector", byID["s"])
	}

	// KNN query: nearest to {1,1} (exactly id 3) then {0.9,0.1} (nearest id 1).
	assertNearestFloat := func(target []float32, wantID string) {
		t.Helper()
		top, err := s.Search(search.NewKnnFloatVectorQuery("fvec", target, 1), 1)
		if err != nil {
			t.Fatalf("KNN search target=%v: %v", target, err)
		}
		if len(top.ScoreDocs) != 1 {
			t.Fatalf("KNN target=%v returned %d hits, want 1", target, len(top.ScoreDocs))
		}
		gotDoc := top.ScoreDocs[0].Doc
		if gotDoc != byID[wantID] {
			t.Fatalf("KNN target=%v nearest doc = %d, want id %q (doc %d)", target, gotDoc, wantID, byID[wantID])
		}
	}
	assertNearestFloat([]float32{1, 1}, "3")
	assertNearestFloat([]float32{0.9, 0.1}, "1")

	// A k=3 query must return all three vector-bearing docs (and never the
	// sparse or deleted doc).
	top, err := s.Search(search.NewKnnFloatVectorQuery("fvec", []float32{0.5, 0.5}, 3), 3)
	if err != nil {
		t.Fatalf("KNN k=3 search: %v", err)
	}
	if len(top.ScoreDocs) != 3 {
		t.Fatalf("KNN k=3 returned %d hits, want 3", len(top.ScoreDocs))
	}
	seen := map[int]bool{}
	for _, sd := range top.ScoreDocs {
		seen[sd.Doc] = true
	}
	for _, id := range []string{"1", "3", "4"} {
		if !seen[byID[id]] {
			t.Errorf("KNN k=3 result set missing id %q (doc %d)", id, byID[id])
		}
	}
	if seen[byID["s"]] {
		t.Errorf("KNN k=3 result set unexpectedly contains sparse doc s")
	}
}

// TestForceMerge_ByteVectorsSparseDeletedRoundTrip is the BYTE-encoding twin of
// TestForceMerge_FloatVectorsSparseDeletedRoundTrip: it drives the real
// ForceMerge path over two segments carrying BYTE KnnVector fields with a
// sparse doc and a deleted doc, then proves the same set of invariants
// (deleted gone, sparse vectorless, survivors round-trip at remapped docIDs,
// .vec/.vem/.vex on disk, KNN nearest correct) for the byte codec path.
func TestForceMerge_ByteVectorsSparseDeletedRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetUseCompoundFile(false)
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	addVecDoc := func(id string, vec []byte) {
		doc := document.NewDocument()
		idF, err := document.NewStringField("id", id, true)
		if err != nil {
			t.Fatalf("id field: %v", err)
		}
		doc.Add(idF)
		if vec != nil {
			f, err := document.NewKnnByteVectorFieldEuclidean("bvec", vec)
			if err != nil {
				t.Fatalf("NewKnnByteVectorFieldEuclidean: %v", err)
			}
			doc.Add(f)
		}
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %q: %v", id, err)
		}
	}

	// Segment 1: ids 1,2 (vectors).
	addVecDoc("1", []byte{10, 0})
	addVecDoc("2", []byte{0, 10})
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg1: %v", err)
	}
	// Segment 2: id 3 (vector), id s (SPARSE), id 4 (vector).
	addVecDoc("3", []byte{10, 10})
	addVecDoc("s", nil)
	addVecDoc("4", []byte{5, 2})
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg2: %v", err)
	}

	if err := w.DeleteDocuments(index.NewTerm("id", "2")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if c := w.GetSegmentCount(); c != 1 {
		t.Fatalf("segment count after ForceMerge = %d, want 1", c)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if vec, vem, vex := segmentVectorFilesPresent(t, dir); !vec || !vem || !vex {
		t.Fatalf("merged segment missing HNSW artefacts: .vec=%v .vem=%v .vex=%v", vec, vem, vex)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if got := reader.NumDocs(); got != 4 {
		t.Fatalf("NumDocs after merge = %d, want 4", got)
	}
	if got := reader.MaxDoc(); got != 4 {
		t.Fatalf("MaxDoc after merge = %d, want 4 (deletes compacted)", got)
	}

	s := search.NewIndexSearcher(reader)
	byID := docIDByStoredID(t, s, reader.MaxDoc())
	if _, ok := byID["2"]; ok {
		t.Fatalf("deleted doc id 2 is still present after merge")
	}

	sr := reader.GetSegmentReaders()[0]
	bvv, err := sr.GetByteVectorValues("bvec")
	if err != nil || bvv == nil {
		t.Fatalf("merged GetByteVectorValues: bvv=%v err=%v", bvv, err)
	}

	// Collect byte vectors keyed by remapped docID.
	gotBytes := map[int][]byte{}
	for {
		d, err := bvv.NextDoc()
		if err != nil {
			t.Fatalf("byte NextDoc: %v", err)
		}
		if d < 0 || d >= sr.MaxDoc() {
			break
		}
		v, err := bvv.Get(d)
		if err != nil {
			t.Fatalf("byte Get(%d): %v", d, err)
		}
		cp := make([]byte, len(v))
		copy(cp, v)
		gotBytes[d] = cp
	}
	if len(gotBytes) != 3 {
		t.Fatalf("merged byte vectors count = %d, want 3", len(gotBytes))
	}
	wantByID := map[string][]byte{
		"1": {10, 0},
		"3": {10, 10},
		"4": {5, 2},
	}
	for id, exp := range wantByID {
		d := byID[id]
		gv, ok := gotBytes[d]
		if !ok {
			t.Errorf("byte vector for id %q (doc %d) missing after merge", id, d)
			continue
		}
		if string(gv) != string(exp) {
			t.Errorf("byte vector id %q = %v, want %v", id, gv, exp)
		}
	}
	if _, ok := gotBytes[byID["s"]]; ok {
		t.Errorf("sparse doc id s (doc %d) unexpectedly has a byte vector", byID["s"])
	}

	// KNN over the byte field: nearest to {10,10} is id 3; nearest to {9,1} is id 1.
	assertNearestByte := func(target []byte, wantID string) {
		t.Helper()
		top, err := s.Search(search.NewKnnByteVectorQuery("bvec", target, 1), 1)
		if err != nil {
			t.Fatalf("byte KNN search target=%v: %v", target, err)
		}
		if len(top.ScoreDocs) != 1 {
			t.Fatalf("byte KNN target=%v returned %d hits, want 1", target, len(top.ScoreDocs))
		}
		if gotDoc := top.ScoreDocs[0].Doc; gotDoc != byID[wantID] {
			t.Fatalf("byte KNN target=%v nearest doc = %d, want id %q (doc %d)", target, gotDoc, wantID, byID[wantID])
		}
	}
	assertNearestByte([]byte{10, 10}, "3")
	assertNearestByte([]byte{9, 1}, "1")
}
