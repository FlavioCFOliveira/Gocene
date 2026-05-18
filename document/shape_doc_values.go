// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ShapeDocValues is the Go port of Lucene 10.4.0's
// org.apache.lucene.document.ShapeDocValues. It is the binary
// doc-values format shared by the geographic (LatLonShape) and
// Cartesian (XYShape) variants.
//
// The Java reference is a package-private abstract class with
// subclasses LatLonShapeDocValues and XYShapeDocValues. The Go port
// replaces the abstract-method hooks with a ShapeDocValuesEncoder
// strategy injected at construction time plus two closures for the
// centroid / bounding-box derivations (template-method analogue). The
// concrete subclass ports — LatLonShapeDocValues and XYShapeDocValues
// — live in separate tasks (GOC-3218 and GOC-4532) and are expected
// to layer over this base.
//
// ShapeDocValues does not support multi-geometries, mirroring the
// Java reference: a single shape's centroid depends on the area of
// the underlying geometry and aggregating multiple shapes would
// require a non-trivial change to the binary doc-values storage.
//
// The class is read-only after construction; concurrent calls to
// Relate are NOT safe, because the underlying ByteArrayDataInput
// position is mutated. Callers that need concurrent reads must use a
// per-goroutine ShapeDocValues built from the same serialized bytes
// (the data is cheaply share-able via util.BytesRef).
type ShapeDocValues struct {
	// data is the serialized binary doc-value payload (DFS-encoded
	// tree of tessellated triangles).
	data *util.BytesRef

	// comparator parses the header up front and provides the
	// tree-traversal Relate entry point.
	comparator *shapeComparator

	// centroid is computed by computeCentroidFn at construction.
	centroid geo.Geometry

	// boundingBox is computed by computeBoundingBoxFn at
	// construction.
	boundingBox geo.Geometry
}

// shapeDocValuesVersion is the on-disk encoding version. Mirrors the
// Java constant ShapeDocValues.VERSION = 0.
const shapeDocValuesVersion byte = 0

// ShapeDocValuesEncoder is the Go port of the Java protected nested
// interface ShapeDocValues.Encoder. Subclasses (LatLonShapeDocValues
// and XYShapeDocValues) supply concrete encoders that map between
// double-precision world coordinates and the int32 encoded values
// used by the BKD tree.
type ShapeDocValuesEncoder interface {
	// EncodeX maps a world-space X coordinate to its int32 encoded
	// representation. Mirrors Encoder.encodeX.
	EncodeX(x float64) int32

	// EncodeY maps a world-space Y coordinate to its int32 encoded
	// representation. Mirrors Encoder.encodeY.
	EncodeY(y float64) int32

	// DecodeX maps an encoded int32 value back to world-space X.
	// Mirrors Encoder.decodeX.
	DecodeX(encoded int32) float64

	// DecodeY maps an encoded int32 value back to world-space Y.
	// Mirrors Encoder.decodeY.
	DecodeY(encoded int32) float64
}

// ShapeDocValuesGeometryFunc is the strategy callback for the
// computeCentroid / computeBoundingBox hooks. The function receives
// the freshly built ShapeDocValues so the subclass can read
// EncodedCentroidX / EncodedBoundingBox accessors and project them
// back through its own encoder into a concrete geo.Geometry.
//
// Mirrors the Java abstract methods
// ShapeDocValues.computeCentroid()/computeBoundingBox().
type ShapeDocValuesGeometryFunc func(sdv *ShapeDocValues) geo.Geometry

// NewShapeDocValuesFromTessellation builds a ShapeDocValues from a
// tessellation: a list of DecodedTriangle entries (with
// already-encoded vertices). Mirrors the Java constructor
// ShapeDocValues(List<DecodedTriangle>).
//
// The encoder is used to project decoded vertices back to world
// space for the per-component centroid arithmetic. The
// computeCentroidFn / computeBoundingBoxFn callbacks are invoked
// after the tessellation is serialized and the comparator is built,
// so they may freely call GetEncodedCentroidX / GetEncodedMinX etc.
func NewShapeDocValuesFromTessellation(
	encoder ShapeDocValuesEncoder,
	tessellation []DecodedTriangle,
	computeCentroidFn ShapeDocValuesGeometryFunc,
	computeBoundingBoxFn ShapeDocValuesGeometryFunc,
) (*ShapeDocValues, error) {
	if encoder == nil {
		return nil, errors.New("ShapeDocValues: encoder must not be nil")
	}
	if computeCentroidFn == nil || computeBoundingBoxFn == nil {
		return nil, errors.New("ShapeDocValues: centroid/boundingBox hooks must not be nil")
	}
	if len(tessellation) == 0 {
		return nil, errors.New("ShapeDocValues: tessellation must not be empty")
	}
	data, err := computeBinaryValue(encoder, tessellation)
	if err != nil {
		return nil, fmt.Errorf("ShapeDocValues: encode tessellation: %w", err)
	}
	return finalizeShapeDocValues(encoder, data, computeCentroidFn, computeBoundingBoxFn)
}

