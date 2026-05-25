// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Round-trip tests for Lucene99HnswVectorsReader.
//
// Tests write vector data via Lucene99HnswVectorsWriter, then open the
// same segment via Lucene99HnswVectorsReader and verify:
//   - CheckIntegrity passes;
//   - GetGraph returns a graph whose Size / NumLevels / EntryNode match
//     the written data;
//   - level-0 neighbours can be iterated for every node.

package codecs

import (
	"crypto/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// hnswRTFieldInfos creates a FieldInfos containing the given fields.
func hnswRTFieldInfos(t *testing.T, fis ...*index.FieldInfo) *index.FieldInfos {
	t.Helper()
	out := index.NewFieldInfos()
	for _, fi := range fis {
		if err := out.Add(fi); err != nil {
			t.Fatalf("FieldInfos.Add(%q): %v", fi.Name(), err)
		}
	}
	return out
}

// hnswRTSegmentState creates an FS-backed directory and a matching
// SegmentWriteState/SegmentReadState for HNSW round-trip tests.
//
// SimpleFSDirectory is used (not ByteBuffersDirectory) to exercise the
// real ChecksumIndexOutput BE header path with the real file reader.
func hnswRTSegmentState(t *testing.T, maxDoc int, fis *index.FieldInfos) (
	*SegmentWriteState, *SegmentReadState, func()) {
	t.Helper()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	si := index.NewSegmentInfo("_hnswrt", maxDoc, dir)
	if err := si.SetID(id); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	ws := &SegmentWriteState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fis,
	}
	rs := &SegmentReadState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fis,
	}
	return ws, rs, func() { _ = dir.Close() }
}

// hnswRTFloatField returns a FieldInfo configured for float HNSW vectors.
func hnswRTFloatField(name string, number, dim int, sim index.VectorSimilarityFunction) *index.FieldInfo {
	return index.NewFieldInfo(name, number, index.FieldInfoOptions{
		VectorDimension:          dim,
		VectorEncoding:           index.VectorEncodingFloat32,
		VectorSimilarityFunction: sim,
	})
}

// collectNeighbors reads all neighbours of node ord on level from graph.
func collectNeighbors(t *testing.T, g utilhnsw.HnswGraph, level, ord int) []int {
	t.Helper()
	if err := g.SeekLevel(level, ord); err != nil {
		t.Fatalf("SeekLevel(%d,%d): %v", level, ord, err)
	}
	var out []int
	for {
		n, err := g.NextNeighbor()
		if err != nil {
			t.Fatalf("NextNeighbor: %v", err)
		}
		if n == util.NO_MORE_DOCS {
			break
		}
		out = append(out, n)
	}
	return out
}

