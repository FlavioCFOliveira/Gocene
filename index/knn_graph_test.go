// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"fmt"
	"math"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Port of org.apache.lucene.index.TestKnnGraph (Lucene 10.4.0).
//
// The reference test exercises the end-to-end IndexWriter -> HNSW knn-vectors
// codec -> DirectoryReader pipeline. rmp #4756 wired that pipeline through
// IndexWriter.AddDocument / Commit and the Lucene99HnswVectorsFormat codec
// (#4731), so these tests now drive the real public API:
//
//   - document.KnnFloatVectorField for indexing,
//   - index.IndexWriter.AddDocument / Commit,
//   - index.OpenDirectoryReader to reopen the committed index,
//   - search.KnnFloatVectorQuery + search.IndexSearcher for search.
//
// Deviation from the Java reference: TestKnnGraph asserts the internal HNSW
// graph topology (Lucene99HnswVectorsReader.getGraph -> numLevels /
// getNodesOnLevel / nextNeighbor) via PerFieldKnnVectorsFormat.FieldsReader.
// Gocene does not expose that codec-internal graph accessor from the index
// package, so the graph-consistency tests (testBasic / testSingleDocument /
// testMerge / testMultipleVectorFields) instead assert the equivalent
// observable contract that the reference's assertConsistentGraph ultimately
// guarantees: every indexed vector reads back byte-for-byte from the leaf's
// FloatVectorValues, the number of docs with vectors matches, and each
// indexed vector is the exact nearest neighbour of itself under a
// KnnFloatVectorQuery. testSearch / testMultiThreadedSearch are faithful
// ports (the reference itself only asserts search results there).

// knnGraphField mirrors the KNN_GRAPH_FIELD constant.
const knnGraphField = "vector"

// gridValues ports the value-generation half of indexData(): it lays out a
// document for every cartesian point in an n x n square, inserting by striding
// with a prime step that is not a divisor of n*n so each point is hit exactly
// once in a deterministic but distributed order.
//
// The returned slice is indexed by insertion id; entry i is the {x, y} vector
// inserted as the i-th document.
func gridValues(n, stepSize int) [][]float32 {
	values := make([][]float32, n*n)
	index := 0
	for i := range values {
		x, y := index%n, index/n
		values[i] = []float32{float32(x), float32(y)}
		index = (index + stepSize) % (n * n)
	}
	return values
}

// assertMaxConn ports the static assertMaxConn helper: every populated
// adjacency row must respect maxConn, and every neighbor it references must
// itself be a populated node.
func assertMaxConn(t *testing.T, graph [][]int, maxConn int) {
	t.Helper()
	for _, neighbors := range graph {
		if neighbors == nil {
			continue
		}
		if len(neighbors) > maxConn {
			t.Fatalf("node fan-out %d exceeds maxConn %d", len(neighbors), maxConn)
		}
		for _, k := range neighbors {
			if graph[k] == nil {
				t.Fatalf("neighbor %d is not a populated node", k)
			}
		}
	}
}

// assertConnected ports the assertConnected helper: starting a BFS from the
// given start node, every populated node must be reachable. The reference
// picks the start node at random; here it is a parameter so the check is
// deterministic.
func assertConnected(t *testing.T, graph [][]int, startNode int) {
	t.Helper()
	nodes := make([]int, 0, len(graph))
	for i := range graph {
		if graph[i] != nil {
			nodes = append(nodes, i)
		}
	}
	visited := make(map[int]struct{}, len(nodes))
	queue := []int{startNode}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		if graph[i] == nil {
			t.Fatalf("expected neighbors of %d", i)
		}
		visited[i] = struct{}{}
		for _, j := range graph[i] {
			if _, seen := visited[j]; !seen {
				queue = append(queue, j)
			}
		}
	}
	for _, node := range nodes {
		if _, ok := visited[node]; !ok {
			t.Fatalf("attempted to walk entire graph but never visited node [%d]", node)
		}
	}
}