// NewShapeDocValuesFromBinary builds a ShapeDocValues from the
// already-serialized binary value. Mirrors the Java constructor
// ShapeDocValues(BytesRef).
func NewShapeDocValuesFromBinary(
	encoder ShapeDocValuesEncoder,
	binaryValue *util.BytesRef,
	computeCentroidFn ShapeDocValuesGeometryFunc,
	computeBoundingBoxFn ShapeDocValuesGeometryFunc,
) (*ShapeDocValues, error) {
	if encoder == nil {
		return nil, errors.New("ShapeDocValues: encoder must not be nil")
	}
	if computeCentroidFn == nil || computeBoundingBoxFn == nil {
		return nil, errors.New("ShapeDocValues: centroid/boundingBox hooks must not be nil")
	}
	if binaryValue == nil {
		return nil, errors.New("ShapeDocValues: binaryValue must not be nil")
	}
	return finalizeShapeDocValues(encoder, binaryValue, computeCentroidFn, computeBoundingBoxFn)
}

func finalizeShapeDocValues(
	encoder ShapeDocValuesEncoder,
	data *util.BytesRef,
	computeCentroidFn ShapeDocValuesGeometryFunc,
	computeBoundingBoxFn ShapeDocValuesGeometryFunc,
) (*ShapeDocValues, error) {
	comparator, err := newShapeComparator(encoder, data)
	if err != nil {
		return nil, fmt.Errorf("ShapeDocValues: unable to read binary shape doc value field: %w", err)
	}
	sdv := &ShapeDocValues{
		data:       data,
		comparator: comparator,
	}
	sdv.centroid = computeCentroidFn(sdv)
	sdv.boundingBox = computeBoundingBoxFn(sdv)
	return sdv, nil
}

// BinaryValue returns the serialized binary doc-values payload.
// Mirrors the Java protected method ShapeDocValues.binaryValue().
func (s *ShapeDocValues) BinaryValue() *util.BytesRef { return s.data }

// NumberOfTerms returns the number of tessellated triangles encoded
// in this shape. Mirrors ShapeDocValues.numberOfTerms.
func (s *ShapeDocValues) NumberOfTerms() int { return s.comparator.numberOfTerms }

// GetEncodedMinX returns the int32 minimum X (longitude / cartesian
// x) of the shape's bounding box, in encoded space.
func (s *ShapeDocValues) GetEncodedMinX() int32 { return s.comparator.boundingBox.minX }

// GetEncodedMinY returns the int32 minimum Y of the shape's
// bounding box, in encoded space.
func (s *ShapeDocValues) GetEncodedMinY() int32 { return s.comparator.boundingBox.minY }

// GetEncodedMaxX returns the int32 maximum X of the shape's
// bounding box, in encoded space.
func (s *ShapeDocValues) GetEncodedMaxX() int32 { return s.comparator.boundingBox.maxX }

// GetEncodedMaxY returns the int32 maximum Y of the shape's
// bounding box, in encoded space.
func (s *ShapeDocValues) GetEncodedMaxY() int32 { return s.comparator.boundingBox.maxY }

// GetEncodedCentroidX returns the int32 X centroid in encoded
// space. Mirrors the Java protected method getEncodedCentroidX.
func (s *ShapeDocValues) GetEncodedCentroidX() int32 { return s.comparator.centroidX }

// GetEncodedCentroidY returns the int32 Y centroid in encoded
// space. Mirrors getEncodedCentroidY.
func (s *ShapeDocValues) GetEncodedCentroidY() int32 { return s.comparator.centroidY }

// GetHighestDimension returns the highest-dimensional decoded
// triangle type (POINT, LINE, or TRIANGLE) present in the shape.
// Mirrors ShapeDocValues.getHighestDimension.
func (s *ShapeDocValues) GetHighestDimension() DecodedTriangleType {
	return s.comparator.highestDimension
}

// GetCentroid returns the centroid geometry computed by the
// subclass-specific hook supplied at construction.
func (s *ShapeDocValues) GetCentroid() geo.Geometry { return s.centroid }

// GetBoundingBox returns the bounding-box geometry computed by the
// subclass-specific hook supplied at construction.
func (s *ShapeDocValues) GetBoundingBox() geo.Geometry { return s.boundingBox }

// Relate returns the spatial relationship between the supplied
// Component2D query and the encoded shape. Mirrors
// ShapeDocValues.relate(Component2D).
func (s *ShapeDocValues) Relate(component geo.Component2D) (geo.Relation, error) {
	return s.comparator.relate(component)
}

// =========================================================================
// Internal: tree node, serialization, deserialization, traversal.
// =========================================================================

// shapeTreeNode is the in-memory tessellation tree node. Mirrors
// the Java nested final class ShapeDocValues.TreeNode.
type shapeTreeNode struct {
	triangle DecodedTriangle

	// midX / midY accumulate the centroid running stats in encoded
	// space (NOT in geographical space — Java explicitly warns
	// against re-purposing).
	midX float64
	midY float64

	// signedArea is non-zero iff this node is a triangle. Always
	// positive (CCW invariant).
	signedArea float64

	// length is non-zero iff this node is a line.
	length float64

	highestType DecodedTriangleType

	minX, maxX, minY, maxY int32

	left   *shapeTreeNode
	right  *shapeTreeNode
	parent *shapeTreeNode

	// byteSize is the encoded size of the subtree rooted at this
	// node. Accumulated during construction. Starts at 1 for the
	// header byte.
	byteSize int
}