// drainNodesIterator collects all ordinals from an iterator using HasNext/NextInt.
func drainNodesIterator(it utilhnsw.NodesIterator) []int {
	var out []int
	for it.HasNext() {
		out = append(out, it.NextInt())
	}
	return out
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestLucene99HnswVectorsReader_EmptySegment verifies that the reader
// opens and passes CheckIntegrity for a segment with no vector fields.
func TestLucene99HnswVectorsReader_EmptySegment(t *testing.T) {
	fis := hnswRTFieldInfos(t)
	ws, rs, cleanup := hnswRTSegmentState(t, 0, fis)
	defer cleanup()

	w, err := NewLucene99HnswVectorsWriter(ws, 16, 100, 100, 1)
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := NewLucene99HnswVectorsReader(rs)
	if err != nil {
		t.Fatalf("reader: %v", err)
	}
	defer func() { _ = r.Close() }()

	if err := r.CheckIntegrity(); err != nil {
		t.Errorf("CheckIntegrity: %v", err)
	}
}

// TestLucene99HnswVectorsReader_SingleFieldRoundTrip writes one float
// field with 8 vectors and verifies that the reader reports the correct
// graph structure: right Size, at least one level, and that every level-0
// node can be seeked without error.
func TestLucene99HnswVectorsReader_SingleFieldRoundTrip(t *testing.T) {
	const (
		fieldName = "vec"
		dim       = 4
		numDocs   = 8
	)
	fi := hnswRTFloatField(fieldName, 0, dim, index.VectorSimilarityFunctionEuclidean)
	fis := hnswRTFieldInfos(t, fi)
	ws, rs, cleanup := hnswRTSegmentState(t, numDocs, fis)
	defer cleanup()

	// --- write ---
	w, err := NewLucene99HnswVectorsWriter(ws, 4 /*M*/, 20 /*beamWidth*/, 100, 1)
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	fw, err := w.AddField(fi)
	if err != nil {
		t.Fatalf("AddField: %v", err)
	}
	for i := 0; i < numDocs; i++ {
		vec := make([]float32, dim)
		for d := range vec {
			vec[d] = float32(i*dim + d)
		}
		if err := fw.AddValueFloat32(i, vec); err != nil {
			t.Fatalf("AddValueFloat32 doc %d: %v", i, err)
		}
	}
	if err := w.Flush(numDocs); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// --- read ---
	r, err := NewLucene99HnswVectorsReader(rs)
	if err != nil {
		t.Fatalf("reader: %v", err)
	}
	defer func() { _ = r.Close() }()

	if err := r.CheckIntegrity(); err != nil {
		t.Errorf("CheckIntegrity: %v", err)
	}

	g, err := r.GetGraph(fieldName)
	if err != nil {
		t.Fatalf("GetGraph: %v", err)
	}

	if got := g.Size(); got != numDocs {
		t.Errorf("Size = %d, want %d", got, numDocs)
	}

	numLevels, err := g.NumLevels()
	if err != nil {
		t.Fatalf("NumLevels: %v", err)
	}
	if numLevels < 1 {
		t.Fatalf("NumLevels = %d, want >= 1", numLevels)
	}

	// Verify every level-0 node can be seeked.
	for ord := 0; ord < numDocs; ord++ {
		if err := g.SeekLevel(0, ord); err != nil {
			t.Errorf("SeekLevel(0,%d): %v", ord, err)
		}
	}

	// Verify all level-0 nodes appear in GetNodesOnLevel.
	it, err := g.GetNodesOnLevel(0)
	if err != nil {
		t.Fatalf("GetNodesOnLevel(0): %v", err)
	}
	levelNodes := drainNodesIterator(it)
	if len(levelNodes) != numDocs {
		t.Errorf("level-0 nodes count = %d, want %d", len(levelNodes), numDocs)
	}
}

// TestLucene99HnswVectorsReader_MultipleFields verifies that the reader
// correctly demultiplexes two independent vector fields written to the
// same segment.
func TestLucene99HnswVectorsReader_MultipleFields(t *testing.T) {
	const (
		dim1    = 3
		dim2    = 6
		numDocs = 5
	)
	fi1 := hnswRTFloatField("f1", 0, dim1, index.VectorSimilarityFunctionEuclidean)
	fi2 := hnswRTFloatField("f2", 1, dim2, index.VectorSimilarityFunctionCosine)
	fis := hnswRTFieldInfos(t, fi1, fi2)
	ws, rs, cleanup := hnswRTSegmentState(t, numDocs, fis)
	defer cleanup()

	w, err := NewLucene99HnswVectorsWriter(ws, 4, 20, 100, 1)
	if err != nil {
		t.Fatalf("writer: %v", err)
	}

	for i, fi := range []*index.FieldInfo{fi1, fi2} {
		fw, err := w.AddField(fi)
		if err != nil {
			t.Fatalf("AddField %d: %v", i, err)
		}
		d := fi.VectorDimension()
		for doc := 0; doc < numDocs; doc++ {
			vec := make([]float32, d)
			for k := range vec {
				vec[k] = float32((i+1)*100 + doc*d + k)
			}
			if err := fw.AddValueFloat32(doc, vec); err != nil {
				t.Fatalf("AddValueFloat32 field=%d doc=%d: %v", i, doc, err)
			}
		}
	}
	if err := w.Flush(numDocs); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := NewLucene99HnswVectorsReader(rs)
	if err != nil {
		t.Fatalf("reader: %v", err)
	}
	defer func() { _ = r.Close() }()

	for _, name := range []string{"f1", "f2"} {
		g, err := r.GetGraph(name)
		if err != nil {
			t.Errorf("GetGraph(%q): %v", name, err)
			continue
		}
		if got := g.Size(); got != numDocs {
			t.Errorf("GetGraph(%q).Size = %d, want %d", name, got, numDocs)
		}
	}
}

// TestLucene99HnswVectorsReader_NeighbourTraversal confirms that seeked
// neighbour lists contain only valid node ordinals (0 <= n < size).
func TestLucene99HnswVectorsReader_NeighbourTraversal(t *testing.T) {
	const (
		fieldName = "nbr"
		dim       = 2
		numDocs   = 12
	)
	fi := hnswRTFloatField(fieldName, 0, dim, index.VectorSimilarityFunctionDotProduct)
	fis := hnswRTFieldInfos(t, fi)
	ws, rs, cleanup := hnswRTSegmentState(t, numDocs, fis)
	defer cleanup()

	w, err := NewLucene99HnswVectorsWriter(ws, 4, 20, 100, 1)
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	fw, err := w.AddField(fi)
	if err != nil {
		t.Fatalf("AddField: %v", err)
	}
	for i := 0; i < numDocs; i++ {
		if err := fw.AddValueFloat32(i, []float32{float32(i), float32(numDocs - i)}); err != nil {
			t.Fatalf("AddValueFloat32 doc %d: %v", i, err)
		}
	}
	if err := w.Flush(numDocs); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	_ = w.Close()

	r, err := NewLucene99HnswVectorsReader(rs)
	if err != nil {
		t.Fatalf("reader: %v", err)
	}
	defer func() { _ = r.Close() }()

	g, err := r.GetGraph(fieldName)
	if err != nil {
		t.Fatalf("GetGraph: %v", err)
	}

	size := g.Size()
	for ord := 0; ord < size; ord++ {
		nbrs := collectNeighbors(t, g, 0, ord)
		for _, n := range nbrs {
			if n < 0 || n >= size {
				t.Errorf("node %d: neighbour %d out of range [0,%d)", ord, n, size)
			}
		}
	}
	t.Logf("traversed %d nodes at level-0", size)
}

// TestLucene99HnswVectorsReader_FieldNotFound verifies GetGraph returns
// an error for a field name not present in the segment.
func TestLucene99HnswVectorsReader_FieldNotFound(t *testing.T) {
	fis := hnswRTFieldInfos(t)
	ws, rs, cleanup := hnswRTSegmentState(t, 0, fis)
	defer cleanup()

	w, err := NewLucene99HnswVectorsWriter(ws, 16, 100, 100, 1)
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	_ = w.Finish()
	_ = w.Close()

	r, err := NewLucene99HnswVectorsReader(rs)
	if err != nil {
		t.Fatalf("reader: %v", err)
	}
	defer func() { _ = r.Close() }()

	if _, err := r.GetGraph("nonexistent"); err == nil {
		t.Error("GetGraph(nonexistent): want error, got nil")
	}
}

// TestLucene99HnswVectorsReader_UnimplementedMethods confirms that methods
// depending on the absent FlatVectorsReader return an error rather than
// panicking.
func TestLucene99HnswVectorsReader_UnimplementedMethods(t *testing.T) {
	fis := hnswRTFieldInfos(t)
	ws, rs, cleanup := hnswRTSegmentState(t, 0, fis)
	defer cleanup()

	w, err := NewLucene99HnswVectorsWriter(ws, 16, 100, 100, 1)
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	_ = w.Finish()
	_ = w.Close()

	r, err := NewLucene99HnswVectorsReader(rs)
	if err != nil {
		t.Fatalf("reader: %v", err)
	}
	defer func() { _ = r.Close() }()

	if _, err := r.GetFloatVectorValues("any"); err == nil {
		t.Error("GetFloatVectorValues: want error, got nil")
	}
	if _, err := r.GetByteVectorValues("any"); err == nil {
		t.Error("GetByteVectorValues: want error, got nil")
	}
	// Search methods with nil args should return error, not panic.
	if err := r.SearchFloat("any", nil, nil, nil); err == nil {
		t.Error("SearchFloat: want error, got nil")
	}
	if err := r.SearchByte("any", nil, nil, nil); err == nil {
		t.Error("SearchByte: want error, got nil")
	}
}

// TestLucene99HnswVectorsReader_GraphProperties verifies scalar properties
// of the returned HnswGraph are consistent with the written configuration.
func TestLucene99HnswVectorsReader_GraphProperties(t *testing.T) {
	cases := []struct {
		name    string
		dim     int
		numDocs int
		M       int
		sim     index.VectorSimilarityFunction
	}{
		{"euclidean-small", 4, 5, 4, index.VectorSimilarityFunctionEuclidean},
		{"cosine-medium", 8, 16, 8, index.VectorSimilarityFunctionCosine},
		{"dot-product", 3, 10, 4, index.VectorSimilarityFunctionDotProduct},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			fi := hnswRTFloatField("v", 0, c.dim, c.sim)
			fis := hnswRTFieldInfos(t, fi)
			ws, rs, cleanup := hnswRTSegmentState(t, c.numDocs, fis)
			defer cleanup()

			w, err := NewLucene99HnswVectorsWriter(ws, c.M, 20, 100, 1)
			if err != nil {
				t.Fatalf("writer: %v", err)
			}
			fw, err := w.AddField(fi)
			if err != nil {
				t.Fatalf("AddField: %v", err)
			}
			for i := 0; i < c.numDocs; i++ {
				vec := make([]float32, c.dim)
				for d := range vec {
					vec[d] = float32(i + d)
				}
				if err := fw.AddValueFloat32(i, vec); err != nil {
					t.Fatalf("doc %d: %v", i, err)
				}
			}
			if err := w.Flush(c.numDocs); err != nil {
				t.Fatalf("Flush: %v", err)
			}
			if err := w.Finish(); err != nil {
				t.Fatalf("Finish: %v", err)
			}
			_ = w.Close()

			r, err := NewLucene99HnswVectorsReader(rs)
			if err != nil {
				t.Fatalf("reader: %v", err)
			}
			defer func() { _ = r.Close() }()

			g, err := r.GetGraph("v")
			if err != nil {
				t.Fatalf("GetGraph: %v", err)
			}
			if got := g.Size(); got != c.numDocs {
				t.Errorf("Size = %d, want %d", got, c.numDocs)
			}
			nl, err := g.NumLevels()
			if err != nil {
				t.Fatalf("NumLevels: %v", err)
			}
			if nl < 1 {
				t.Errorf("NumLevels = %d, want >= 1", nl)
			}
			en, err := g.EntryNode()
			if err != nil {
				t.Fatalf("EntryNode: %v", err)
			}
			if en < 0 || en >= c.numDocs {
				t.Errorf("EntryNode = %d, out of range [0,%d)", en, c.numDocs)
			}
			mc := g.MaxConn()
			if mc < 1 {
				t.Errorf("MaxConn = %d, want >= 1", mc)
			}
			t.Logf("%s: size=%d levels=%d entry=%d maxConn=%d",
				c.name, g.Size(), nl, en, mc)
		})
	}
}

