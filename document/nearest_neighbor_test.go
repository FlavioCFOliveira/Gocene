// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"container/heap"
	"math"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// --- heap ordering -----------------------------------------------------------

func TestNearestCellHeap_OrderedByDistanceAsc(t *testing.T) {
	t.Parallel()

	h := &nearestCellHeap{}
	heap.Init(h)
	for _, key := range []float64{3.0, 1.0, 4.0, 1.5, 2.0} {
		heap.Push(h, &nearestCell{distanceSortKey: key})
	}

	var got []float64
	for h.Len() > 0 {
		got = append(got, heap.Pop(h).(*nearestCell).distanceSortKey)
	}

	want := []float64{1.0, 1.5, 2.0, 3.0, 4.0}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %v want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("cell heap: got %v want %v", got, want)
		}
	}
}

func TestNearestHitHeap_WorstFirstWithDocIDTieBreak(t *testing.T) {
	t.Parallel()

	h := &nearestHitHeap{}
	heap.Init(h)

	// Mix of distances; two entries share distance 5.0 — the worst at
	// the head should be the one with the *higher* docID.
	hits := []NearestHit{
		{DocID: 1, DistanceSortKey: 2.0},
		{DocID: 7, DistanceSortKey: 5.0},
		{DocID: 3, DistanceSortKey: 5.0},
		{DocID: 9, DistanceSortKey: 1.0},
	}
	for i := range hits {
		heap.Push(h, &hits[i])
	}

	head := (*h)[0]
	if head.DistanceSortKey != 5.0 || head.DocID != 7 {
		t.Fatalf("head: got (docID=%d, key=%g) want (docID=7, key=5.0)",
			head.DocID, head.DistanceSortKey)
	}

	// Drain confirms worst-to-best order with the docID tie-break.
	var got []NearestHit
	for h.Len() > 0 {
		got = append(got, *heap.Pop(h).(*NearestHit))
	}
	want := []NearestHit{
		{DocID: 7, DistanceSortKey: 5.0},
		{DocID: 3, DistanceSortKey: 5.0},
		{DocID: 1, DistanceSortKey: 2.0},
		{DocID: 9, DistanceSortKey: 1.0},
	}
	if len(got) != len(want) {
		t.Fatalf("len: got %v want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("hit %d: got %v want %v", i, got[i], want[i])
		}
	}
}

// --- approxBestDistance ------------------------------------------------------

func TestApproxBestDistance_InsideCellReturnsZero(t *testing.T) {
	t.Parallel()

	d := approxBestDistanceLatLon(
		-10, 10, -20, 20, // bbox
		0, 0, // query inside
	)
	if d != 0 {
		t.Fatalf("inside cell: got %g want 0", d)
	}
}

func TestApproxBestDistance_OutsideUsesMinCornerKey(t *testing.T) {
	t.Parallel()

	// Bbox: lat [10,20], lon [10,20]. Query well to the SW.
	pointLat, pointLon := 0.0, 0.0
	got := approxBestDistanceLatLon(10, 20, 10, 20, pointLat, pointLon)

	// The minimum of the four corner haversin sort keys must equal the
	// SW corner (10,10) — closest to (0,0).
	want := util.HaversinSortKey(pointLat, pointLon, 10, 10)
	if got != want {
		t.Fatalf("outside cell SW: got %g want %g", got, want)
	}
}

// --- nearestVisitor.Compare --------------------------------------------------

// Packed lat/lon helper mirroring the 8-byte LatLonPoint wire form.
func packedLatLon(lat, lon float64) []byte {
	out := make([]byte, 8)
	util.IntToSortableBytes(geo.EncodeLatitude(lat), out, 0)
	util.IntToSortableBytes(geo.EncodeLongitude(lon), out, 4)
	return out
}

