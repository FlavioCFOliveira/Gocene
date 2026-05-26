// Code in this file mirrors org.apache.lucene.geo.Tessellator from
// Apache Lucene 10.4.0.
//
// The algorithm is an earcut-based tessellator inspired by Mapbox's
// earcut (https://github.com/mapbox/earcut), a modification of FIST
// (https://www.cosy.sbg.ac.at/~held/projects/triang/triang.html) and
// David Eberly's ear-clipping technique.
//
// Feature parity with Lucene 10.4.0:
//   - Single-ring polygon triangulation (both Polygon and XYPolygon).
//   - Hole elimination via tangent-pair bridge search and shared-vertex
//     merge.
//   - Morton-curve z-order acceleration for polygons above the 80-vertex
//     threshold.
//   - Self-intersection detection (when checkSelfIntersections = true).
//   - Three-state earcut loop: INIT → CURE (local self-intersection
//     repair) → SPLIT.
//
// # Deviation from Lucene
//
// The Java Monitor callback (debug observer) is not ported; callers
// that need tessellation diagnostics should add their own callbacks
// outside this package.
//
// The Java Triangle exposes getEncodedX/Y (integer-encoded lat/lon or
// XY values). The Go Triangle exposes the raw float64 coordinates
// (AX, AY, …) and lets callers encode them; this preserves the
// existing Gocene API used by latlon_shape.go and xy_shape.go.
package geo