// TestLucene99HnswVectorsReader_LevelConsistency verifies that every node
// appearing in level L > 0 also appears in level 0, and that
// GetNodesOnLevel returns a sorted list at level 0.
func TestLucene99HnswVectorsReader_LevelConsistency(t *testing.T) {
	const (
		fieldName = "lc"
		dim       = 3
		numDocs   = 20
	)
	fi := hnswRTFloatField(fieldName, 0, dim, index.VectorSimilarityFunctionEuclidean)
	fis := hnswRTFieldInfos(t, fi)
	ws, rs, cleanup := hnswRTSegmentState(t, numDocs, fis)
	defer cleanup()

	w, err := NewLucene99HnswVectorsWriter(ws, 4, 20, 100, 1)
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	fw, err := w.AddField(fi)
	if err != nil {
		t.Fatalf("AddField: %v", err)
	}
	for i := 0; i < numDocs; i++ {
		vec := make([]float32, dim)
		for d := range vec {
			vec[d] = float32(i*3 + d)
		}
		if err := fw.AddValueFloat32(i, vec); err != nil {
			t.Fatalf("doc %d: %v", i, err)
		}
	}
	if err := w.Flush(numDocs); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	_ = w.Close()

	r, err := NewLucene99HnswVectorsReader(rs)
	if err != nil {
		t.Fatalf("reader: %v", err)
	}
	defer func() { _ = r.Close() }()

	g, err := r.GetGraph(fieldName)
	if err != nil {
		t.Fatalf("GetGraph: %v", err)
	}

	numLevels, err := g.NumLevels()
	if err != nil {
		t.Fatalf("NumLevels: %v", err)
	}

	// Collect all level-0 nodes into a set; verify they are sorted.
	it0, err := g.GetNodesOnLevel(0)
	if err != nil {
		t.Fatalf("GetNodesOnLevel(0): %v", err)
	}
	level0 := make(map[int]struct{}, numDocs)
	prev := -1
	for it0.HasNext() {
		n := it0.NextInt()
		if n <= prev {
			t.Errorf("level-0 nodes not sorted: %d after %d", n, prev)
		}
		level0[n] = struct{}{}
		prev = n
	}

	// Every node on level L > 0 must appear in level 0.
	for L := 1; L < numLevels; L++ {
		itL, err := g.GetNodesOnLevel(L)
		if err != nil {
			t.Fatalf("GetNodesOnLevel(%d): %v", L, err)
		}
		for itL.HasNext() {
			n := itL.NextInt()
			if _, ok := level0[n]; !ok {
				t.Errorf("node %d on level %d not found in level-0", n, L)
			}
		}
	}

	t.Logf("consistency OK: %d levels, %d level-0 nodes", numLevels, len(level0))
}