// reachesAll is a non-fatal BFS used only by the fixture test to confirm the
// assertConnected helper would reject a disconnected graph.
func reachesAll(graph [][]int, start int) bool {
	visited := make(map[int]struct{})
	queue := []int{start}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		if graph[i] == nil {
			return false
		}
		visited[i] = struct{}{}
		for _, j := range graph[i] {
			if _, seen := visited[j]; !seen {
				queue = append(queue, j)
			}
		}
	}
	for i := range graph {
		if graph[i] == nil {
			continue
		}
		if _, ok := visited[i]; !ok {
			return false
		}
	}
	return true
}

// l2normalizeKnnGraph is the dimension-independent normalization the reference
// applies via VectorUtil.l2normalize when building random vectors.
func l2normalizeKnnGraph(v []float32) {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return
	}
	norm := float32(1.0 / math.Sqrt(sum))
	for i := range v {
		v[i] *= norm
	}
}

// newKnnWriter builds an IndexWriter over a fresh on-disk directory. The
// production Lucene104 codec (registered via the codecs blank import in
// setup_codec_test.go) supplies the PerFieldKnnVectorsFormat over
// Lucene99HnswVectorsFormat used to build and read the HNSW graph.
func newKnnWriter(t *testing.T) (store.Directory, *index.IndexWriter) {
	t.Helper()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	cfg := index.NewIndexWriterConfig(nil)
	iw, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return dir, iw
}

// addKnnDoc adds one document carrying the EUCLIDEAN knn vector field
// (when vector != nil) plus a stored "id" string, mirroring TestKnnGraph.add.
func addKnnDoc(t *testing.T, iw *index.IndexWriter, field string, id int, vector []float32) {
	t.Helper()
	doc := document.NewDocument()
	if vector != nil {
		f, err := document.NewKnnFloatVectorField(field, vector, index.VectorSimilarityFunctionEuclidean)
		if err != nil {
			t.Fatalf("NewKnnFloatVectorField(id=%d): %v", id, err)
		}
		doc.Add(f)
	}
	idField, err := document.NewStringField("id", fmt.Sprintf("%d", id), true)
	if err != nil {
		t.Fatalf("NewStringField(id=%d): %v", id, err)
	}
	doc.Add(idField)
	if err := iw.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument(id=%d): %v", id, err)
	}
}

// docIDToInsertionID reads the stored "id" of a global docID through the
// searcher, mapping a result docID back to its insertion id (as the reference
// does via storedFields.document(doc).get("id")).
func docIDToInsertionID(t *testing.T, searcher *search.IndexSearcher, docID int) int {
	t.Helper()
	doc, err := searcher.Doc(docID)
	if err != nil {
		t.Fatalf("Doc(%d): %v", docID, err)
	}
	field := doc.Get("id")
	if field == nil {
		t.Fatalf("doc %d has no stored id field", docID)
	}
	var id int
	if _, err := fmt.Sscanf(field.StringValue(), "%d", &id); err != nil {
		t.Fatalf("parsing id %q: %v", field.StringValue(), err)
	}
	return id
}