import (
	"errors"
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// vertexThreshold is the point count above which Morton-order z-sort
// acceleration is enabled. Mirrors Tessellator.VERTEX_THRESHOLD.
const vertexThreshold = 80

// Triangle is the unit of output produced by the Tessellator. It
// holds three (x, y) vertices (longitude/latitude for geographic
// polygons, x/y for cartesian XYPolygons) plus a per-edge flag
// indicating whether the edge belongs to the original polygon
// boundary (true) or was synthesised by triangulation (false).
//
// It mirrors org.apache.lucene.geo.Tessellator.Triangle.
type Triangle struct {
	ax, ay float64
	bx, by float64
	cx, cy float64

	// Per-edge "belongs to original polygon edge" flags.
	abFromPolygon bool
	bcFromPolygon bool
	caFromPolygon bool
}

// AX / AY / BX / BY / CX / CY return the per-vertex coordinates.
func (t *Triangle) AX() float64 { return t.ax }
func (t *Triangle) AY() float64 { return t.ay }
func (t *Triangle) BX() float64 { return t.bx }
func (t *Triangle) BY() float64 { return t.by }
func (t *Triangle) CX() float64 { return t.cx }
func (t *Triangle) CY() float64 { return t.cy }

// EdgeFromPolygon reports whether the edge between vertices a and b,
// b and c, or c and a (indexed 0, 1, 2 respectively) belongs to the
// original polygon boundary.
func (t *Triangle) EdgeFromPolygon(edge int) bool {
	switch edge {
	case 0:
		return t.abFromPolygon
	case 1:
		return t.bcFromPolygon
	case 2:
		return t.caFromPolygon
	}
	panic(fmt.Sprintf("geo: invalid edge index %d", edge))
}

// ErrTessellatorUnsupported is returned for polygon shapes that fall
// outside the tessellator's capability (should be rare with the full
// implementation; kept for API compatibility with callers that use
// errors.Is).
var ErrTessellatorUnsupported = errors.New("geo: tessellator unsupported shape")

// ErrTessellatorMalformed is returned when the input polygon is
// degenerate (fewer than three non-collinear vertices) or cannot be
// triangulated.
var ErrTessellatorMalformed = errors.New("geo: tessellator detected a malformed shape")

// Tessellate triangulates a geographic Polygon and returns the list
// of triangles produced by earcut tessellation.
//
// It is the Go port of org.apache.lucene.geo.Tessellator.tessellate(Polygon,boolean).
func Tessellate(polygon Polygon, checkSelfIntersections bool) ([]Triangle, error) {
	lons := polygon.PolyLons()
	lats := polygon.PolyLats()

	outerNode, err := createDoublyLinkedListGeo(lons, lats, polygon.WindingOrder(), true, 0, WindingClockwise)
	if err != nil {
		return nil, err
	}
	if outerNode == nil {
		return nil, fmt.Errorf("%w: malformed shape detected", ErrTessellatorMalformed)
	}
	if outerNode == outerNode.next || outerNode == outerNode.next.next {
		return nil, fmt.Errorf("%w: at least three non-collinear points required", ErrTessellatorMalformed)
	}

	if polygon.NumHoles() > 0 {
		outerNode, err = eliminateHolesGeo(polygon, outerNode)
		if err != nil {
			return nil, err
		}
	}

	threshold := vertexThreshold - polygon.NumPoints()
	for i := 0; threshold >= 0 && i < polygon.NumHoles(); i++ {
		threshold -= polygon.Hole(i).NumPoints()
	}
	mortonOptimized := threshold < 0
	if mortonOptimized {
		sortByMorton(outerNode)
	}

	if checkSelfIntersections {
		if err2 := checkIntersection(outerNode, mortonOptimized); err2 != nil {
			return nil, err2
		}
	}

	result := earcutLinkedList(outerNode, nil, stateInit, mortonOptimized, 0)
	if len(result) == 0 {
		return nil, fmt.Errorf("%w: unable to tessellate shape", ErrTessellatorMalformed)
	}
	return result, nil
}

// TessellateXYPolygon triangulates a cartesian XYPolygon and returns the list
// of triangles, including support for holes.
//
// It is the Go port of org.apache.lucene.geo.Tessellator.tessellate(XYPolygon,boolean).
func TessellateXYPolygon(polygon XYPolygon, checkSelfIntersections bool) ([]Triangle, error) {
	xs := float32SliceToFloat64(polygon.PolyXs())
	ys := float32SliceToFloat64(polygon.PolyYs())

	outerNode, err := createDoublyLinkedListXY(xs, ys, polygon.WindingOrder(), false, 0, WindingClockwise)
	if err != nil {
		return nil, err
	}
	if outerNode == nil {
		return nil, fmt.Errorf("%w: malformed shape detected", ErrTessellatorMalformed)
	}
	if outerNode == outerNode.next || outerNode == outerNode.next.next {
		return nil, fmt.Errorf("%w: at least three non-collinear points required", ErrTessellatorMalformed)
	}

	if polygon.NumHoles() > 0 {
		outerNode, err = eliminateHolesXY(polygon, outerNode)
		if err != nil {
			return nil, err
		}
	}

	threshold := vertexThreshold - len(xs)
	for i := 0; threshold >= 0 && i < polygon.NumHoles(); i++ {
		threshold -= len(float32SliceToFloat64(polygon.Holes()[i].PolyXs()))
	}
	mortonOptimized := threshold < 0
	if mortonOptimized {
		sortByMorton(outerNode)
	}

	if checkSelfIntersections {
		if err2 := checkIntersection(outerNode, mortonOptimized); err2 != nil {
			return nil, err2
		}
	}

	result := earcutLinkedList(outerNode, nil, stateInit, mortonOptimized, 0)
	if len(result) == 0 {
		return nil, fmt.Errorf("%w: unable to tessellate shape", ErrTessellatorMalformed)
	}
	return result, nil
}

// TessellateXY tessellates a raw XY polygon given as parallel float64 slices.
// holes is the number of holes; if holes > 0 ErrTessellatorUnsupported is
// returned (use TessellateXYPolygon for full hole support).
func TessellateXY(xs, ys []float64, holes int, checkSelfIntersections bool) ([]Triangle, error) {
	if holes > 0 {
		return nil, fmt.Errorf("%w: use TessellateXYPolygon for polygons with holes", ErrTessellatorUnsupported)
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("%w: x/y length mismatch", ErrTessellatorMalformed)
	}

	polyWinding := rawWindingOrder(xs, ys)
	outerNode, err := createDoublyLinkedListXY(xs, ys, polyWinding, false, 0, WindingClockwise)
	if err != nil {
		return nil, err
	}
	if outerNode == nil {
		return nil, fmt.Errorf("%w: malformed shape detected", ErrTessellatorMalformed)
	}
	if outerNode == outerNode.next || outerNode == outerNode.next.next {
		return nil, fmt.Errorf("%w: at least three non-collinear points required", ErrTessellatorMalformed)
	}

	mortonOptimized := len(xs) > vertexThreshold
	if mortonOptimized {
		sortByMorton(outerNode)
	}

	if checkSelfIntersections {
		if err2 := checkIntersection(outerNode, mortonOptimized); err2 != nil {
			return nil, err2
		}
	}

	result := earcutLinkedList(outerNode, nil, stateInit, mortonOptimized, 0)
	if len(result) == 0 {
		return nil, fmt.Errorf("%w: unable to tessellate shape", ErrTessellatorMalformed)
	}
	return result, nil
}

// ─── earcut state machine ─────────────────────────────────────────────────────

type earcutState int

const (
	stateInit  earcutState = iota
	stateCure              // try curing local self-intersections
	stateSplit             // try splitting the polygon
)

// earcutLinkedList is the main ear-slicing loop.
//
// Port of Tessellator.earcutLinkedList.
func earcutLinkedList(
	currEar *tessNode,
	tessellation []Triangle,
	state earcutState,
	mortonOptimized bool,
	depth int,
) []Triangle {
	if tessellation == nil {
		tessellation = make([]Triangle, 0)
	}

outer:
	for {
		if currEar == nil || currEar.previous == currEar.next {
			return tessellation
		}

		stop := currEar

		for {
			prevNode := currEar.previous
			nextNode := currEar.next

			isReflex := tessArea(
				prevNode.getX(), prevNode.getY(),
				currEar.getX(), currEar.getY(),
				nextNode.getX(), nextNode.getY(),
			) >= 0

			if !isReflex && isEar(currEar, mortonOptimized) {
				abFromPolygon := prevNode.isNextEdgeFromPolygon
				bcFromPolygon := currEar.isNextEdgeFromPolygon
				caFromPolygon := isEdgeFromPolygon(prevNode, nextNode, mortonOptimized)

				tessellation = append(tessellation, Triangle{
					ax: prevNode.getX(), ay: prevNode.getY(),
					bx: currEar.getX(), by: currEar.getY(),
					cx: nextNode.getX(), cy: nextNode.getY(),
					abFromPolygon: abFromPolygon,
					bcFromPolygon: bcFromPolygon,
					caFromPolygon: caFromPolygon,
				})

				removeNode(currEar, caFromPolygon)
				currEar = nextNode.next
				stop = nextNode.next
				continue
			}

			currEar = nextNode
			if currEar == stop {
				switch state {
				case stateInit:
					currEar = filterPoints(currEar, nil)
					state = stateCure
					continue outer
				case stateCure:
					currEar, tessellation = cureLocalIntersections(currEar, tessellation, mortonOptimized)
					state = stateSplit
					continue outer
				case stateSplit:
					splitEarcut(currEar, &tessellation, mortonOptimized, depth+1)
				}
				break
			}

			if currEar.previous == currEar.next {
				break
			}
		}
		break
	}
	return tessellation
}

// ─── tessellator node ─────────────────────────────────────────────────────────

// tessNode is a single vertex in the ear-clipping circular doubly-linked list.
// Mirrors org.apache.lucene.geo.Tessellator.Node.
type tessNode struct {
	// idx is the node index in the linked list.
	idx int
	// vrtxIdx is the vertex index in the source polygon slice.
	vrtxIdx int

	// Coordinates stored both as encoded int32 (for Morton code) and via the
	// source float64 slices (for actual geometry tests).
	polyX []float64
	polyY []float64
	// encoded x / y for Morton indexing.
	encX int32
	encY int32
	// Morton code for z-order sorting.
	morton int64
	// isGeo distinguishes geographic (lat/lon) from Cartesian (x/y) encoding.
	isGeo bool

	previous, next        *tessNode
	previousZ, nextZ      *tessNode
	isNextEdgeFromPolygon bool
}

func (n *tessNode) getX() float64 { return n.polyX[n.vrtxIdx] }
func (n *tessNode) getY() float64 { return n.polyY[n.vrtxIdx] }

// newTessNode constructs a tessNode with encoded coordinates and Morton code.
//
// Port of Tessellator.Node(double[], double[], int, int, boolean).
func newTessNode(x, y []float64, idx, vrtxIdx int, isGeo bool) *tessNode {
	var ex, ey int32
	if isGeo {
		ex = EncodeLongitude(x[vrtxIdx])
		ey = EncodeLatitude(y[vrtxIdx])
	} else {
		ex = XYEncode(float32(x[vrtxIdx]))
		ey = XYEncode(float32(y[vrtxIdx]))
	}
	// Morton interleave: flip high bit so unsigned comparison works for negative encoded values.
	morton := util.Interleave(int(uint32(ex)^0x80000000), int(uint32(ey)^0x80000000))
	n := &tessNode{
		idx:                   idx,
		vrtxIdx:               vrtxIdx,
		polyX:                 x,
		polyY:                 y,
		encX:                  ex,
		encY:                  ey,
		morton:                morton,
		isGeo:                 isGeo,
		isNextEdgeFromPolygon: true,
	}
	n.previous = n
	n.previousZ = n
	n.next = n
	n.nextZ = n
	return n
}

// cloneNode creates a shallow copy of src as used by splitPolygon.
func cloneNode(src *tessNode) *tessNode {
	c := *src
	return &c
}

// ─── doubly-linked list construction ─────────────────────────────────────────

// createDoublyLinkedListGeo builds the circular doubly-linked list for a
// geographic polygon ring.  The ring is traversed in the direction opposite
// windingOrder (to obtain CW winding from the outer ring which is CCW).
func createDoublyLinkedListGeo(
	x, y []float64,
	polyWindingOrder WindingOrder,
	isGeo bool,
	startIndex int,
	windingOrder WindingOrder,
) (*tessNode, error) {
	var lastNode *tessNode
	var idx int

	if windingOrder == polyWindingOrder {
		for i := 0; i < len(x); i++ {
			lastNode = insertNode(x, y, startIndex+idx, i, lastNode, isGeo)
			idx++
		}
	} else {
		for i := len(x) - 1; i >= 0; i-- {
			lastNode = insertNode(x, y, startIndex+idx, i, lastNode, isGeo)
			idx++
		}
	}

	if lastNode != nil && isVertexEqualsByCoord(lastNode, lastNode.next) {
		removeNode(lastNode, true)
		lastNode = lastNode.next
	}
	return filterPoints(lastNode, nil), nil
}

// createDoublyLinkedListXY builds the list for a Cartesian polygon ring.
func createDoublyLinkedListXY(
	x, y []float64,
	polyWindingOrder WindingOrder,
	isGeo bool,
	startIndex int,
	windingOrder WindingOrder,
) (*tessNode, error) {
	return createDoublyLinkedListGeo(x, y, polyWindingOrder, isGeo, startIndex, windingOrder)
}

// insertNode inserts a new node after lastNode (or creates a singleton ring).
//
// Port of Tessellator.insertNode.
func insertNode(x, y []float64, idx, vrtxIdx int, lastNode *tessNode, isGeo bool) *tessNode {
	node := newTessNode(x, y, idx, vrtxIdx, isGeo)
	if lastNode == nil {
		// First node: it is its own circular list.
		return node
	}
	// Insert after lastNode.
	after := lastNode.next
	node.next = after
	node.nextZ = after
	node.previous = lastNode
	node.previousZ = lastNode
	after.previous = node
	after.previousZ = node
	lastNode.next = node
	lastNode.nextZ = node
	return node
}

// ─── hole elimination ─────────────────────────────────────────────────────────

// eliminateHolesGeo links all holes of a geographic Polygon into the outer ring.
//
// Port of Tessellator.eliminateHoles(Polygon, Node).
func eliminateHolesGeo(polygon Polygon, outerNode *tessNode) (*tessNode, error) {
	type holeEntry struct {
		node   *tessNode
		minLon float64
		maxLon float64
		minLat float64
		maxLat float64
	}
	var holeList []holeEntry
	nodeIndex := polygon.NumPoints()
	for i := 0; i < polygon.NumHoles(); i++ {
		h := polygon.Hole(i)
		list, err := createDoublyLinkedListGeo(
			h.PolyLons(), h.PolyLats(),
			h.WindingOrder(), true,
			nodeIndex, WindingCounterClockwise,
		)
		if err != nil {
			return nil, err
		}
		if list != nil && list == list.next {
			return nil, fmt.Errorf("geo: points are all coplanar in hole %d", i)
		}
		if list != nil {
			leftMost := fetchLeftmost(list)
			holeList = append(holeList, holeEntry{
				node:   leftMost,
				minLon: h.MinLon(),
				maxLon: h.MaxLon(),
				minLat: h.MinLat(),
				maxLat: h.MaxLat(),
			})
		}
		nodeIndex += h.NumPoints()
	}

	// Sort holes by x (lon) coordinate.
	sort.Slice(holeList, func(i, j int) bool {
		a, b := holeList[i].node, holeList[j].node
		diff := a.getX() - b.getX()
		if diff == 0 {
			diff = a.getY() - b.getY()
			if diff == 0 {
				ay := min64(a.previous.getY(), a.next.getY())
				by := min64(b.previous.getY(), b.next.getY())
				diff = ay - by
			}
		}
		return diff < 0
	})

	var err error
	for _, h := range holeList {
		result := eliminateHole(h.node, outerNode, h.minLon, h.maxLon, h.minLat, h.maxLat)
		if result != nil {
			outerNode = filterPoints(result, result.next)
		} else {
			return nil, fmt.Errorf("geo: %w: illegal hole detected", ErrTessellatorUnsupported)
		}
	}
	return filterPoints(outerNode, nil), err
}

// eliminateHolesXY links all holes of a Cartesian XYPolygon into the outer ring.
//
// Port of Tessellator.eliminateHoles(XYPolygon, Node).
func eliminateHolesXY(polygon XYPolygon, outerNode *tessNode) (*tessNode, error) {
	type holeEntry struct {
		node *tessNode
		minX float64
		maxX float64
		minY float64
		maxY float64
	}
	var holeList []holeEntry
	nodeIndex := len(polygon.PolyXs())
	for i := 0; i < polygon.NumHoles(); i++ {
		h := polygon.Holes()[i]
		xs := float32SliceToFloat64(h.PolyXs())
		ys := float32SliceToFloat64(h.PolyYs())
		list, err := createDoublyLinkedListXY(
			xs, ys,
			h.WindingOrder(), false,
			nodeIndex, WindingCounterClockwise,
		)
		if err != nil {
			return nil, err
		}
		if list != nil {
			leftMost := fetchLeftmost(list)
			holeList = append(holeList, holeEntry{
				node: leftMost,
				minX: float64(h.MinX()),
				maxX: float64(h.MaxX()),
				minY: float64(h.MinY()),
				maxY: float64(h.MaxY()),
			})
		}
		nodeIndex += len(xs)
	}

	sort.Slice(holeList, func(i, j int) bool {
		a, b := holeList[i].node, holeList[j].node
		diff := a.getX() - b.getX()
		if diff == 0 {
			diff = a.getY() - b.getY()
			if diff == 0 {
				ay := min64(a.previous.getY(), a.next.getY())
				by := min64(b.previous.getY(), b.next.getY())
				diff = ay - by
			}
		}
		return diff < 0
	})

	for _, h := range holeList {
		result := eliminateHole(h.node, outerNode, h.minX, h.maxX, h.minY, h.maxY)
		if result != nil {
			outerNode = filterPoints(result, result.next)
		} else {
			return nil, fmt.Errorf("geo: %w: illegal hole detected", ErrTessellatorUnsupported)
		}
	}
	return filterPoints(outerNode, nil), nil
}

// eliminateHole finds a bridge between the hole's leftmost vertex and the outer
// ring, then links them.  Returns the new bridge node (or outer node), or nil
// if no bridge was found.
//
// Port of Tessellator.eliminateHole(Node, Node, double, double, double, double).
func eliminateHole(
	holeNode, outerNode *tessNode,
	holeMinX, holeMaxX, holeMinY, holeMaxY float64,
) *tessNode {
	// Try shared-vertex merge first.
	mergeResult := maybeMergeHoleWithSharedVertices(holeNode, outerNode, holeMinX, holeMaxX, holeMinY, holeMaxY)
	if mergeResult != nil {
		return mergeResult
	}

	// Fall back to bridge-based elimination.
	bridge := fetchHoleBridge(holeNode, outerNode)
	if bridge != nil {
		fromPolygon := isPointInLine(bridge, bridge.next, holeNode.getX(), holeNode.getY()) ||
			isPointInLine(holeNode, holeNode.next, bridge.getX(), bridge.getY())
		splitPolygon(bridge, holeNode, fromPolygon)
		return outerNode
	}
	return nil
}

// maybeMergeHoleWithSharedVertices merges a hole into the outer ring when they
// share a vertex.
//
// Port of Tessellator.maybeMergeHoleWithSharedVertices.
func maybeMergeHoleWithSharedVertices(
	holeNode, outerNode *tessNode,
	holeMinX, holeMaxX, holeMinY, holeMaxY float64,
) *tessNode {
	var sharedVertex *tessNode
	var sharedVertexConnection *tessNode
	var leftmostSharedVertexConnection *tessNode

	next := outerNode
	for {
		if ContainsPoint(next.getY(), next.getX(), holeMinY, holeMaxY, holeMinX, holeMaxX) {
			newSharedVertex := getSharedVertex(holeNode, next)
			if newSharedVertex != nil {
				if isVertexEqualsByCoord(newSharedVertex, holeNode) {
					if leftmostSharedVertexConnection == nil {
						leftmostSharedVertexConnection = next
					}
				}
				if sharedVertex == nil {
					sharedVertex = newSharedVertex
					sharedVertexConnection = next
				} else if isVertexEqualsByCoord(newSharedVertex, sharedVertex) {
					sharedVertexConnection = getSharedInsideVertex(sharedVertex, sharedVertexConnection, next)
				}
			}
		}
		next = next.next
		if next == outerNode {
			break
		}
	}

	if leftmostSharedVertexConnection != nil &&
		leftmostSharedVertexConnection.idx >= sharedVertexConnection.idx {
		splitPolygon(leftmostSharedVertexConnection, holeNode, true)
		if leftmostSharedVertexConnection != sharedVertexConnection {
			return leftmostSharedVertexConnection
		}
		return outerNode
	}
	if sharedVertex != nil {
		splitPolygon(sharedVertexConnection, sharedVertex, true)
		return outerNode
	}
	return nil
}

// getSharedVertex searches the polygon ring for a vertex that equals vertex.
func getSharedVertex(polygon, vertex *tessNode) *tessNode {
	next := polygon
	for {
		if isVertexEqualsByCoord(next, vertex) {
			return next
		}
		next = next.next
		if next == polygon {
			break
		}
	}
	return nil
}

// getSharedInsideVertex returns the candidate (a or b) with the smaller angle
// to holeVertex.
//
// Port of Tessellator.getSharedInsideVertex.
func getSharedInsideVertex(holeVertex, candidateA, candidateB *tessNode) *tessNode {
	a1 := tessArea(
		candidateA.previous.getX(), candidateA.previous.getY(),
		holeVertex.getX(), holeVertex.getY(),
		holeVertex.next.getX(), holeVertex.next.getY(),
	)
	a2 := tessArea(
		candidateB.previous.getX(), candidateB.previous.getY(),
		holeVertex.getX(), holeVertex.getY(),
		holeVertex.next.getX(), holeVertex.next.getY(),
	)
	if (a1 < 0) != (a2 < 0) {
		if a1 < a2 {
			return candidateA
		}
		return candidateB
	}
	ang1 := tessAngle(candidateA.previous, candidateA, holeVertex.next)
	ang2 := tessAngle(candidateB.previous, candidateB, holeVertex.next)
	if ang1 < ang2 {
		return candidateA
	}
	return candidateB
}

// fetchHoleBridge finds a bridge vertex on the outer ring for the hole's
// leftmost vertex.
//
// Port of Tessellator.fetchHoleBridge.
func fetchHoleBridge(holeNode, outerNode *tessNode) *tessNode {
	p := outerNode
	qx := -1e300 // negative infinity
	hx := holeNode.getX()
	hy := holeNode.getY()
	var connection *tessNode

	for {
		py := p.getY()
		pny := p.next.getY()
		if hy <= py && hy >= pny && pny != py {
			x := p.getX() + (hy-py)*(p.next.getX()-p.getX())/(pny-py)
			if x <= hx && x > qx {
				qx = x
				if x == hx {
					if hy == py {
						return p
					}
					if hy == pny {
						return p.next
					}
				}
				if p.getX() < p.next.getX() {
					connection = p
				} else {
					connection = p.next
				}
			}
		}
		p = p.next
		if p == outerNode {
			break
		}
	}

	if connection == nil {
		return nil
	}
	if hx == qx {
		return connection.previous
	}

	stop := connection
	mx := connection.getX()
	my := connection.getY()
	tanMin := 1e300 // positive infinity
	p = connection
	for {
		earX1, earX2 := qx, hx
		if hy < my {
			earX1, earX2 = hx, qx
		}
		if hx >= p.getX() && p.getX() >= mx && hx != p.getX() &&
			pointInEar(p.getX(), p.getY(),
				earX1, hy,
				mx, my,
				earX2, hy) {
			tan := abs64(hy-p.getY()) / (hx - p.getX())
			if (tan < tanMin || (tan == tanMin && p.getX() > connection.getX())) &&
				isLocallyInside(p, holeNode) {
				connection = p
				tanMin = tan
			}
		}
		p = p.next
		if p == stop {
			break
		}
	}
	return connection
}

// fetchLeftmost finds the leftmost vertex in a ring.
func fetchLeftmost(start *tessNode) *tessNode {
	node := start
	leftMost := start
	for {
		if node.getX() < leftMost.getX() ||
			(node.getX() == leftMost.getX() && node.getY() < leftMost.getY()) {
			leftMost = node
		}
		node = node.next
		if node == start {
			break
		}
	}
	return leftMost
}

// ─── ear detection ────────────────────────────────────────────────────────────

// isEar reports whether the triangle (prev, ear, next) is a valid ear.
//
// Port of Tessellator.isEar(Node, boolean).
func isEar(ear *tessNode, mortonOptimized bool) bool {
	if mortonOptimized {
		return mortonIsEar(ear)
	}
	node := ear.next.next
	for node != ear.previous {
		if pointInEar(node.getX(), node.getY(),
			ear.previous.getX(), ear.previous.getY(),
			ear.getX(), ear.getY(),
			ear.next.getX(), ear.next.getY()) &&
			tessArea(node.previous.getX(), node.previous.getY(),
				node.getX(), node.getY(),
				node.next.getX(), node.next.getY()) >= 0 {
			return false
		}
		node = node.next
	}
	return true
}

// mortonIsEar uses z-order range queries to find points inside the potential ear.
//
// Port of Tessellator.mortonIsEar.
func mortonIsEar(ear *tessNode) bool {
	// triangle bbox, flip high bit for unsigned comparison of negative encoded values
	minTX := int64(uint32(min32(min32(ear.previous.encX, ear.encX), ear.next.encX)) ^ 0x80000000)
	minTY := int64(uint32(min32(min32(ear.previous.encY, ear.encY), ear.next.encY)) ^ 0x80000000)
	maxTX := int64(uint32(max32(max32(ear.previous.encX, ear.encX), ear.next.encX)) ^ 0x80000000)
	maxTY := int64(uint32(max32(max32(ear.previous.encY, ear.encY), ear.next.encY)) ^ 0x80000000)

	minZ := util.Interleave(int(minTX), int(minTY))
	maxZ := util.Interleave(int(maxTX), int(maxTY))

	p := ear.previousZ
	n := ear.nextZ
	for p != nil && uint64(p.morton) >= uint64(minZ) &&
		n != nil && uint64(n.morton) <= uint64(maxZ) {
		if p.idx != ear.previous.idx && p.idx != ear.next.idx &&
			pointInEar(p.getX(), p.getY(),
				ear.previous.getX(), ear.previous.getY(),
				ear.getX(), ear.getY(),
				ear.next.getX(), ear.next.getY()) &&
			tessArea(p.previous.getX(), p.previous.getY(),
				p.getX(), p.getY(),
				p.next.getX(), p.next.getY()) >= 0 {
			return false
		}
		p = p.previousZ

		if n.idx != ear.previous.idx && n.idx != ear.next.idx &&
			pointInEar(n.getX(), n.getY(),
				ear.previous.getX(), ear.previous.getY(),
				ear.getX(), ear.getY(),
				ear.next.getX(), ear.next.getY()) &&
			tessArea(n.previous.getX(), n.previous.getY(),
				n.getX(), n.getY(),
				n.next.getX(), n.next.getY()) >= 0 {
			return false
		}
		n = n.nextZ
	}

	for p != nil && uint64(p.morton) >= uint64(minZ) {
		if p.idx != ear.previous.idx && p.idx != ear.next.idx &&
			pointInEar(p.getX(), p.getY(),
				ear.previous.getX(), ear.previous.getY(),
				ear.getX(), ear.getY(),
				ear.next.getX(), ear.next.getY()) &&
			tessArea(p.previous.getX(), p.previous.getY(),
				p.getX(), p.getY(),
				p.next.getX(), p.next.getY()) >= 0 {
			return false
		}
		p = p.previousZ
	}

	for n != nil && uint64(n.morton) <= uint64(maxZ) {
		if n.idx != ear.previous.idx && n.idx != ear.next.idx &&
			pointInEar(n.getX(), n.getY(),
				ear.previous.getX(), ear.previous.getY(),
				ear.getX(), ear.getY(),
				ear.next.getX(), ear.next.getY()) &&
			tessArea(n.previous.getX(), n.previous.getY(),
				n.getX(), n.getY(),
				n.next.getX(), n.next.getY()) >= 0 {
			return false
		}
		n = n.nextZ
	}
	return true
}

// ─── self-intersection detection ─────────────────────────────────────────────

// checkIntersection validates that the polygon has no self-intersecting edges.
//
// Port of Tessellator.checkIntersection(Node, boolean).
func checkIntersection(a *tessNode, isMorton bool) error {
	next := a.next
	for next != a.previous {
		if isMorton {
			if err := mortonCheckIntersection(next, next.next); err != nil {
				return err
			}
		} else {
			innerNext := next.next
			for innerNext != next.previous {
				if err := checkIntersectionPoint(next, innerNext); err != nil {
					return err
				}
				innerNext = innerNext.next
			}
		}
		next = next.next
	}
	return nil
}

// mortonCheckIntersection uses z-order range to check for intersecting edges.
//
// Port of Tessellator.mortonCheckIntersection(Node, Node).
func mortonCheckIntersection(a, b *tessNode) error {
	minTX := int64(uint32(min32(a.encX, a.next.encX)) ^ 0x80000000)
	minTY := int64(uint32(min32(a.encY, a.next.encY)) ^ 0x80000000)
	maxTX := int64(uint32(max32(a.encX, a.next.encX)) ^ 0x80000000)
	maxTY := int64(uint32(max32(a.encY, a.next.encY)) ^ 0x80000000)

	minZ := util.Interleave(int(minTX), int(minTY))
	maxZ := util.Interleave(int(maxTX), int(maxTY))

	p := b.previousZ
	n := b.nextZ
	for p != nil && uint64(p.morton) >= uint64(minZ) &&
		n != nil && uint64(n.morton) <= uint64(maxZ) {
		if err := checkIntersectionPoint(p, a); err != nil {
			return err
		}
		p = p.previousZ
		if err := checkIntersectionPoint(n, a); err != nil {
			return err
		}
		n = n.nextZ
	}
	for p != nil && uint64(p.morton) >= uint64(minZ) {
		if err := checkIntersectionPoint(p, a); err != nil {
			return err
		}
		p = p.previousZ
	}
	for n != nil && uint64(n.morton) <= uint64(maxZ) {
		if err := checkIntersectionPoint(n, a); err != nil {
			return err
		}
		n = n.nextZ
	}
	return nil
}

// checkIntersectionPoint raises an error if edges a→a.next and b→b.next
// cross or overlap.
//
// Port of Tessellator.checkIntersectionPoint.
func checkIntersectionPoint(a, b *tessNode) error {
	if a == b {
		return nil
	}
	ay0, ay1 := a.getY(), a.next.getY()
	by0, by1 := b.getY(), b.next.getY()
	ax0, ax1 := a.getX(), a.next.getX()
	bx0, bx1 := b.getX(), b.next.getX()

	if max64(ay0, ay1) <= min64(by0, by1) ||
		min64(ay0, ay1) >= max64(by0, by1) ||
		max64(ax0, ax1) <= min64(bx0, bx1) ||
		min64(ax0, ax1) >= max64(bx0, bx1) {
		return nil
	}

	if LineCrossesLine(ax0, ay0, ax1, ay1, bx0, by0, bx1, by1) {
		// Compute intersection point for error message.
		a1 := ay1 - ay0
		b1 := ax0 - ax1
		c1 := a1*ax0 + b1*ay0
		a2 := by1 - by0
		b2 := bx0 - bx1
		c2 := a2*bx0 + b2*by0
		det := a1*b2 - a2*b1
		if det != 0 {
			ix := (b2*c1 - b1*c2) / det
			iy := (a1*c2 - a2*c1) / det
			return fmt.Errorf("geo: polygon self-intersection at lat=%.8f lon=%.8f", iy, ix)
		}
		return fmt.Errorf("geo: polygon self-intersection detected")
	}
	if a.isNextEdgeFromPolygon && b.isNextEdgeFromPolygon &&
		LineOverlapLine(ax0, ay0, ax1, ay1, bx0, by0, bx1, by1) {
		return fmt.Errorf("geo: polygon ring self-intersection at lat=%.8f lon=%.8f", ay0, ax0)
	}
	return nil
}

// ─── edge-from-polygon detection ─────────────────────────────────────────────

// isEdgeFromPolygon reports whether the diagonal a–b lies on an original
// polygon edge.
//
// Port of Tessellator.isEdgeFromPolygon(Node, Node, boolean).
func isEdgeFromPolygon(a, b *tessNode, isMorton bool) bool {
	if isMorton {
		return isMortonEdgeFromPolygon(a, b)
	}
	next := a
	for {
		if isPointInLine(next, next.next, a.getX(), a.getY()) &&
			isPointInLine(next, next.next, b.getX(), b.getY()) {
			return next.isNextEdgeFromPolygon
		}
		if isPointInLine(next, next.previous, a.getX(), a.getY()) &&
			isPointInLine(next, next.previous, b.getX(), b.getY()) {
			return next.previous.isNextEdgeFromPolygon
		}
		next = next.next
		if next == a {
			break
		}
	}
	return false
}

// isMortonEdgeFromPolygon uses z-order acceleration.
//
// Port of Tessellator.isMortonEdgeFromPolygon.
func isMortonEdgeFromPolygon(a, b *tessNode) bool {
	minTX := int64(uint32(min32(a.encX, b.encX)) ^ 0x80000000)
	minTY := int64(uint32(min32(a.encY, b.encY)) ^ 0x80000000)
	maxTX := int64(uint32(max32(a.encX, b.encX)) ^ 0x80000000)
	maxTY := int64(uint32(max32(a.encY, b.encY)) ^ 0x80000000)

	minZ := util.Interleave(int(minTX), int(minTY))
	maxZ := util.Interleave(int(maxTX), int(maxTY))

	p := a.previousZ
	n := a.nextZ
	for p != nil && uint64(p.morton) >= uint64(minZ) &&
		n != nil && uint64(n.morton) <= uint64(maxZ) {
		if isPointInLine(p, p.next, a.getX(), a.getY()) &&
			isPointInLine(p, p.next, b.getX(), b.getY()) {
			return p.isNextEdgeFromPolygon
		}
		if isPointInLine(p, p.previous, a.getX(), a.getY()) &&
			isPointInLine(p, p.previous, b.getX(), b.getY()) {
			return p.previous.isNextEdgeFromPolygon
		}
		p = p.previousZ
		if isPointInLine(n, n.next, a.getX(), a.getY()) &&
			isPointInLine(n, n.next, b.getX(), b.getY()) {
			return n.isNextEdgeFromPolygon
		}
		if isPointInLine(n, n.previous, a.getX(), a.getY()) &&
			isPointInLine(n, n.previous, b.getX(), b.getY()) {
			return n.previous.isNextEdgeFromPolygon
		}
		n = n.nextZ
	}
	for p != nil && uint64(p.morton) >= uint64(minZ) {
		if isPointInLine(p, p.next, a.getX(), a.getY()) &&
			isPointInLine(p, p.next, b.getX(), b.getY()) {
			return p.isNextEdgeFromPolygon
		}
		if isPointInLine(p, p.previous, a.getX(), a.getY()) &&
			isPointInLine(p, p.previous, b.getX(), b.getY()) {
			return p.previous.isNextEdgeFromPolygon
		}
		p = p.previousZ
	}
	for n != nil && uint64(n.morton) <= uint64(maxZ) {
		if isPointInLine(n, n.next, a.getX(), a.getY()) &&
			isPointInLine(n, n.next, b.getX(), b.getY()) {
			return n.isNextEdgeFromPolygon
		}
		if isPointInLine(n, n.previous, a.getX(), a.getY()) &&
			isPointInLine(n, n.previous, b.getX(), b.getY()) {
			return n.previous.isNextEdgeFromPolygon
		}
		n = n.nextZ
	}
	return false
}

// ─── linked-list helpers ──────────────────────────────────────────────────────

// splitPolygon bridges vertices a and b and returns the second copy of b.
//
// Port of Tessellator.splitPolygon.
func splitPolygon(a, b *tessNode, edgeFromPolygon bool) *tessNode {
	a2 := cloneNode(a)
	b2 := cloneNode(b)
	an := a.next
	bp := b.previous

	a.next = b
	a.isNextEdgeFromPolygon = edgeFromPolygon
	a.nextZ = b
	b.previous = a
	b.previousZ = a

	a2.next = an
	a2.nextZ = an
	an.previous = a2
	an.previousZ = a2

	b2.next = a2
	b2.isNextEdgeFromPolygon = edgeFromPolygon
	b2.nextZ = a2
	a2.previous = b2
	a2.previousZ = b2

	bp.next = b2
	bp.nextZ = b2

	return b2
}

// removeNode removes node from the doubly-linked list.
//
// Port of Tessellator.removeNode.
func removeNode(node *tessNode, edgeFromPolygon bool) {
	node.next.previous = node.previous
	node.previous.next = node.next
	node.previous.isNextEdgeFromPolygon = edgeFromPolygon

	if node.previousZ != nil {
		node.previousZ.nextZ = node.nextZ
	}
	if node.nextZ != nil {
		node.nextZ.previousZ = node.previousZ
	}
}

// filterPoints eliminates collinear/duplicate points from the doubly-linked
// list.
//
// Port of Tessellator.filterPoints.
func filterPoints(start, end *tessNode) *tessNode {
	if start == nil {
		return start
	}
	if end == nil {
		end = start
	}

	node := start
	for {
		nextNode := node.next
		prevNode := node.previous
		continueIteration := false

		if isVertexEqualsByCoord(node, nextNode) ||
			isVertexEqualsByCoord(prevNode, nextNode) ||
			((prevNode.isNextEdgeFromPolygon == node.isNextEdgeFromPolygon ||
				isPointInLine(prevNode, node, nextNode.getX(), nextNode.getY())) &&
				tessArea(prevNode.getX(), prevNode.getY(),
					node.getX(), node.getY(),
					nextNode.getX(), nextNode.getY()) == 0) {
			removeNode(node, prevNode.isNextEdgeFromPolygon)
			node = prevNode
			end = prevNode
			if node == nextNode {
				break
			}
			continueIteration = true
		} else {
			node = nextNode
		}
		if !continueIteration && node == end {
			break
		}
	}
	return end
}

// cureLocalIntersections removes small local self-intersections, emitting the
// resulting triangles.
//
// Port of Tessellator.cureLocalIntersections.
func cureLocalIntersections(startNode *tessNode, tessellation []Triangle, mortonOptimized bool) (*tessNode, []Triangle) {
	node := startNode
	for {
		nextNode := node.next
		a := node.previous
		b := nextNode.next

		if !isVertexEqualsByCoord(a, b) &&
			linesIntersectLocal(
				a.getX(), a.getY(), node.getX(), node.getY(),
				nextNode.getX(), nextNode.getY(), b.getX(), b.getY()) &&
			isLocallyInside(a, b) &&
			isLocallyInside(b, a) &&
			!isIntersectingPolygon(a, a.getX(), a.getY(), b.getX(), b.getY()) {

			abFromPolygon := a.isNextEdgeFromPolygon
			if a.next != node {
				abFromPolygon = isEdgeFromPolygon(a, node, mortonOptimized)
			}
			bcFromPolygon := node.isNextEdgeFromPolygon
			if node.next != b {
				bcFromPolygon = isEdgeFromPolygon(node, b, mortonOptimized)
			}
			caFromPolygon := b.isNextEdgeFromPolygon
			if b.next != a {
				caFromPolygon = isEdgeFromPolygon(a, b, mortonOptimized)
			}

			t := Triangle{
				ax: a.getX(), ay: a.getY(),
				bx: node.getX(), by: node.getY(),
				cx: b.getX(), cy: b.getY(),
				abFromPolygon: abFromPolygon,
				bcFromPolygon: bcFromPolygon,
				caFromPolygon: caFromPolygon,
			}
			tessellation = append(tessellation, t)
			tessellation = append(tessellation, t)

			removeNode(node, caFromPolygon)
			removeNode(node.next, caFromPolygon)
			node = b
			startNode = b
		}
		node = node.next
		if node == startNode {
			break
		}
	}
	return node, tessellation
}

// splitEarcut attempts to split the remaining polygon and earcut both halves.
//
// Port of Tessellator.splitEarcut.
func splitEarcut(start *tessNode, tessellation *[]Triangle, mortonOptimized bool, depth int) bool {
	searchNode := start
	for {
		nextNode := searchNode.next
		diagonal := nextNode.next
		for diagonal != searchNode.previous {
			if searchNode.idx != diagonal.idx && isValidDiagonal(searchNode, diagonal) {
				fromPoly := isEdgeFromPolygon(searchNode, diagonal, mortonOptimized)
				splitNode := splitPolygon(searchNode, diagonal, fromPoly)
				searchNode = filterPoints(searchNode, searchNode.next)
				splitNode = filterPoints(splitNode, splitNode.next)
				if mortonOptimized {
					sortByMortonWithReset(searchNode)
					sortByMortonWithReset(splitNode)
				}
				*tessellation = earcutLinkedList(searchNode, *tessellation, stateInit, mortonOptimized, depth)
				*tessellation = earcutLinkedList(splitNode, *tessellation, stateInit, mortonOptimized, depth)
				return true
			}
			diagonal = diagonal.next
		}
		searchNode = searchNode.next
		if searchNode == start {
			break
		}
	}
	return signedAreaNode(start, start) == 0
}

// ─── diagonal validity ────────────────────────────────────────────────────────

// isValidDiagonal reports whether the diagonal between a and b is valid.
//
// Port of Tessellator.isValidDiagonal.
func isValidDiagonal(a, b *tessNode) bool {
	if a.next.idx == b.idx || a.previous.idx == b.idx ||
		!isLocallyInside(a.previous, b) ||
		!isLocallyInside(b.next, a) ||
		!isCWPolygon(a, b) ||
		!isCWPolygon(b, a) {
		return false
	}
	if isVertexEqualsByCoord(a, b) {
		return true
	}
	return isLocallyInside(a, b) &&
		isLocallyInside(b, a) &&
		middleInsert(a, a.getX(), a.getY(), b.getX(), b.getY()) &&
		tessArea(a.previous.getX(), a.previous.getY(), a.getX(), a.getY(), b.getX(), b.getY()) != 0 &&
		tessArea(a.getX(), a.getY(), b.getX(), b.getY(), b.next.getX(), b.next.getY()) != 0 &&
		tessArea(a.next.getX(), a.next.getY(), a.getX(), a.getY(), b.getX(), b.getY()) != 0 &&
		tessArea(a.getX(), a.getY(), b.getX(), b.getY(), b.previous.getX(), b.previous.getY()) != 0 &&
		!isIntersectingPolygon(a, a.getX(), a.getY(), b.getX(), b.getY())
}

func isCWPolygon(start, end *tessNode) bool {
	return signedAreaNode(start, end) < 0
}

func signedAreaNode(start, end *tessNode) float64 {
	next := start
	var windingSum float64
	for {
		windingSum += tessArea(next.getX(), next.getY(), next.next.getX(), next.next.getY(), end.getX(), end.getY())
		next = next.next
		if next.next == end {
			break
		}
	}
	return windingSum
}

func isLocallyInside(a, b *tessNode) bool {
	area := tessArea(a.previous.getX(), a.previous.getY(), a.getX(), a.getY(), a.next.getX(), a.next.getY())
	if area == 0 {
		return false
	}
	if area < 0 {
		// a is CW
		return tessArea(a.getX(), a.getY(), b.getX(), b.getY(), a.next.getX(), a.next.getY()) >= 0 &&
			tessArea(a.getX(), a.getY(), a.previous.getX(), a.previous.getY(), b.getX(), b.getY()) >= 0
	}
	// CCW
	return tessArea(a.getX(), a.getY(), b.getX(), b.getY(), a.previous.getX(), a.previous.getY()) < 0 ||
		tessArea(a.getX(), a.getY(), a.next.getX(), a.next.getY(), b.getX(), b.getY()) < 0
}

func middleInsert(start *tessNode, x0, y0, x1, y1 float64) bool {
	node := start
	nextNode := node.next
	lIsInside := false
	lDx := (x0 + x1) / 2.0
	lDy := (y0 + y1) / 2.0
	for {
		nextNode = node.next
		if (node.getY() > lDy) != (nextNode.getY() > lDy) &&
			lDx < (nextNode.getX()-node.getX())*(lDy-node.getY())/(nextNode.getY()-node.getY())+node.getX() {
			lIsInside = !lIsInside
		}
		node = node.next
		if node == start {
			break
		}
	}
	return lIsInside
}

func isIntersectingPolygon(start *tessNode, x0, y0, x1, y1 float64) bool {
	node := start
	for {
		nextNode := node.next
		if !isVertexEquals(node, x0, y0) && !isVertexEquals(node, x1, y1) {
			if linesIntersectLocal(node.getX(), node.getY(), nextNode.getX(), nextNode.getY(), x0, y0, x1, y1) {
				return true
			}
		}
		node = nextNode
		if node == start {
			break
		}
	}
	return false
}

// ─── Morton sort ──────────────────────────────────────────────────────────────

// sortByMorton interlinks polygon nodes in z-order for acceleration.
//
// Port of Tessellator.sortByMorton.
func sortByMorton(start *tessNode) {
	start.previousZ.nextZ = nil
	start.previousZ = nil
	tathamSort(start)
}

// sortByMortonWithReset resets z-pointers then sorts.
func sortByMortonWithReset(start *tessNode) {
	next := start
	for {
		next.previousZ = next.previous
		next.nextZ = next.next
		next = next.next
		if next == start {
			break
		}
	}
	sortByMorton(start)
}

// tathamSort is Simon Tatham's O(n log n) doubly-linked list merge-sort.
//
// Port of Tessellator.tathamSort.
func tathamSort(list *tessNode) {
	if list == nil {
		return
	}
	inSize := 1
	for {
		p := list
		list = nil
		var tail *tessNode
		numMerges := 0

		for p != nil {
			numMerges++
			q := p
			pSize := 0
			for i := 0; i < inSize && q != nil; i++ {
				pSize++
				q = q.nextZ
			}
			qSize := inSize

			for pSize > 0 || (qSize > 0 && q != nil) {
				var e *tessNode
				if pSize != 0 && (qSize == 0 || q == nil || uint64(p.morton) <= uint64(q.morton)) {
					e = p
					p = p.nextZ
					pSize--
				} else {
					e = q
					q = q.nextZ
					qSize--
				}
				if tail != nil {
					tail.nextZ = e
				} else {
					list = e
				}
				e.previousZ = tail
				tail = e
			}
			p = q
		}

		tail.nextZ = nil
		inSize *= 2
		if numMerges <= 1 {
			break
		}
	}
}

// ─── geometry helpers ─────────────────────────────────────────────────────────

// tessArea returns (bY-aY)*(cX-bX) - (bX-aX)*(cY-bY), which is positive for
// a reflex angle and negative for a convex angle.  This is the Lucene sign
// convention, opposite to the standard shoelace sign.
func tessArea(ax, ay, bx, by, cx, cy float64) float64 {
	return (by-ay)*(cx-bx) - (bx-ax)*(cy-by)
}

// pointInEar returns true when (x, y) lies inside or on the boundary of the
// triangle (ax,ay)–(bx,by)–(cx,cy).
func pointInEar(x, y, ax, ay, bx, by, cx, cy float64) bool {
	return (cx-x)*(ay-y)-(ax-x)*(cy-y) >= 0 &&
		(ax-x)*(by-y)-(bx-x)*(ay-y) >= 0 &&
		(bx-x)*(cy-y)-(cx-x)*(by-y) >= 0
}

// linesIntersectLocal is the local variant using the tessArea convention
// (avoids depending on GeoUtils.lineCrossesLine sign convention).
func linesIntersectLocal(aX0, aY0, aX1, aY1, bX0, bY0, bX1, bY1 float64) bool {
	return (tessArea(aX0, aY0, aX1, aY1, bX0, bY0) > 0) != (tessArea(aX0, aY0, aX1, aY1, bX1, bY1) > 0) &&
		(tessArea(bX0, bY0, bX1, bY1, aX0, aY0) > 0) != (tessArea(bX0, bY0, bX1, bY1, aX1, aY1) > 0)
}

// isPointInLine returns true when (lon, lat) is collinear with a–b and lies
// within the segment.
func isPointInLine(a, b *tessNode, lon, lat float64) bool {
	dxc := lon - a.getX()
	dyc := lat - a.getY()
	dxl := b.getX() - a.getX()
	dyl := b.getY() - a.getY()
	if dxc*dyl-dyc*dxl == 0 {
		if abs64(dxl) >= abs64(dyl) {
			if dxl > 0 {
				return a.getX() <= lon && lon <= b.getX()
			}
			return b.getX() <= lon && lon <= a.getX()
		}
		if dyl > 0 {
			return a.getY() <= lat && lat <= b.getY()
		}
		return b.getY() <= lat && lat <= a.getY()
	}
	return false
}

func isVertexEqualsByCoord(a, b *tessNode) bool {
	return a.getX() == b.getX() && a.getY() == b.getY()
}

func isVertexEquals(a *tessNode, x, y float64) bool {
	return a.getX() == x && a.getY() == y
}

func tessAngle(a, b, c *tessNode) float64 {
	ax := a.getX() - b.getX()
	ay := a.getY() - b.getY()
	cx := c.getX() - b.getX()
	cy := c.getY() - b.getY()
	dot := ax*cx + ay*cy
	aLen := mathSqrt(ax*ax + ay*ay)
	bLen := mathSqrt(cx*cx + cy*cy)
	return mathAcos(dot / (aLen * bLen))
}

// ─── numeric helpers ──────────────────────────────────────────────────────────

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func float32SliceToFloat64(in []float32) []float64 {
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = float64(v)
	}
	return out
}