func newShapeTreeNode(encoder ShapeDocValuesEncoder, t DecodedTriangle) *shapeTreeNode {
	n := &shapeTreeNode{
		triangle: t,
		minX:     minInt32x3(t.AX, t.BX, t.CX),
		minY:     minInt32x3(t.AY, t.BY, t.CY),
		maxX:     maxInt32x3(t.AX, t.BX, t.CX),
		maxY:     maxInt32x3(t.AY, t.BY, t.CY),
		byteSize: 1,
	}
	ax := encoder.DecodeX(t.AX)
	ay := encoder.DecodeY(t.AY)
	switch t.Kind {
	case DecodedTriangleTypePoint:
		n.midX = ax
		n.midY = ay
	case DecodedTriangleTypeLine:
		bx := encoder.DecodeX(t.BX)
		by := encoder.DecodeY(t.BY)
		n.length = math.Hypot(ax-bx, ay-by)
		// Weight the midpoint by the segment length, matching the
		// straight-line centroid formula used by Lucene.
		n.midX = 0.5 * (ax + bx) * n.length
		n.midY = 0.5 * (ay + by) * n.length
	case DecodedTriangleTypeTriangle:
		bx := encoder.DecodeX(t.BX)
		by := encoder.DecodeY(t.BY)
		cx := encoder.DecodeX(t.CX)
		cy := encoder.DecodeY(t.CY)
		n.signedArea = math.Abs(0.5 * ((bx-ax)*(cy-ay) - (cx-ax)*(by-ay)))
		// Weight by signed area, matching the triangle centroid
		// formula in Lucene (graphics-FAQ reference).
		n.midX = ((ax + bx + cx) / 3.0) * n.signedArea
		n.midY = ((ay + by + cy) / 3.0) * n.signedArea
	default:
		// Defensive: Kind is an enum but we do not panic; the
		// caller validated input already.
		n.midX = ax
		n.midY = ay
	}
	return n
}

// computeBinaryValue is the Go port of ShapeDocValues.computeBinaryValue.
func computeBinaryValue(encoder ShapeDocValuesEncoder, tessellation []DecodedTriangle) (*util.BytesRef, error) {
	dfs := make([]*shapeTreeNode, 0, len(tessellation))
	if _, err := buildShapeTree(encoder, tessellation, &dfs); err != nil {
		return nil, err
	}
	out := store.NewByteBuffersDataOutputWithSize(int64(len(tessellation) * 16))
	if err := writeShapeTree(encoder, out, &dfs); err != nil {
		return nil, err
	}
	buf := out.ToArrayCopy()
	return &util.BytesRef{Bytes: buf, Offset: 0, Length: len(buf)}, nil
}

// buildShapeTree mirrors ShapeDocValues.buildTree.
func buildShapeTree(
	encoder ShapeDocValuesEncoder,
	tessellation []DecodedTriangle,
	dfsSerialized *[]*shapeTreeNode,
) (*shapeTreeNode, error) {
	if len(tessellation) == 1 {
		t := tessellation[0]
		node := newShapeTreeNode(encoder, t)
		if t.Kind == DecodedTriangleTypeLine {
			if node.length != 0 {
				node.midX /= node.length
				node.midY /= node.length
			}
		} else if t.Kind == DecodedTriangleTypeTriangle {
			if node.signedArea != 0 {
				node.midX /= node.signedArea
				node.midY /= node.signedArea
			}
		}
		node.highestType = t.Kind
		*dfsSerialized = append(*dfsSerialized, node)
		return node, nil
	}
	triangles := make([]*shapeTreeNode, len(tessellation))
	minY := int32(math.MaxInt32)
	minX := int32(math.MaxInt32)
	maxY := int32(math.MinInt32)
	maxX := int32(math.MinInt32)

	var (
		totalSignedArea float64
		totalLength     float64
		numXPnt         float64
		numYPnt         float64
		numXLin         float64
		numYLin         float64
		numXPly         float64
		numYPly         float64
	)
	highestType := DecodedTriangleTypePoint

	for i, t := range tessellation {
		node := newShapeTreeNode(encoder, t)
		triangles[i] = node
		minY = minInt32(minY, node.minY)
		minX = minInt32(minX, node.minX)
		maxY = maxInt32(maxY, node.maxY)
		maxX = maxInt32(maxX, node.maxX)

		totalSignedArea += node.signedArea
		totalLength += node.length
		switch t.Kind {
		case DecodedTriangleTypePoint:
			numXPnt += node.midX
			numYPnt += node.midY
		case DecodedTriangleTypeLine:
			if highestType == DecodedTriangleTypePoint {
				highestType = DecodedTriangleTypeLine
			}
			numXLin += node.midX
			numYLin += node.midY
		case DecodedTriangleTypeTriangle:
			if highestType != DecodedTriangleTypeTriangle {
				highestType = DecodedTriangleTypeTriangle
			}
			numXPly += node.midX
			numYPly += node.midY
		}
	}

	root := createShapeTree(triangles, 0, len(triangles)-1, false, nil, dfsSerialized)
	if root == nil {
		return nil, errors.New("ShapeDocValues: empty tree from non-empty tessellation")
	}
	// Pull up min values so the root bbox is consistent with the
	// pass above; createShapeTree only pulls maxX/maxY upward.
	root.minY = minY
	root.minX = minX

	root.highestType = highestType

	// Compute centroid values for the root so the centroid is
	// consistent. Mirrors the corresponding block in the Java
	// reference.
	n := float64(len(tessellation))
	switch highestType {
	case DecodedTriangleTypePoint:
		root.midX = numXPnt / n
		root.midY = numYPnt / n
	case DecodedTriangleTypeLine:
		root.midX = numXLin
		root.midY = numYLin
		if totalLength != 0 {
			root.midX /= totalLength
			root.midY /= totalLength
		}
	default:
		root.midX = numXPly
		root.midY = numYPly
		if totalSignedArea != 0 {
			root.midX /= totalSignedArea
			root.midY /= totalSignedArea
		}
	}

	return root, nil
}