func TestNearestVisitor_Compare_OutsideAndCrosses(t *testing.T) {
	t.Parallel()

	v := newNearestVisitor(&nearestHitHeap{}, 1, 0, 0)
	// Tighten the bbox manually as if a hit had triggered an update.
	v.minLat = -1
	v.maxLat = 1
	v.minLon = -1
	v.maxLon = 1
	v.minLon2 = math.Inf(+1)

	tests := []struct {
		name           string
		minLat, maxLat float64
		minLon, maxLon float64
		want           PointTreeCellRelation
	}{
		{
			name:   "cell fully inside bbox crosses",
			minLat: -0.5, maxLat: 0.5, minLon: -0.5, maxLon: 0.5,
			want: PointTreeCellCrossesQuery,
		},
		{
			name:   "cell to the north is outside",
			minLat: 5, maxLat: 10, minLon: -0.5, maxLon: 0.5,
			want: PointTreeCellOutsideQuery,
		},
		{
			name:   "cell to the east is outside",
			minLat: -0.5, maxLat: 0.5, minLon: 5, maxLon: 10,
			want: PointTreeCellOutsideQuery,
		},
		{
			name:   "cell straddling bbox crosses",
			minLat: -2, maxLat: 0, minLon: -2, maxLon: 0,
			want: PointTreeCellCrossesQuery,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := v.Compare(
				packedLatLon(tc.minLat, tc.minLon),
				packedLatLon(tc.maxLat, tc.maxLon),
			)
			if got != tc.want {
				t.Fatalf("Compare: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestNearestVisitor_Compare_DatelineSplit(t *testing.T) {
	t.Parallel()

	// Simulate a post-bbox-update state where the cap straddles the
	// antimeridian: the visitor uses minLon..maxLon for the western
	// half (here -inf..-170) and minLon2..+inf for the eastern half
	// (here 170..+inf).
	v := newNearestVisitor(&nearestHitHeap{}, 1, 0, 180)
	v.minLat = -5
	v.maxLat = 5
	v.minLon = math.Inf(-1)
	v.maxLon = -170
	v.minLon2 = 170

	// A cell well inside the western fragment is "crosses".
	if got := v.Compare(
		packedLatLon(-1, -179),
		packedLatLon(1, -171),
	); got != PointTreeCellCrossesQuery {
		t.Fatalf("western fragment: got %v want CROSSES", got)
	}
	// A cell well inside the eastern fragment is "crosses".
	if got := v.Compare(
		packedLatLon(-1, 171),
		packedLatLon(1, 179),
	); got != PointTreeCellCrossesQuery {
		t.Fatalf("eastern fragment: got %v want CROSSES", got)
	}
	// A cell at lon [-160, 160] sits in the gap and is OUTSIDE: its
	// max-lon (160) is < minLon2 (170) AND its longitude span misses
	// the western range (minLon=-inf, maxLon=-170 → cellMinLon=-160
	// is > maxLon).
	if got := v.Compare(
		packedLatLon(-1, -160),
		packedLatLon(1, 160),
	); got != PointTreeCellOutsideQuery {
		t.Fatalf("middle gap: got %v want OUTSIDE", got)
	}
}

// --- nearestVisitor.VisitWithPackedValue ------------------------------------

func TestNearestVisitor_FillThenEvictWithLowerDocIDTieBreak(t *testing.T) {
	t.Parallel()

	hq := &nearestHitHeap{}
	heap.Init(hq)
	v := newNearestVisitor(hq, 2, 0, 0) // query origin
	v.curDocBase = 0

	mustVisit := func(t *testing.T, docID int, lat, lon float64) {
		t.Helper()
		if err := v.VisitWithPackedValue(docID, packedLatLon(lat, lon)); err != nil {
			t.Fatalf("VisitWithPackedValue: %v", err)
		}
	}

	// Heap initially seeds with two hits at non-trivial distances.
	mustVisit(t, 10, 1, 1) // distance ~157km
	mustVisit(t, 20, 2, 2) // distance ~314km, becomes worst.

	if hq.Len() != 2 {
		t.Fatalf("heap len after fill: got %d want 2", hq.Len())
	}
	if (*hq)[0].DocID != 20 {
		t.Fatalf("worst doc after fill: got %d want 20", (*hq)[0].DocID)
	}

	// A nearer doc must evict the worst.
	mustVisit(t, 30, 0.5, 0.5) // smaller distance.
	if hq.Len() != 2 {
		t.Fatalf("heap len after eviction: got %d want 2", hq.Len())
	}
	docs := []int{(*hq)[0].DocID, (*hq)[1].DocID}
	sort.Ints(docs)
	if docs[0] != 10 || docs[1] != 30 {
		t.Fatalf("heap docs after eviction: got %v want [10 30]", docs)
	}

	// A doc at *equal* distance to the current worst, but with a
	// lower docID, must evict the worst (tie-break "fullDocID < hit.DocID").
	worstDist := (*hq)[0].DistanceSortKey
	worstDocID := (*hq)[0].DocID
	// Find an existing doc at distance == worstDist by computing the
	// inverse: a point on the same lat/lon arc. We re-use the same
	// lat/lon as the current worst and just hand a lower docID.
	worstLat, worstLon := 1.0, 1.0
	if worstDocID == 30 {
		worstLat, worstLon = 0.5, 0.5
	}
	// New doc id = 1, lower than any current head; same distance.
	mustVisit(t, 1, worstLat, worstLon)

	if hq.Len() != 2 {
		t.Fatalf("heap len after tie-break: got %d want 2", hq.Len())
	}
	// The current worst's docID must now be the highest of the
	// surviving two; we asserted worstDocID was present before the tie
	// (it has been replaced by docID=1 if the tie-break fired).
	found := false
	for _, h := range *hq {
		if h.DocID == 1 && h.DistanceSortKey == worstDist {
			found = true
		}
	}
	if !found {
		t.Fatalf("tie-break did not insert docID=1: heap=%v", *hq)
	}
}

// --- End-to-end: in-memory binary BKD walker -------------------------------

// memTreeNode is a node of the in-memory binary BKD used in the
// end-to-end test. Leaf nodes hold packed point/docID tuples; non-leaf
// nodes hold two children. Min/max packed are computed at construction.
type memTreeNode struct {
	min, max []byte

	// Leaf payload.
	leaf   bool
	points [][]byte
	docs   []int

	// Children.
	left, right *memTreeNode
}

// memTreeWalker is a cursor over [memTreeNode]. It implements
// [PointTreeWalker] with the same "moveToChild walks to the first
// child, moveToSibling walks to the right sibling" semantics as
// PointTree in Lucene.
type memTreeWalker struct {
	// stack[len-1] is the current node. We push on MoveToChild and pop
	// on MoveToSibling-after-MoveToChild-from-right.
	stack []memCursor
}

type memCursor struct {
	node    *memTreeNode
	parent  *memTreeNode // nil for root
	isRight bool         // true when node == parent.right
}

func newMemTreeWalker(root *memTreeNode) *memTreeWalker {
	return &memTreeWalker{stack: []memCursor{{node: root}}}
}

func (w *memTreeWalker) current() memCursor { return w.stack[len(w.stack)-1] }

func (w *memTreeWalker) MinPackedValue() []byte { return w.current().node.min }
func (w *memTreeWalker) MaxPackedValue() []byte { return w.current().node.max }

func (w *memTreeWalker) MoveToChild() bool {
	cur := w.current()
	if cur.node.leaf {
		return false
	}
	w.stack = append(w.stack, memCursor{
		node:    cur.node.left,
		parent:  cur.node,
		isRight: false,
	})
	return true
}

func (w *memTreeWalker) MoveToSibling() bool {
	cur := w.current()
	if cur.parent == nil || cur.isRight {
		return false
	}
	w.stack[len(w.stack)-1] = memCursor{
		node:    cur.parent.right,
		parent:  cur.parent,
		isRight: true,
	}
	return true
}

func (w *memTreeWalker) Clone() PointTreeWalker {
	dup := make([]memCursor, len(w.stack))
	copy(dup, w.stack)
	return &memTreeWalker{stack: dup}
}

func (w *memTreeWalker) VisitDocValues(v PointTreeNearestVisitor) error {
	cur := w.current().node
	if !cur.leaf {
		return nil
	}
	for i, p := range cur.points {
		if err := v.VisitWithPackedValue(cur.docs[i], p); err != nil {
			return err
		}
	}
	return nil
}

// buildBinaryTree partitions points by dimension 0 (latitude) and then
// dimension 1 (longitude), recursively. Leaves carry up to leafSize
// points. The resulting tree's min/max are the per-dimension extents
// of the subtree's points, matching the BKD contract.
func buildBinaryTree(points [][]byte, docs []int, depth, leafSize int) *memTreeNode {
	if len(points) == 0 {
		return nil
	}
	if len(points) <= leafSize {
		n := &memTreeNode{
			leaf:   true,
			points: append([][]byte(nil), points...),
			docs:   append([]int(nil), docs...),
		}
		n.min, n.max = computeMinMax(points)
		return n
	}
	// Sort by dimension == depth % 2 (0 = lat bytes, 1 = lon bytes).
	dim := depth % 2
	offset := dim * 4
	idxs := make([]int, len(points))
	for i := range idxs {
		idxs[i] = i
	}
	sort.Slice(idxs, func(i, j int) bool {
		return compareBytes(points[idxs[i]][offset:offset+4], points[idxs[j]][offset:offset+4]) < 0
	})
	sortedPoints := make([][]byte, len(points))
	sortedDocs := make([]int, len(points))
	for i, idx := range idxs {
		sortedPoints[i] = points[idx]
		sortedDocs[i] = docs[idx]
	}
	mid := len(sortedPoints) / 2
	n := &memTreeNode{
		left:  buildBinaryTree(sortedPoints[:mid], sortedDocs[:mid], depth+1, leafSize),
		right: buildBinaryTree(sortedPoints[mid:], sortedDocs[mid:], depth+1, leafSize),
	}
	n.min, n.max = computeMinMax(sortedPoints)
	return n
}

func computeMinMax(points [][]byte) (minPacked, maxPacked []byte) {
	if len(points) == 0 {
		return nil, nil
	}
	minPacked = append([]byte(nil), points[0]...)
	maxPacked = append([]byte(nil), points[0]...)
	for _, p := range points[1:] {
		// Per dimension (2 dims × 4 bytes).
		for dim := 0; dim < 2; dim++ {
			off := dim * 4
			if compareBytes(p[off:off+4], minPacked[off:off+4]) < 0 {
				copy(minPacked[off:off+4], p[off:off+4])
			}
			if compareBytes(p[off:off+4], maxPacked[off:off+4]) > 0 {
				copy(maxPacked[off:off+4], p[off:off+4])
			}
		}
	}
	return minPacked, maxPacked
}

// compareBytes is a per-byte unsigned comparator (the sortable-bytes
// encoding is unsigned-comparable by design).
func compareBytes(a, b []byte) int {
	for i := range a {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func TestNearest_EndToEnd_TopKMatchesBruteForce(t *testing.T) {
	t.Parallel()

	type pt struct {
		docID    int
		lat, lon float64
	}
	corpus := []pt{
		{0, 38.7223, -9.1393},   // Lisbon
		{1, 38.7138, -9.1393},   // Just south of origin
		{2, 40.4168, -3.7038},   // Madrid (~500km)
		{3, 41.3851, 2.1734},    // Barcelona (~1000km)
		{4, 48.8566, 2.3522},    // Paris
		{5, 51.5074, -0.1278},   // London
		{6, 52.5200, 13.4050},   // Berlin
		{7, 38.7500, -9.1500},   // Very close to origin
		{8, 38.7100, -9.1200},   // Close
		{9, 39.4699, -0.3763},   // Valencia
		{10, 37.9755, -1.1280},  // Murcia
		{11, 35.6762, 139.6503}, // Tokyo (across the dateline path)
	}

	packed := make([][]byte, len(corpus))
	docs := make([]int, len(corpus))
	for i, p := range corpus {
		packed[i] = packedLatLon(p.lat, p.lon)
		docs[i] = p.docID
	}
	root := buildBinaryTree(packed, docs, 0, 2)
	if root == nil {
		t.Fatal("buildBinaryTree: nil root for non-empty corpus")
	}

	pointLat, pointLon := 38.7223, -9.1393 // Lisbon
	n := 5

	hits, err := Nearest(pointLat, pointLon, []PointTreeNearestReader{
		{Tree: newMemTreeWalker(root), DocBase: 0},
	}, n)
	if err != nil {
		t.Fatalf("Nearest: %v", err)
	}
	if len(hits) != n {
		t.Fatalf("len(hits): got %d want %d", len(hits), n)
	}

	// Brute force: compute the haversin sort key for every doc using
	// the *encoded-then-decoded* lat/lon (matching what the BKD visitor
	// observes after the sortable-bytes round-trip), sort by
	// (distance asc, docID asc), take top n.
	type ranked struct {
		docID int
		key   float64
	}
	all := make([]ranked, 0, len(corpus))
	for i, p := range corpus {
		decLat := geo.DecodeLatitudeBytes(packed[i], 0)
		decLon := geo.DecodeLongitudeBytes(packed[i], 4)
		all = append(all, ranked{
			docID: p.docID,
			key:   util.HaversinSortKey(pointLat, pointLon, decLat, decLon),
		})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].key != all[j].key {
			return all[i].key < all[j].key
		}
		return all[i].docID < all[j].docID
	})
	want := all[:n]

	for i := range hits {
		if hits[i].DocID != want[i].docID || hits[i].DistanceSortKey != want[i].key {
			t.Fatalf("hit[%d]: got (docID=%d, key=%g) want (docID=%d, key=%g)",
				i, hits[i].DocID, hits[i].DistanceSortKey, want[i].docID, want[i].key)
		}
	}

	// Hits must be best-first.
	for i := 1; i < len(hits); i++ {
		if hits[i-1].DistanceSortKey > hits[i].DistanceSortKey {
			t.Fatalf("hits not sorted asc at %d: %v", i, hits)
		}
	}
}

func TestNearest_DocBaseGlobalisation(t *testing.T) {
	t.Parallel()

	// Two readers (segments). Each has a single point, with different
	// doc bases so the final docIDs must be globalised correctly.
	pointA := packedLatLon(0, 0)
	pointB := packedLatLon(0.01, 0.01)

	leafA := &memTreeNode{leaf: true, points: [][]byte{pointA}, docs: []int{4}}
	leafA.min, leafA.max = computeMinMax(leafA.points)
	leafB := &memTreeNode{leaf: true, points: [][]byte{pointB}, docs: []int{7}}
	leafB.min, leafB.max = computeMinMax(leafB.points)

	readers := []PointTreeNearestReader{
		{Tree: newMemTreeWalker(leafA), DocBase: 100},
		{Tree: newMemTreeWalker(leafB), DocBase: 200},
	}

	hits, err := Nearest(0, 0, readers, 2)
	if err != nil {
		t.Fatalf("Nearest: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("len(hits): got %d want 2", len(hits))
	}
	// Reader 0 carries the exact origin → must be the closer hit.
	if hits[0].DocID != 104 {
		t.Fatalf("hit[0].DocID: got %d want 104 (= 100 + 4)", hits[0].DocID)
	}
	if hits[1].DocID != 207 {
		t.Fatalf("hit[1].DocID: got %d want 207 (= 200 + 7)", hits[1].DocID)
	}
}

func TestNearest_LiveDocsFilterDropsDeleted(t *testing.T) {
	t.Parallel()

	pts := [][]byte{
		packedLatLon(0, 0),
		packedLatLon(0.001, 0.001),
		packedLatLon(0.002, 0.002),
	}
	leaf := &memTreeNode{
		leaf:   true,
		points: pts,
		docs:   []int{0, 1, 2},
	}
	leaf.min, leaf.max = computeMinMax(pts)

	// Mark docID=0 as deleted.
	live := &simpleBits{set: []bool{false, true, true}}

	hits, err := Nearest(0, 0, []PointTreeNearestReader{
		{Tree: newMemTreeWalker(leaf), LiveDocs: live, DocBase: 0},
	}, 5)
	if err != nil {
		t.Fatalf("Nearest: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("len(hits): got %d want 2 (one deleted)", len(hits))
	}
	for _, h := range hits {
		if h.DocID == 0 {
			t.Fatalf("deleted docID=0 leaked into hits: %v", hits)
		}
	}
}

func TestNearest_EmptyReadersReturnsEmpty(t *testing.T) {
	t.Parallel()

	hits, err := Nearest(0, 0, nil, 5)
	if err != nil {
		t.Fatalf("Nearest: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("len(hits): got %d want 0", len(hits))
	}
}

func TestNearest_NonPositiveN_Rejected(t *testing.T) {
	t.Parallel()

	for _, n := range []int{0, -1, -42} {
		if _, err := Nearest(0, 0, nil, n); err == nil {
			t.Fatalf("Nearest(n=%d): expected error", n)
		}
	}
}

// simpleBits is a tiny [util.Bits] for the live-docs test.
type simpleBits struct{ set []bool }

func (b *simpleBits) Get(i int) bool {
	if i < 0 || i >= len(b.set) {
		return false
	}
	return b.set[i]
}
func (b *simpleBits) Length() int { return len(b.set) }
