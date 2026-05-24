// Package util implements org.apache.lucene.spatial.util.
package util

// DoubleValuesSource is the lightweight contract every spatial double value
// source satisfies.
type DoubleValuesSource interface {
	GetValues(docID int) (float64, error)
}

// CachingDoubleValueSource memoises the values returned by an inner source.
// Mirrors org.apache.lucene.spatial.util.CachingDoubleValueSource.
type CachingDoubleValueSource struct {
	Inner DoubleValuesSource
	cache map[int]float64
}

// NewCachingDoubleValueSource builds the cache.
func NewCachingDoubleValueSource(inner DoubleValuesSource) *CachingDoubleValueSource {
	return &CachingDoubleValueSource{Inner: inner, cache: make(map[int]float64)}
}

// GetValues memoises the inner source.
func (s *CachingDoubleValueSource) GetValues(docID int) (float64, error) {
	if v, ok := s.cache[docID]; ok {
		return v, nil
	}
	v, err := s.Inner.GetValues(docID)
	if err != nil {
		return 0, err
	}
	s.cache[docID] = v
	return v, nil
}

var _ DoubleValuesSource = (*CachingDoubleValueSource)(nil)

// ReciprocalDoubleValuesSource transforms x -> a/(b+x*c). Mirrors
// org.apache.lucene.spatial.util.ReciprocalDoubleValuesSource.
type ReciprocalDoubleValuesSource struct {
	Inner DoubleValuesSource
	A     float64
	B     float64
	C     float64
}

// NewReciprocalDoubleValuesSource builds the source.
func NewReciprocalDoubleValuesSource(inner DoubleValuesSource, a, b, c float64) *ReciprocalDoubleValuesSource {
	return &ReciprocalDoubleValuesSource{Inner: inner, A: a, B: b, C: c}
}

// GetValues applies the reciprocal transformation.
func (s *ReciprocalDoubleValuesSource) GetValues(docID int) (float64, error) {
	v, err := s.Inner.GetValues(docID)
	if err != nil {
		return 0, err
	}
	denom := s.B + s.C*v
	if denom == 0 {
		return 0, nil
	}
	return s.A / denom, nil
}

var _ DoubleValuesSource = (*ReciprocalDoubleValuesSource)(nil)

// DistanceToShapeValueSource returns the distance from each document's
// resolved shape to a fixed reference shape.
type DistanceToShapeValueSource struct {
	DistanceFn func(docID int) (float64, error)
}

// NewDistanceToShapeValueSource builds the source.
func NewDistanceToShapeValueSource(fn func(docID int) (float64, error)) *DistanceToShapeValueSource {
	return &DistanceToShapeValueSource{DistanceFn: fn}
}

// GetValues delegates to the configured function.
func (s *DistanceToShapeValueSource) GetValues(docID int) (float64, error) {
	if s.DistanceFn == nil {
		return 0, nil
	}
	return s.DistanceFn(docID)
}

var _ DoubleValuesSource = (*DistanceToShapeValueSource)(nil)

// ShapeAreaValueSource returns the area (or perimeter for geodesic shapes)
// of each document's resolved shape.
type ShapeAreaValueSource struct {
	AreaFn func(docID int) (float64, error)
}

// NewShapeAreaValueSource builds the source.
func NewShapeAreaValueSource(fn func(docID int) (float64, error)) *ShapeAreaValueSource {
	return &ShapeAreaValueSource{AreaFn: fn}
}

// GetValues delegates to the configured function.
func (s *ShapeAreaValueSource) GetValues(docID int) (float64, error) {
	if s.AreaFn == nil {
		return 0, nil
	}
	return s.AreaFn(docID)
}

var _ DoubleValuesSource = (*ShapeAreaValueSource)(nil)