// createShapeTree mirrors ShapeDocValues.createTree. The split
// alternates between X and Y dimensions; the partition uses
// util.Select (introselect) to place the median element at position
// `mid` without fully sorting the slice.
func createShapeTree(
	triangles []*shapeTreeNode,
	low, high int,
	splitX bool,
	parent *shapeTreeNode,
	dfsSerialized *[]*shapeTreeNode,
) *shapeTreeNode {
	if low > high {
		return nil
	}
	// Match Java's (low + high) >>> 1 (logical right shift); since
	// indices are non-negative the arithmetic right shift is
	// equivalent.
	mid := (low + high) >> 1
	if low < high {
		var cmp func(a, b *shapeTreeNode) int
		if splitX {
			cmp = func(a, b *shapeTreeNode) int {
				if a.minX != b.minX {
					return int(a.minX) - int(b.minX)
				}
				return int(a.maxX) - int(b.maxX)
			}
		} else {
			cmp = func(a, b *shapeTreeNode) int {
				if a.minY != b.minY {
					return int(a.minY) - int(b.minY)
				}
				return int(a.maxY) - int(b.maxY)
			}
		}
		util.Select(triangles, low, high+1, mid, cmp)
	}
	newNode := triangles[mid]
	*dfsSerialized = append(*dfsSerialized, newNode)
	newNode.parent = parent

	newNode.left = createShapeTree(triangles, low, mid-1, !splitX, newNode, dfsSerialized)
	newNode.right = createShapeTree(triangles, mid+1, high, !splitX, newNode, dfsSerialized)

	if newNode.left != nil {
		newNode.minX = minInt32(newNode.minX, newNode.left.minX)
		newNode.minY = minInt32(newNode.minY, newNode.left.minY)
		newNode.maxX = maxInt32(newNode.maxX, newNode.left.maxX)
		newNode.maxY = maxInt32(newNode.maxY, newNode.left.maxY)
	}
	if newNode.right != nil {
		newNode.minX = minInt32(newNode.minX, newNode.right.minX)
		newNode.minY = minInt32(newNode.minY, newNode.right.minY)
		newNode.maxX = maxInt32(newNode.maxX, newNode.right.maxX)
		newNode.maxY = maxInt32(newNode.maxY, newNode.right.maxY)
	}

	if newNode.left != nil {
		newNode.left.byteSize += VLongSize(int64(newNode.maxX) - int64(newNode.left.minX))
		newNode.left.byteSize += VLongSize(int64(newNode.maxY) - int64(newNode.left.minY))
		newNode.left.byteSize += VLongSize(int64(newNode.maxX) - int64(newNode.left.maxX))
		newNode.left.byteSize += VLongSize(int64(newNode.maxY) - int64(newNode.left.maxY))
		newNode.left.byteSize += computeShapeComponentSize(newNode.left, newNode.maxX, newNode.maxY)
		newNode.byteSize += VIntSize(int32(newNode.left.byteSize)) + newNode.left.byteSize
	}
	if newNode.right != nil {
		newNode.right.byteSize += VLongSize(int64(newNode.maxX) - int64(newNode.right.minX))
		newNode.right.byteSize += VLongSize(int64(newNode.maxY) - int64(newNode.right.minY))
		newNode.right.byteSize += VLongSize(int64(newNode.maxX) - int64(newNode.right.maxX))
		newNode.right.byteSize += VLongSize(int64(newNode.maxY) - int64(newNode.right.maxY))
		newNode.right.byteSize += computeShapeComponentSize(newNode.right, newNode.maxX, newNode.maxY)
		newNode.byteSize += VIntSize(int32(newNode.right.byteSize)) + newNode.right.byteSize
	}
	return newNode
}

func computeShapeComponentSize(node *shapeTreeNode, maxX, maxY int32) int {
	size := 0
	t := node.triangle
	size += VLongSize(int64(maxX) - int64(t.AX))
	size += VLongSize(int64(maxY) - int64(t.AY))
	if t.Kind == DecodedTriangleTypeLine || t.Kind == DecodedTriangleTypeTriangle {
		size += VLongSize(int64(maxX) - int64(t.BX))
		size += VLongSize(int64(maxY) - int64(t.BY))
	}
	if t.Kind == DecodedTriangleTypeTriangle {
		size += VLongSize(int64(maxX) - int64(t.CX))
		size += VLongSize(int64(maxY) - int64(t.CY))
	}
	return size
}