// assertVectorsReadBack verifies the observable half of the reference's
// assertConsistentGraph: every leaf vector reads back exactly equal to the
// originally indexed value (keyed by the stored insertion id), and the total
// number of docs carrying a vector equals the number of non-nil entries in
// values. This is the Gocene-observable equivalent of the reference's
// "stored vector values are the same as original" and "expectedNumDocsWithVectors"
// assertions, without reaching into the codec-internal HNSW graph.
func assertVectorsReadBack(t *testing.T, dir store.Directory, field string, values [][]float32) {
	t.Helper()
	dr, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer dr.Close()

	searcher := search.NewIndexSearcher(dr)

	numDocsWithVectors := 0
	docBase := 0
	for _, sr := range dr.GetSegmentReaders() {
		fvv, err := sr.GetFloatVectorValues(field)
		if err != nil {
			t.Fatalf("GetFloatVectorValues: %v", err)
		}
		if fvv == nil {
			docBase += sr.MaxDoc()
			continue
		}
		for i := 0; i < sr.MaxDoc(); i++ {
			vec, err := fvv.Get(i)
			if err != nil {
				t.Fatalf("FloatVectorValues.Get(%d): %v", i, err)
			}
			id := docIDToInsertionID(t, searcher, docBase+i)
			if vec == nil {
				if id < len(values) && values[id] != nil {
					t.Fatalf("doc %d (id=%d) expected a vector but leaf returned none", docBase+i, id)
				}
				continue
			}
			if id >= len(values) || values[id] == nil {
				t.Fatalf("doc %d (id=%d) has a vector but none was indexed", docBase+i, id)
			}
			if !float32SlicesEqual(vec, values[id]) {
				t.Fatalf("vector mismatch for doc %d id=%d: got %v want %v", docBase+i, id, vec, values[id])
			}
			numDocsWithVectors++
		}
		docBase += sr.MaxDoc()
	}

	expected := 0
	for _, v := range values {
		if v != nil {
			expected++
		}
	}
	if numDocsWithVectors != expected {
		t.Fatalf("docs-with-vectors = %d, want %d", numDocsWithVectors, expected)
	}
}

// assertEachVectorIsItsOwnNearest verifies that, for every indexed vector,
// a KnnFloatVectorQuery for that exact vector returns a document whose stored
// vector equals the query (an exact, score-maximal match). This is the
// observable consequence of a correctly built, fully connected HNSW graph
// (the reference's connectivity invariant): a node must be reachable, hence
// findable, from the search entry point.
//
// The match is checked by vector equality rather than by insertion id because
// the deterministic test generators can produce duplicate vectors; when two
// documents share a vector, either is a correct nearest neighbour of that
// vector.
func assertEachVectorIsItsOwnNearest(t *testing.T, dir store.Directory, field string, values [][]float32) {
	t.Helper()
	dr, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer dr.Close()
	searcher := search.NewIndexSearcher(dr)

	for id, v := range values {
		if v == nil {
			continue
		}
		q := search.NewKnnFloatVectorQuery(field, v, 1)
		top, err := searcher.Search(q, 1)
		if err != nil {
			t.Fatalf("Search(id=%d): %v", id, err)
		}
		if len(top.ScoreDocs) == 0 {
			t.Fatalf("query for indexed vector id=%d returned no hits", id)
		}
		gotID := docIDToInsertionID(t, searcher, top.ScoreDocs[0].Doc)
		if gotID >= len(values) || !float32SlicesEqual(values[gotID], v) {
			t.Fatalf("nearest of vector id=%d (%v) was doc id=%d (%v); vectors differ (vector not findable)",
				id, v, gotID, vectorOrNil(values, gotID))
		}
	}
}

// vectorOrNil safely indexes values for diagnostic messages.
func vectorOrNil(values [][]float32, id int) []float32 {
	if id >= 0 && id < len(values) {
		return values[id]
	}
	return nil
}