// TestLucene99HnswVectorsReader_ByteVectors writes a field using BYTE
// encoding and verifies the reader can open and seek the graph.
func TestLucene99HnswVectorsReader_ByteVectors(t *testing.T) {
	const (
		fieldName = "bytevec"
		dim       = 4
		numDocs   = 6
	)
	fi := index.NewFieldInfo(fieldName, 0, index.FieldInfoOptions{
		VectorDimension:          dim,
		VectorEncoding:           index.VectorEncodingByte,
		VectorSimilarityFunction: index.VectorSimilarityFunctionEuclidean,
	})
	fis := hnswRTFieldInfos(t, fi)
	ws, rs, cleanup := hnswRTSegmentState(t, numDocs, fis)
	defer cleanup()

	w, err := NewLucene99HnswVectorsWriter(ws, 4, 20, 100, 1)
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	fw, err := w.AddField(fi)
	if err != nil {
		t.Fatalf("AddField: %v", err)
	}
	for i := 0; i < numDocs; i++ {
		vec := make([]byte, dim)
		for d := range vec {
			vec[d] = byte(i*dim + d)
		}
		if err := fw.AddValueByte(i, vec); err != nil {
			t.Fatalf("AddValueByte doc %d: %v", i, err)
		}
	}
	if err := w.Flush(numDocs); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	_ = w.Close()

	r, err := NewLucene99HnswVectorsReader(rs)
	if err != nil {
		t.Fatalf("reader: %v", err)
	}
	defer func() { _ = r.Close() }()

	if err := r.CheckIntegrity(); err != nil {
		t.Errorf("CheckIntegrity: %v", err)
	}

	g, err := r.GetGraph(fieldName)
	if err != nil {
		t.Fatalf("GetGraph: %v", err)
	}
	if got := g.Size(); got != numDocs {
		t.Errorf("Size = %d, want %d", got, numDocs)
	}
	for ord := 0; ord < numDocs; ord++ {
		if err := g.SeekLevel(0, ord); err != nil {
			t.Errorf("SeekLevel(0,%d): %v", ord, err)
		}
	}
}