// writeShapeTree mirrors ShapeDocValues.Writer.writeTree.
func writeShapeTree(
	encoder ShapeDocValuesEncoder,
	out *store.ByteBuffersDataOutput,
	dfsSerialized *[]*shapeTreeNode,
) error {
	if len(*dfsSerialized) == 0 {
		return errors.New("ShapeDocValues: empty serialization list")
	}
	if err := out.WriteByte(shapeDocValuesVersion); err != nil {
		return err
	}
	if err := out.WriteVInt(int32(len(*dfsSerialized))); err != nil {
		return err
	}
	root := (*dfsSerialized)[0]
	*dfsSerialized = (*dfsSerialized)[1:]

	if err := out.WriteVLong(int64(root.minX) - int64(math.MinInt32)); err != nil {
		return err
	}
	if err := out.WriteVLong(int64(root.maxX) - int64(math.MinInt32)); err != nil {
		return err
	}
	if err := out.WriteVLong(int64(root.minY) - int64(math.MinInt32)); err != nil {
		return err
	}
	if err := out.WriteVLong(int64(root.maxY) - int64(math.MinInt32)); err != nil {
		return err
	}
	if err := out.WriteVLong(int64(encoder.EncodeX(root.midX)) - int64(math.MinInt32)); err != nil {
		return err
	}
	if err := out.WriteVLong(int64(encoder.EncodeY(root.midY)) - int64(math.MinInt32)); err != nil {
		return err
	}
	if err := out.WriteVInt(int32(root.highestType)); err != nil {
		return err
	}
	if err := writeShapeHeader(out, root); err != nil {
		return err
	}
	if err := writeShapeComponent(out, root, root.maxX, root.maxY); err != nil {
		return err
	}

	for _, n := range *dfsSerialized {
		if err := writeShapeNode(out, n); err != nil {
			return err
		}
	}
	return nil
}

func writeShapeNode(out *store.ByteBuffersDataOutput, node *shapeTreeNode) error {
	if err := out.WriteVInt(int32(node.byteSize)); err != nil {
		return err
	}
	if err := writeShapeBounds(out, node); err != nil {
		return err
	}
	if err := writeShapeHeader(out, node); err != nil {
		return err
	}
	if err := writeShapeComponent(out, node, node.parent.maxX, node.parent.maxY); err != nil {
		return err
	}
	return nil
}

func writeShapeComponent(out *store.ByteBuffersDataOutput, node *shapeTreeNode, pMaxX, pMaxY int32) error {
	t := node.triangle
	if err := out.WriteVLong(int64(pMaxX) - int64(t.AX)); err != nil {
		return err
	}
	if err := out.WriteVLong(int64(pMaxY) - int64(t.AY)); err != nil {
		return err
	}
	if t.Kind == DecodedTriangleTypeLine || t.Kind == DecodedTriangleTypeTriangle {
		if err := out.WriteVLong(int64(pMaxX) - int64(t.BX)); err != nil {
			return err
		}
		if err := out.WriteVLong(int64(pMaxY) - int64(t.BY)); err != nil {
			return err
		}
	}
	if t.Kind == DecodedTriangleTypeTriangle {
		if err := out.WriteVLong(int64(pMaxX) - int64(t.CX)); err != nil {
			return err
		}
		if err := out.WriteVLong(int64(pMaxY) - int64(t.CY)); err != nil {
			return err
		}
	}
	return nil
}

// writeShapeHeader mirrors the bit-layout of Writer.writeHeader.
// The header byte is written as a vInt to mirror the Java reference
// — single-byte values < 128 occupy exactly one byte under vInt
// encoding, so the on-disk size is unchanged.
func writeShapeHeader(out *store.ByteBuffersDataOutput, node *shapeTreeNode) error {
	header := int32(0)
	if node.right != nil {
		header |= 0x01
	}
	if node.left != nil {
		header |= 0x02
	}
	if node.triangle.Kind == DecodedTriangleTypePoint {
		header |= 0x04
	} else if node.triangle.Kind == DecodedTriangleTypeLine {
		header |= 0x08
	}
	if node.triangle.AB {
		header |= 0x10
	}
	if node.triangle.BC {
		header |= 0x20
	}
	if node.triangle.CA {
		header |= 0x40
	}
	return out.WriteVInt(header)
}

func writeShapeBounds(out *store.ByteBuffersDataOutput, node *shapeTreeNode) error {
	if err := out.WriteVLong(int64(node.parent.maxX) - int64(node.minX)); err != nil {
		return err
	}
	if err := out.WriteVLong(int64(node.parent.maxY) - int64(node.minY)); err != nil {
		return err
	}
	if err := out.WriteVLong(int64(node.parent.maxX) - int64(node.maxX)); err != nil {
		return err
	}
	if err := out.WriteVLong(int64(node.parent.maxY) - int64(node.maxY)); err != nil {
		return err
	}
	return nil
}

// =========================================================================
// Reader / comparator: parse the header and run the tree traversal.
// =========================================================================

// shapeBBox is the local mirror of search.EncodedRectangle used as
// scratch by the reader. It exposes mutable fields so the tree
// traversal can reset the box per-node without allocating.
//
// This duplicates a subset of search.EncodedRectangle's contract;
// the canonical type lives in search/encoded_rectangle.go (GOC-4531)
// but is read-only after construction. The duplication is bounded
// to the four bbox fields actually needed by the relate() loop and
// is therefore preferred over a structural refactor that would move
// EncodedRectangle into a lower-level package; see the sprint-55
// task notes for the rationale.
type shapeBBox struct {
	minX, maxX, minY, maxY int32
}