func float32SlicesEqual(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestKnnGraph_Fixture exercises the deterministic data builders and graph
// invariant helpers so the ported logic is exercised independently of the
// codec pipeline.
func TestKnnGraph_Fixture(t *testing.T) {
	// gridValues must hit every cartesian point exactly once. With n=5 and
	// stepSize=17 (gcd(17,25)=1) the 25 generated vectors are a permutation of
	// the 5x5 grid.
	values := gridValues(5, 17)
	if len(values) != 25 {
		t.Fatalf("gridValues length = %d, want 25", len(values))
	}
	seen := make(map[[2]float32]struct{}, 25)
	for i, v := range values {
		if len(v) != 2 {
			t.Fatalf("vector %d has dimension %d, want 2", i, len(v))
		}
		key := [2]float32{v[0], v[1]}
		if _, dup := seen[key]; dup {
			t.Fatalf("grid point (%v,%v) generated twice", v[0], v[1])
		}
		seen[key] = struct{}{}
	}
	if values[0][0] != 0 || values[0][1] != 0 {
		t.Fatalf("values[0] = %v, want origin {0,0}", values[0])
	}
	if values[1][0] != 2 || values[1][1] != 3 {
		t.Fatalf("values[1] = %v, want {2,3}", values[1])
	}

	ring := [][]int{{1, 2}, {0, 2}, {0, 1}}
	assertConnected(t, ring, 0)
	assertMaxConn(t, ring, 2)

	t.Run("disconnected_is_detected", func(t *testing.T) {
		broken := [][]int{{1}, {0}, {3}, {2}}
		if reachesAll(broken, 0) {
			t.Fatalf("expected {0,1} component to exclude nodes {2,3}")
		}
	})

	// l2normalize must produce a unit vector.
	u := []float32{3, 4}
	l2normalizeKnnGraph(u)
	if d := math.Abs(float64(u[0]*u[0]+u[1]*u[1]) - 1.0); d > 1e-6 {
		t.Fatalf("l2normalize produced non-unit vector: %v", u)
	}
}

// TestKnnGraph_Basic ports testBasic: index several vectors of dimension >= 3,
// then assert every leaf vector reads back equal to the original and every
// vector is findable as its own nearest neighbour.
//
// Dense-only deviation: the Java reference gives only some documents a vector
// (random().nextBoolean()), exercising the sparse on-disk layout. Gocene's
// flat-vectors writer does not yet support the sparse case (it requires the
// IndexedDISI port tracked by rmp #4755), so this port indexes a vector on
// every document. The observable graph contract being verified is identical.
func TestKnnGraph_Basic(t *testing.T) {
	dir, iw := newKnnWriter(t)
	defer dir.Close()

	const numDoc = 12
	const dimension = 4
	values := make([][]float32, numDoc)
	for i := 0; i < numDoc; i++ {
		v := make([]float32, dimension)
		for j := 0; j < dimension; j++ {
			v[j] = float32((i*7+j*13)%17) + 1
		}
		l2normalizeKnnGraph(v)
		values[i] = v
		addKnnDoc(t, iw, knnGraphField, i, values[i])
	}
	if err := iw.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := iw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	assertVectorsReadBack(t, dir, knnGraphField, values)
	assertEachVectorIsItsOwnNearest(t, dir, knnGraphField, values)
}

// TestKnnGraph_SingleDocument ports testSingleDocument: a single 3-d vector,
// asserted consistent both before and after a second commit.
func TestKnnGraph_SingleDocument(t *testing.T) {
	dir, iw := newKnnWriter(t)
	defer dir.Close()

	values := [][]float32{{0, 1, 2}}
	addKnnDoc(t, iw, knnGraphField, 0, values[0])
	if err := iw.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	assertVectorsReadBack(t, dir, knnGraphField, values)

	// A second commit (no new docs) must preserve the single-vector graph.
	if err := iw.Commit(); err != nil {
		t.Fatalf("second Commit: %v", err)
	}
	if err := iw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	assertVectorsReadBack(t, dir, knnGraphField, values)
	assertEachVectorIsItsOwnNearest(t, dir, knnGraphField, values)
}

// TestKnnGraph_Merge ports testMerge: index many vectors across several
// intermediate commits (producing multiple segments) and verify the vectors
// survive and remain findable. Gocene's IndexWriter does not yet perform
// segment merges on commit, so this exercises the multi-segment read path
// rather than a literal forceMerge(1); the observable contract (every vector
// read back, every vector findable) is identical.
func TestKnnGraph_Merge(t *testing.T) {
	dir, iw := newKnnWriter(t)
	defer dir.Close()

	const numDoc = 60
	const dimension = 10
	values := make([][]float32, numDoc)
	for i := 0; i < numDoc; i++ {
		// Dense-only (see TestKnnGraph_Basic): every document carries a vector.
		v := make([]float32, dimension)
		for j := 0; j < dimension; j++ {
			v[j] = float32((i*3+j*5)%19) + 1
		}
		l2normalizeKnnGraph(v)
		values[i] = v
		addKnnDoc(t, iw, knnGraphField, i, values[i])
		if i%17 == 3 {
			// Force several segment boundaries.
			if err := iw.Commit(); err != nil {
				t.Fatalf("intermediate Commit at i=%d: %v", i, err)
			}
		}
	}
	if err := iw.Commit(); err != nil {
		t.Fatalf("final Commit: %v", err)
	}
	if err := iw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	assertVectorsReadBack(t, dir, knnGraphField, values)
	assertEachVectorIsItsOwnNearest(t, dir, knnGraphField, values)
}

// TestKnnGraph_MultipleVectorFields ports testMultipleVectorFields: write
// several distinct knn vector fields with independent dimensions and assert
// each field's vectors read back and are findable.
func TestKnnGraph_MultipleVectorFields(t *testing.T) {
	dir, iw := newKnnWriter(t)
	defer dir.Close()

	const numVectorFields = 3
	const numDoc = 30
	dims := []int{3, 4, 5}
	values := make([][][]float32, numVectorFields)
	for f := 0; f < numVectorFields; f++ {
		values[f] = make([][]float32, numDoc)
		for d := 0; d < numDoc; d++ {
			// Dense-only (see TestKnnGraph_Basic): every document carries a
			// vector for every field.
			v := make([]float32, dims[f])
			for j := 0; j < dims[f]; j++ {
				v[j] = float32((d*7+f*11+j*13)%23) + 1
			}
			l2normalizeKnnGraph(v)
			values[f][d] = v
		}
	}

	for d := 0; d < numDoc; d++ {
		doc := document.NewDocument()
		for f := 0; f < numVectorFields; f++ {
			field := fmt.Sprintf("%s%d", knnGraphField, f)
			vf, err := document.NewKnnFloatVectorField(field, values[f][d], index.VectorSimilarityFunctionEuclidean)
			if err != nil {
				t.Fatalf("NewKnnFloatVectorField(field=%d doc=%d): %v", f, d, err)
			}
			doc.Add(vf)
		}
		idField, err := document.NewStringField("id", fmt.Sprintf("%d", d), true)
		if err != nil {
			t.Fatalf("NewStringField: %v", err)
		}
		doc.Add(idField)
		if err := iw.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(doc=%d): %v", d, err)
		}
	}
	if err := iw.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := iw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for f := 0; f < numVectorFields; f++ {
		field := fmt.Sprintf("%s%d", knnGraphField, f)
		assertVectorsReadBack(t, dir, field, values[f])
		assertEachVectorIsItsOwnNearest(t, dir, field, values[f])
	}
}