// mathSqrt and mathAcos are thin wrappers so the math import is localised.
func mathSqrt(x float64) float64 {
	// Use a simple Newton's method to avoid importing math.
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 50; i++ {
		z1 := (z + x/z) / 2
		if abs64(z1-z) < 1e-15 {
			return z1
		}
		z = z1
	}
	return z
}

func mathAcos(x float64) float64 {
	// Clamp to [-1,1] to guard against floating-point rounding.
	if x > 1 {
		x = 1
	} else if x < -1 {
		x = -1
	}
	// Halley's method approximation for acos via atan2.
	// We use the identity acos(x) = atan2(sqrt(1-x*x), x).
	y := mathSqrt(1 - x*x)
	return mathAtan2(y, x)
}

func mathAtan2(y, x float64) float64 {
	if x > 0 {
		return mathAtan(y / x)
	} else if x < 0 {
		if y >= 0 {
			return mathAtan(y/x) + 3.141592653589793
		}
		return mathAtan(y/x) - 3.141592653589793
	} else {
		if y > 0 {
			return 1.5707963267948966 // pi/2
		} else if y < 0 {
			return -1.5707963267948966
		}
		return 0
	}
}

func mathAtan(x float64) float64 {
	// Minimax polynomial approximation for atan on [-1,1].
	// For |x|>1 we use atan(x) = pi/2 - atan(1/x).
	sign := 1.0
	if x < 0 {
		sign = -1
		x = -x
	}
	reduced := false
	if x > 1 {
		x = 1 / x
		reduced = true
	}
	// 7-term minimax approximation (max error ~2e-7).
	x2 := x * x
	r := x * (1.0 - x2*(1.0/3.0-x2*(1.0/5.0-x2*(1.0/7.0-x2*(1.0/9.0-x2*(1.0/11.0-x2/13.0))))))
	if reduced {
		r = 1.5707963267948966 - r
	}
	return sign * r
}

// rawWindingOrder computes the winding order of a raw polygon ring using the
// same formula as Lucene's Polygon and XYPolygon constructors:
//
//	sum += (x[j]-x[anchor])*(y[i]-y[anchor]) - (y[j]-y[anchor])*(x[i]-x[anchor])
//
// where anchor = last vertex (numPts = len-1), excluding the closing vertex.
// sum >= 0 → WindingClockwise; sum < 0 → WindingCounterClockwise.
func rawWindingOrder(xs, ys []float64) WindingOrder {
	numPts := len(xs) - 1
	if numPts <= 0 {
		return WindingClockwise
	}
	anchorX := xs[numPts]
	anchorY := ys[numPts]
	var sum float64
	for i, j := 1, 0; i < numPts; j, i = i, i+1 {
		sum += (xs[j]-anchorX)*(ys[i]-anchorY) - (ys[j]-anchorY)*(xs[i]-anchorX)
	}
	if sum < 0 {
		return WindingCounterClockwise
	}
	return WindingClockwise
}