func (b *shapeBBox) reset(minX, maxX, minY, maxY int32) {
	b.minX = minX
	b.maxX = maxX
	b.minY = minY
	b.maxY = maxY
}

// shapeReader wraps a ByteArrayDataInput plus a scratch bbox.
// Mirrors the Java private final class Reader.
type shapeReader struct {
	in    *store.ByteArrayDataInput
	bbox  shapeBBox
	bytes []byte
}

func newShapeReader(binaryValue *util.BytesRef) *shapeReader {
	// ByteArrayDataInput operates on the raw bytes; we honour the
	// BytesRef offset by slicing.
	slice := binaryValue.Bytes[binaryValue.Offset : binaryValue.Offset+binaryValue.Length]
	return &shapeReader{
		in:    store.NewByteArrayDataInput(slice),
		bytes: slice,
		bbox: shapeBBox{
			minX: math.MaxInt32, maxX: -math.MaxInt32,
			minY: math.MaxInt32, maxY: -math.MaxInt32,
		},
	}
}

func (r *shapeReader) rewind() { _ = r.in.SetPosition(0) }

func (r *shapeReader) readBBox() (*shapeBBox, error) {
	minX, err := r.in.ReadVLong()
	if err != nil {
		return nil, err
	}
	maxX, err := r.in.ReadVLong()
	if err != nil {
		return nil, err
	}
	minY, err := r.in.ReadVLong()
	if err != nil {
		return nil, err
	}
	maxY, err := r.in.ReadVLong()
	if err != nil {
		return nil, err
	}
	r.bbox.reset(
		int32(minX+int64(math.MinInt32)),
		int32(maxX+int64(math.MinInt32)),
		int32(minY+int64(math.MinInt32)),
		int32(maxY+int64(math.MinInt32)),
	)
	return &r.bbox, nil
}

// shapeHeaderReadType mirrors Reader.Header.readType.
func shapeHeaderReadType(bits int32) DecodedTriangleType {
	if bits&0x04 == 0x04 {
		return DecodedTriangleTypePoint
	}
	if bits&0x08 == 0x08 {
		return DecodedTriangleTypeLine
	}
	return DecodedTriangleTypeTriangle
}

func shapeHeaderHasLeft(bits int32) bool  { return bits&0x02 == 0x02 }
func shapeHeaderHasRight(bits int32) bool { return bits&0x01 == 0x01 }

// shapeComparator parses the header once at construction time and
// then drives the recursive Relate traversal.
type shapeComparator struct {
	reader           *shapeReader
	encoder          ShapeDocValuesEncoder
	numberOfTerms    int
	boundingBox      shapeBBox
	centroidX        int32
	centroidY        int32
	highestDimension DecodedTriangleType
}

func newShapeComparator(encoder ShapeDocValuesEncoder, binaryValue *util.BytesRef) (*shapeComparator, error) {
	rdr := newShapeReader(binaryValue)
	version, err := rdr.in.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("ShapeDocValues: read version: %w", err)
	}
	if version != shapeDocValuesVersion {
		return nil, fmt.Errorf("ShapeDocValues: unsupported version %d", version)
	}
	numTerms, err := rdr.in.ReadVInt()
	if err != nil {
		return nil, fmt.Errorf("ShapeDocValues: read numTerms: %w", err)
	}
	bbox, err := rdr.readBBox()
	if err != nil {
		return nil, fmt.Errorf("ShapeDocValues: read bbox: %w", err)
	}
	bboxCopy := *bbox

	cx, err := rdr.in.ReadVLong()
	if err != nil {
		return nil, fmt.Errorf("ShapeDocValues: read centroidX: %w", err)
	}
	cy, err := rdr.in.ReadVLong()
	if err != nil {
		return nil, fmt.Errorf("ShapeDocValues: read centroidY: %w", err)
	}
	hd, err := rdr.in.ReadVInt()
	if err != nil {
		return nil, fmt.Errorf("ShapeDocValues: read highestDimension: %w", err)
	}
	rdr.rewind()
	return &shapeComparator{
		reader:           rdr,
		encoder:          encoder,
		numberOfTerms:    int(numTerms),
		boundingBox:      bboxCopy,
		centroidX:        int32(cx + int64(math.MinInt32)),
		centroidY:        int32(cy + int64(math.MinInt32)),
		highestDimension: DecodedTriangleType(hd),
	}, nil
}

// skipCentroid mirrors the Java private method ShapeComparator.skipCentroid.
func (c *shapeComparator) skipCentroid() error {
	if _, err := c.reader.in.ReadVLong(); err != nil {
		return err
	}
	if _, err := c.reader.in.ReadVLong(); err != nil {
		return err
	}
	return nil
}

func (c *shapeComparator) skipHighestDimension() error {
	_, err := c.reader.in.ReadVInt()
	return err
}

