// Package bbox implements org.apache.lucene.spatial.bbox helpers.
package bbox

// Rect is a (minX, minY, maxX, maxY) rectangle used by the bbox-based
// scoring helpers.
type Rect struct {
	MinX float64
	MinY float64
	MaxX float64
	MaxY float64
}

// Area returns the rectangle's area.
func (r Rect) Area() float64 {
	w := r.MaxX - r.MinX
	h := r.MaxY - r.MinY
	if w < 0 || h < 0 {
		return 0
	}
	return w * h
}

// Intersection returns the rectangle representing the overlap with other (or
// a zero-area rectangle when disjoint).
func (r Rect) Intersection(other Rect) Rect {
	out := Rect{
		MinX: max64(r.MinX, other.MinX),
		MinY: max64(r.MinY, other.MinY),
		MaxX: min64(r.MaxX, other.MaxX),
		MaxY: min64(r.MaxY, other.MaxY),
	}
	if out.MaxX < out.MinX || out.MaxY < out.MinY {
		return Rect{}
	}
	return out
}

// BBoxOverlapRatioValueSource returns the overlap ratio between each
// document's bbox and a reference rectangle. Mirrors
// org.apache.lucene.spatial.bbox.BBoxOverlapRatioValueSource.
type BBoxOverlapRatioValueSource struct {
	Reference Rect
	DocFn     func(docID int) (Rect, error)
}

// NewBBoxOverlapRatioValueSource builds the source.
func NewBBoxOverlapRatioValueSource(reference Rect, fn func(docID int) (Rect, error)) *BBoxOverlapRatioValueSource {
	return &BBoxOverlapRatioValueSource{Reference: reference, DocFn: fn}
}

// GetValues returns the overlap ratio in [0, 1].
func (s *BBoxOverlapRatioValueSource) GetValues(docID int) (float64, error) {
	if s.DocFn == nil {
		return 0, nil
	}
	r, err := s.DocFn(docID)
	if err != nil {
		return 0, err
	}
	inter := r.Intersection(s.Reference)
	area := r.Area() + s.Reference.Area() - inter.Area()
	if area <= 0 {
		return 0, nil
	}
	return inter.Area() / area, nil
}

// BBoxSimilarityValueSource scores documents by similarity between their
// bbox and a reference. Mirrors
// org.apache.lucene.spatial.bbox.BBoxSimilarityValueSource.
type BBoxSimilarityValueSource struct {
	Reference Rect
	DocFn     func(docID int) (Rect, error)
}

// NewBBoxSimilarityValueSource builds the source.
func NewBBoxSimilarityValueSource(reference Rect, fn func(docID int) (Rect, error)) *BBoxSimilarityValueSource {
	return &BBoxSimilarityValueSource{Reference: reference, DocFn: fn}
}

// GetValues returns the inverse of (1 + non-overlapping-area).
func (s *BBoxSimilarityValueSource) GetValues(docID int) (float64, error) {
	if s.DocFn == nil {
		return 0, nil
	}
	r, err := s.DocFn(docID)
	if err != nil {
		return 0, err
	}
	inter := r.Intersection(s.Reference)
	nonOverlap := r.Area() + s.Reference.Area() - 2*inter.Area()
	return 1.0 / (1.0 + nonOverlap), nil
}

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