// indexGridData ports indexData: index the 5x5 EUCLIDEAN grid, committing after
// document 13 to force two segments, mirroring the reference column-major
// insertion order.
func indexGridData(t *testing.T, iw *index.IndexWriter) {
	t.Helper()
	const n, stepSize = 5, 17
	values := gridValues(n, stepSize)
	for i, v := range values {
		addKnnDoc(t, iw, knnGraphField, i, v)
		if i == 13 {
			// create 2 segments
			if err := iw.Commit(); err != nil {
				t.Fatalf("Commit after doc 13: %v", err)
			}
		}
	}
	if err := iw.Commit(); err != nil {
		t.Fatalf("final Commit: %v", err)
	}
}

// assertGraphSearch ports assertGraphSearch: run a k=5 KnnFloatVectorQuery,
// map result docIDs to insertion ids, and assert the exact id order and
// scores.
func assertGraphSearch(t *testing.T, searcher *search.IndexSearcher, expected []int, expectedScores []float32, vector []float32) {
	t.Helper()
	q := search.NewKnnFloatVectorQuery(knnGraphField, vector, 5)
	top, err := searcher.Search(q, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(top.ScoreDocs) != len(expected) {
		t.Fatalf("result count = %d, want %d (results=%+v)", len(top.ScoreDocs), len(expected), top.ScoreDocs)
	}
	for i := range expected {
		gotID := docIDToInsertionID(t, searcher, top.ScoreDocs[i].Doc)
		if gotID != expected[i] {
			t.Fatalf("doc mismatch at idx %d: got id=%d want id=%d (results=%+v)", i, gotID, expected[i], top.ScoreDocs)
		}
		if d := math.Abs(float64(top.ScoreDocs[i].Score - expectedScores[i])); d > 0.01 {
			t.Fatalf("score mismatch at idx %d: got %f want %f", i, top.ScoreDocs[i].Score, expectedScores[i])
		}
	}
}

// TestKnnGraph_Search ports testSearch: index the 5x5 EUCLIDEAN grid across
// two segments and assert the exact knn result ids and scores for two query
// points.
func TestKnnGraph_Search(t *testing.T) {
	dir, iw := newKnnWriter(t)
	defer dir.Close()

	indexGridData(t, iw)
	if err := iw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dr, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer dr.Close()
	searcher := search.NewIndexSearcher(dr)

	// Insertion order (column major, origin upper left):
	//  0 15  5 20 10
	//  3 18  8 23 13
	//  6 21 11  1 16
	//  9 24 14  4 19
	// 12  2 17  7 22
	assertGraphSearch(t, searcher,
		[]int{0, 15, 3, 18, 5},
		[]float32{0.99, 0.55, 0.50, 0.36, 0.22},
		[]float32{0, 0.1})
	assertGraphSearch(t, searcher,
		[]int{15, 18, 0, 3, 5},
		[]float32{0.88, 0.65, 0.58, 0.47, 0.40},
		[]float32{0.3, 0.8})
}

// TestKnnGraph_MultiThreadedSearch ports testMultiThreadedSearch: run the
// testSearch query concurrently from several goroutines against a shared
// SearcherManager.
func TestKnnGraph_MultiThreadedSearch(t *testing.T) {
	dir, iw := newKnnWriter(t)
	defer dir.Close()

	indexGridData(t, iw)
	if err := iw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dr, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer dr.Close()

	initial := search.NewIndexSearcher(dr)
	manager, err := search.NewSearcherManager(initial, search.NewDefaultSearcherFactory(), nil)
	if err != nil {
		t.Fatalf("NewSearcherManager: %v", err)
	}

	const numThreads = 4
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, numThreads)
	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			searcher, err := manager.Acquire()
			if err != nil {
				errs <- fmt.Errorf("Acquire: %w", err)
				return
			}
			defer func() { _ = manager.Release(searcher) }()
			if err := runGraphSearch(searcher,
				[]int{0, 15, 3, 18, 5},
				[]float32{0.99, 0.55, 0.50, 0.36, 0.22},
				[]float32{0, 0.1}); err != nil {
				errs <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// runGraphSearch is the error-returning variant of assertGraphSearch used by
// the concurrent test (testing.T must not be used from a goroutine for fatal
// assertions).
func runGraphSearch(searcher *search.IndexSearcher, expected []int, expectedScores []float32, vector []float32) error {
	q := search.NewKnnFloatVectorQuery(knnGraphField, vector, 5)
	top, err := searcher.Search(q, 5)
	if err != nil {
		return err
	}
	if len(top.ScoreDocs) != len(expected) {
		return fmt.Errorf("result count = %d, want %d", len(top.ScoreDocs), len(expected))
	}
	for i := range expected {
		doc, err := searcher.Doc(top.ScoreDocs[i].Doc)
		if err != nil {
			return err
		}
		field := doc.Get("id")
		if field == nil {
			return fmt.Errorf("doc %d has no id", top.ScoreDocs[i].Doc)
		}
		var gotID int
		if _, err := fmt.Sscanf(field.StringValue(), "%d", &gotID); err != nil {
			return err
		}
		if gotID != expected[i] {
			return fmt.Errorf("doc mismatch at idx %d: got id=%d want id=%d", i, gotID, expected[i])
		}
		if d := math.Abs(float64(top.ScoreDocs[i].Score - expectedScores[i])); d > 0.01 {
			return fmt.Errorf("score mismatch at idx %d: got %f want %f", i, top.ScoreDocs[i].Score, expectedScores[i])
		}
	}
	return nil
}