// relate is the main entry point. Mirrors
// ShapeComparator.relate(Component2D). On exit the reader is
// rewound to the start of the payload so the comparator may be
// re-used.
func (c *shapeComparator) relate(query geo.Component2D) (rel geo.Relation, retErr error) {
	defer c.reader.rewind()

	if _, err := c.reader.in.ReadByte(); err != nil {
		return geo.CellOutsideQuery, err
	}
	if _, err := c.reader.in.ReadVInt(); err != nil {
		return geo.CellOutsideQuery, err
	}
	bbox, err := c.reader.readBBox()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	tMinX := bbox.minX
	tMaxX := bbox.maxX
	tMaxY := bbox.maxY

	r := query.Relate(
		c.encoder.DecodeX(bbox.minX),
		c.encoder.DecodeX(bbox.maxX),
		c.encoder.DecodeY(bbox.minY),
		c.encoder.DecodeY(bbox.maxY),
	)
	if r != geo.CellCrossesQuery {
		return r, nil
	}

	if err := c.skipCentroid(); err != nil {
		return geo.CellOutsideQuery, err
	}
	if err := c.skipHighestDimension(); err != nil {
		return geo.CellOutsideQuery, err
	}

	headerBits, err := c.reader.in.ReadVInt()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	xDelta, err := c.reader.in.ReadVLong()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	x := int32(int64(tMaxX) - xDelta)

	rc, err := c.relateComponent(shapeHeaderReadType(headerBits), bbox, tMaxX, tMaxY, c.encoder.DecodeX(x), query)
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	if rc == geo.CellCrossesQuery {
		return geo.CellCrossesQuery, nil
	}
	r = geo.CellOutsideQuery

	if shapeHeaderHasLeft(headerBits) {
		size, err := c.reader.in.ReadVInt()
		if err != nil {
			return geo.CellOutsideQuery, err
		}
		rr, err := c.relateRecurse(query, false, tMaxX, tMaxY, int(size))
		if err != nil {
			return geo.CellOutsideQuery, err
		}
		if rr == geo.CellCrossesQuery {
			return geo.CellCrossesQuery, nil
		}
		r = rr
	}
	if shapeHeaderHasRight(headerBits) {
		if query.MaxX() >= c.encoder.DecodeX(tMinX) {
			size, err := c.reader.in.ReadVInt()
			if err != nil {
				return geo.CellOutsideQuery, err
			}
			rr, err := c.relateRecurse(query, false, tMaxX, tMaxY, int(size))
			if err != nil {
				return geo.CellOutsideQuery, err
			}
			if rr == geo.CellCrossesQuery {
				return geo.CellCrossesQuery, nil
			}
			r = rr
		}
	}
	return r, nil
}

// relateRecurse mirrors the Java private method
// ShapeComparator.relate(Component2D, boolean, int, int, int).
func (c *shapeComparator) relateRecurse(
	query geo.Component2D, splitX bool, pMaxX, pMaxY int32, nodeSize int,
) (geo.Relation, error) {
	prePos := c.reader.in.GetPosition()
	tMinXDelta, err := c.reader.in.ReadVLong()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	tMinYDelta, err := c.reader.in.ReadVLong()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	tMaxXDelta, err := c.reader.in.ReadVLong()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	tMaxYDelta, err := c.reader.in.ReadVLong()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	tMinX := int32(int64(pMaxX) - tMinXDelta)
	tMinY := int32(int64(pMaxY) - tMinYDelta)
	tMaxX := int32(int64(pMaxX) - tMaxXDelta)
	tMaxY := int32(int64(pMaxY) - tMaxYDelta)

	headerBits, err := c.reader.in.ReadVInt()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	nodeSize -= c.reader.in.GetPosition() - prePos

	// Cheap reject: if the query lies strictly beyond tMaxX/tMaxY
	// the whole subtree can be skipped.
	if query.MinX() > c.encoder.DecodeX(tMaxX) || query.MinY() > c.encoder.DecodeY(tMaxY) {
		if err := c.reader.in.SetPosition(c.reader.in.GetPosition() + nodeSize); err != nil {
			return geo.CellOutsideQuery, err
		}
		return geo.CellOutsideQuery, nil
	}

	xDelta, err := c.reader.in.ReadVLong()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	x := int32(int64(pMaxX) - xDelta)
	c.reader.bbox.reset(tMinX, tMaxX, tMinY, tMaxY)

	rc, err := c.relateComponent(
		shapeHeaderReadType(headerBits), &c.reader.bbox, pMaxX, pMaxY, c.encoder.DecodeX(x), query,
	)
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	if rc == geo.CellCrossesQuery {
		return geo.CellCrossesQuery, nil
	}

	if shapeHeaderHasLeft(headerBits) {
		size, err := c.reader.in.ReadVInt()
		if err != nil {
			return geo.CellOutsideQuery, err
		}
		rr, err := c.relateRecurse(query, !splitX, tMaxX, tMaxY, int(size))
		if err != nil {
			return geo.CellOutsideQuery, err
		}
		if rr == geo.CellCrossesQuery {
			return geo.CellCrossesQuery, nil
		}
	}
	if shapeHeaderHasRight(headerBits) {
		size, err := c.reader.in.ReadVInt()
		if err != nil {
			return geo.CellOutsideQuery, err
		}
		if (!splitX && query.MaxY() >= c.encoder.DecodeY(tMinY)) ||
			(splitX && query.MaxX() >= c.encoder.DecodeX(tMinX)) {
			rr, err := c.relateRecurse(query, !splitX, tMaxX, tMaxY, int(size))
			if err != nil {
				return geo.CellOutsideQuery, err
			}
			if rr == geo.CellCrossesQuery {
				return geo.CellCrossesQuery, nil
			}
		} else {
			if err := c.reader.in.SetPosition(c.reader.in.GetPosition() + int(size)); err != nil {
				return geo.CellOutsideQuery, err
			}
		}
	}
	return geo.CellOutsideQuery, nil
}

func (c *shapeComparator) relateComponent(
	kind DecodedTriangleType,
	bbox *shapeBBox,
	pMaxX, pMaxY int32,
	x float64,
	query geo.Component2D,
) (geo.Relation, error) {
	var r geo.Relation
	var err error
	switch kind {
	case DecodedTriangleTypePoint:
		r, err = c.relatePoint(pMaxY, x, query)
	case DecodedTriangleTypeLine:
		r, err = c.relateLine(pMaxX, pMaxY, x, query)
	case DecodedTriangleTypeTriangle:
		r, err = c.relateTriangle(pMaxX, pMaxY, x, query)
	default:
		r = geo.CellOutsideQuery
	}
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	if r == geo.CellCrossesQuery {
		return geo.CellCrossesQuery, nil
	}
	return geo.CellOutsideQuery, nil
}

func (c *shapeComparator) relatePoint(pMaxY int32, ax float64, query geo.Component2D) (geo.Relation, error) {
	yDelta, err := c.reader.in.ReadVLong()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	y := int32(int64(pMaxY) - yDelta)
	if query.Contains(ax, c.encoder.DecodeY(y)) {
		return geo.CellCrossesQuery, nil
	}
	return geo.CellOutsideQuery, nil
}

func (c *shapeComparator) relateLine(pMaxX, pMaxY int32, ax float64, query geo.Component2D) (geo.Relation, error) {
	ayDelta, err := c.reader.in.ReadVLong()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	bxDelta, err := c.reader.in.ReadVLong()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	byDelta, err := c.reader.in.ReadVLong()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	ay := int32(int64(pMaxY) - ayDelta)
	bx := c.encoder.DecodeX(int32(int64(pMaxX) - bxDelta))
	by := int32(int64(pMaxY) - byDelta)
	if geo.IntersectsLineDefault(query, ax, c.encoder.DecodeY(ay), bx, c.encoder.DecodeY(by)) {
		return geo.CellCrossesQuery, nil
	}
	return geo.CellOutsideQuery, nil
}

func (c *shapeComparator) relateTriangle(pMaxX, pMaxY int32, ax float64, query geo.Component2D) (geo.Relation, error) {
	ayDelta, err := c.reader.in.ReadVLong()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	bxDelta, err := c.reader.in.ReadVLong()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	byDelta, err := c.reader.in.ReadVLong()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	cxDelta, err := c.reader.in.ReadVLong()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	cyDelta, err := c.reader.in.ReadVLong()
	if err != nil {
		return geo.CellOutsideQuery, err
	}
	ay := int32(int64(pMaxY) - ayDelta)
	bx := c.encoder.DecodeX(int32(int64(pMaxX) - bxDelta))
	by := int32(int64(pMaxY) - byDelta)
	cx := c.encoder.DecodeX(int32(int64(pMaxX) - cxDelta))
	cy := int32(int64(pMaxY) - cyDelta)
	if geo.IntersectsTriangleDefault(
		query, ax, c.encoder.DecodeY(ay), bx, c.encoder.DecodeY(by), cx, c.encoder.DecodeY(cy),
	) {
		return geo.CellCrossesQuery, nil
	}
	return geo.CellOutsideQuery, nil
}

// =========================================================================
// Public helpers: variable-length size functions, exported because
// the Java reference exports them protected-static and the test peer
// relies on them.
// =========================================================================

// VLongSize returns the number of bytes a variable-length long
// occupies on disk. Mirrors ShapeDocValues.vLongSize.
func VLongSize(i int64) int {
	size := 0
	u := uint64(i)
	for u&^0x7F != 0 {
		u >>= 7
		size++
	}
	return size + 1
}

// VIntSize returns the number of bytes a variable-length int
// occupies on disk. Mirrors ShapeDocValues.vIntSize.
func VIntSize(i int32) int {
	size := 0
	u := uint32(i)
	for u&^0x7F != 0 {
		u >>= 7
		size++
	}
	return size + 1
}

// =========================================================================
// Stub: newGeometryQuery. Mirrors the Java static method
// ShapeDocValues.newGeometryQuery which currently returns null with
// a TODO comment pending the ShapeDocValuesQuery port.
// =========================================================================

// NewGeometryQuery is a placeholder mirroring the Java stub of the
// same name: it currently returns nil pending the
// ShapeDocValuesQuery port. The signature returns interface{} so we
// avoid an import cycle on search.Query; callers in the search
// layer will type-assert when the concrete query type lands.
//
// TODO(GOC-4532+): return the actual ShapeDocValuesQuery instance
// once the spatial query family is ported.
func NewGeometryQuery(field string, relation QueryRelation, geometries ...interface{}) interface{} {
	_ = field
	_ = relation
	_ = geometries
	return nil
}

// =========================================================================
// Local arithmetic helpers (kept package-local so the hot path stays
// allocation-free).
// =========================================================================

func minInt32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func minInt32x3(a, b, c int32) int32 { return minInt32(minInt32(a, b), c) }
func maxInt32x3(a, b, c int32) int32 { return maxInt32(maxInt32(a, b), c) }
