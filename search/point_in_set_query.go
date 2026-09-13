package search

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PointInSetQuery is a query that finds all documents whose single or multi-dimensional point values
// are contained in the specified set.
//
// This is a base query class; concrete implementations should embed this and provide
// the toStringValue function.
type PointInSetQuery struct {
	field                      string
	numDims                    int
	bytesPerDim                int
	sortedPackedPoints         *index.PrefixCodedTerms
	sortedPackedPointsHashCode int
	ramBytesUsed               int64
	lowerPoint                 []byte
	upperPoint                 []byte
	toStringValue              func([]byte) string
}

// PointInSetQueryStream is an iterator of encoded point values.
type PointInSetQueryStream interface {
	util.BytesRefIterator
}

// NewPointInSetQuery creates a new PointInSetQuery.
// packedPoints must be in sorted order.
func NewPointInSetQuery(field string, numDims, bytesPerDim int, packedPoints PointInSetQueryStream, toStringValue func([]byte) string) (*PointInSetQuery, error) {
	if bytesPerDim < 1 || bytesPerDim > 8 { // Use 8 as MAX_NUM_BYTES based on BKDTree
		return nil, fmt.Errorf("bytesPerDim must be > 0 and <= 8; got %d", bytesPerDim)
	}
	if numDims < 1 || numDims > 32 { // Use 32 as MAX_INDEX_DIMENSIONS (approx)
		return nil, fmt.Errorf("numDims must be > 0 and <= 32; got %d", numDims)
	}

	builder := index.NewPrefixCodedTermsBuilder()
	var previous *util.BytesRef
	var lowerPoint, upperPoint []byte

	for {
		current, err := packedPoints.Next()
		if err != nil {
			return nil, err
		}
		if current == nil {
			break
		}
		if len(current.Bytes) != numDims*bytesPerDim {
			return nil, fmt.Errorf("packed point length should be %d but got %d; field=%q numDims=%d bytesPerDim=%d",
				numDims*bytesPerDim, len(current.Bytes), field, numDims, bytesPerDim)
		}

		if previous == nil {
			lowerPoint = make([]byte, len(current.Bytes))
			copy(lowerPoint, current.Bytes[current.Offset:])
		} else {
			cmp := bytes.Compare(previous.Bytes[previous.Offset:], current.Bytes[current.Offset:])
			if cmp == 0 {
				continue // deduplicate
			} else if cmp > 0 {
				return nil, fmt.Errorf("values are out of order: saw %v before %v", previous, current)
			}
		}
		builder.AddFieldBytes(field, current.Bytes[current.Offset:])
		previous = current
	}

	sortedPackedPoints := builder.Finish()
	if previous != nil {
		upperPoint = make([]byte, len(previous.Bytes))
		copy(upperPoint, previous.Bytes[previous.Offset:])
	}

	// Hash code calculation mirroring Java's 31 * hash + ...
	hash := 0
	hash = 31*hash + len(field) // Simplification for field hash
	hash = 31*hash + int(sortedPackedPoints.Size())
	hash = 31*hash + numDims
	hash = 31*hash + bytesPerDim

	ramBytesUsed := int64(util.ShallowSizeOf(field)) +
		int64(util.ShallowSizeOf(sortedPackedPoints))

	return &PointInSetQuery{
		field:                      field,
		numDims:                    numDims,
		bytesPerDim:                bytesPerDim,
		sortedPackedPoints:         sortedPackedPoints,
		sortedPackedPointsHashCode: hash,
		ramBytesUsed:               ramBytesUsed,
		lowerPoint:                 lowerPoint,
		upperPoint:                 upperPoint,
		toStringValue:              toStringValue,
	}, nil
}

func (q *PointInSetQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q)
	}
}

func (q *PointInSetQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return &pointInSetWeight{
		BaseWeight: BaseWeight{query: q},
		query:      q,
		boost:      boost,
		score:      boost, // Constant score
		sMode:      scoreMode,
	}, nil
}

type pointInSetWeight struct {
	BaseWeight
	query *PointInSetQuery
	boost float32
	score float32
	sMode ScoreMode
}

// IsCacheable mirrors the anonymous ConstantScoreWeight's
// isCacheable(LeafReaderContext), whose body is `return true`.
func (w *pointInSetWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// pointInSetPointTreeIntersect is the rich, visitor-driven read surface a
// BKD-backed PointValues exposes beyond the metadata-only index.PointValues.
// The on-disk reader returned by LeafReader.GetPointValues (the codec's
// *pointValues) satisfies it structurally; the parameter type is the
// index-package alias so the type assertion succeeds for the real codec reader
// (the same reason LatLonPointDistanceQuery and PointRangeQuery alias
// index.PointTreeIntersectVisitor).
//
// PORT NOTE. Java's cost() calls PointValues.estimateDocCount(visitor), a final
// method that rescales estimatePointCount(visitor) by docCount/size. Gocene's
// BKD reader exposes only EstimatePointCount, so that is what the cost uses —
// the same treatment lat_lon_point_distance_query.go applies.
type pointInSetPointTreeIntersect interface {
	Intersect(visitor index.PointTreeIntersectVisitor) error
	EstimatePointCount(visitor index.PointTreeIntersectVisitor) int64
}

// ScorerSupplier mirrors the anonymous ConstantScoreWeight's
// scorerSupplier(LeafReaderContext) in PointInSetQuery.createWeight.
func (w *pointInSetWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	reader := ctx.LeafReader()
	values, err := reader.GetPointValues(w.query.field)
	if err != nil {
		return nil, err
	}
	if values == nil {
		// No docs in this segment/field indexed any points
		return nil, nil
	}

	if values.GetNumDimensions() != w.query.numDims {
		return nil, fmt.Errorf("field=%q was indexed with numIndexDims=%d but this query has numIndexDims=%d",
			w.query.field, values.GetNumDimensions(), w.query.numDims)
	}
	if values.GetBytesPerDimension() != w.query.bytesPerDim {
		return nil, fmt.Errorf("field=%q was indexed with bytesPerDim=%d but this query has bytesPerDim=%d",
			w.query.field, values.GetBytesPerDimension(), w.query.bytesPerDim)
	}

	if values.GetDocCount() == 0 {
		return nil, nil
	} else if w.query.lowerPoint != nil {
		// Fast overlap check
		minPacked, err := values.GetMinPackedValue()
		if err != nil {
			return nil, err
		}
		maxPacked, err := values.GetMaxPackedValue()
		if err != nil {
			return nil, err
		}
		for i := 0; i < w.query.numDims; i++ {
			offset := i * w.query.bytesPerDim
			if bytes.Compare(w.query.lowerPoint[offset:], maxPacked[offset:]) > 0 ||
				bytes.Compare(w.query.upperPoint[offset:], minPacked[offset:]) < 0 {
				return nil, nil
			}
		}
	}

	tree, ok := values.(pointInSetPointTreeIntersect)
	if !ok {
		// The PointValues does not expose the visitor-driven walk, so no
		// document can be materialised from it.
		return nil, nil
	}

	if w.query.numDims == 1 {
		// We optimize this common case, effectively doing a merge sort of the
		// indexed values vs the queried set.
		return &pointInSetScorerSupplier1D{
			query:  w.query,
			reader: reader,
			values: tree,
			weight: w,
			cost:   -1,
		}, nil
	}

	// NOTE: this is naive implementation, where for each point we re-walk the
	// KD tree to intersect.
	return &pointInSetScorerSupplierND{
		query:  w.query,
		reader: reader,
		values: tree,
		weight: w,
		cost:   -1,
	}, nil
}

type pointInSetScorerSupplier1D struct {
	BaseScorerSupplier
	query  *PointInSetQuery
	reader index.LeafReader
	values pointInSetPointTreeIntersect
	weight *pointInSetWeight
	cost   int64 // calculate lazily, only once
}

// Cost mirrors cost() of the 1-dimension anonymous ScorerSupplier: computing it
// may be expensive, so the estimate is produced once and cached.
func (s *pointInSetScorerSupplier1D) Cost() int64 {
	if s.cost == -1 {
		result := util.NewDocIdSetBuilder(s.reader.MaxDoc())
		s.cost = s.values.EstimatePointCount(newMergePointVisitor(s.query.sortedPackedPoints, result))
	}
	return s.cost
}

// Get mirrors get(long leadCost) of the anonymous ScorerSupplier that
// PointInSetQuery.createWeight returns in the 1-dimension case of Apache
// Lucene 10.5.0 (PointInSetQuery.java:209-214):
//
//	DocIdSetBuilder result = new DocIdSetBuilder(reader.maxDoc(), values);
//	values.intersect(new MergePointVisitor(sortedPackedPoints, result));
//	DocIdSetIterator iterator = result.build().iterator();
//	return new ConstantScoreScorer(score(), scoreMode, iterator);
//
// Java's get declares `throws IOException`, so the Go rendering returns
// (Scorer, error) and propagates the failures of intersect and build instead
// of discarding them.
func (s *pointInSetScorerSupplier1D) Get(leadCost int64) (Scorer, error) {
	result := util.NewDocIdSetBuilder(s.reader.MaxDoc())
	if err := s.values.Intersect(newMergePointVisitor(s.query.sortedPackedPoints, result)); err != nil {
		return nil, err
	}
	set, err := result.Build()
	if err != nil {
		return nil, err
	}
	return NewConstantScoreScorer(s.weight.score, s.weight.sMode, set.Iterator()), nil
}

type pointInSetScorerSupplierND struct {
	BaseScorerSupplier
	query  *PointInSetQuery
	reader index.LeafReader
	values pointInSetPointTreeIntersect
	weight *pointInSetWeight
	cost   int64 // calculate lazily, only once
}

// Cost mirrors cost() of the n-dimension anonymous ScorerSupplier: it sums the
// per-point estimates of one KD-tree walk each, once.
func (s *pointInSetScorerSupplierND) Cost() int64 {
	if s.cost == -1 {
		result := util.NewDocIdSetBuilder(s.reader.MaxDoc())
		visitor := &singlePointVisitor{
			result:      result,
			bytesPerDim: s.query.bytesPerDim,
			numDims:     s.query.numDims,
		}
		iterator := s.query.sortedPackedPoints.Iterator()
		var cost int64
		for point := iterator.Next(); point != nil; point = iterator.Next() {
			visitor.setPoint(point)
			cost += s.values.EstimatePointCount(visitor)
		}
		s.cost = cost
	}
	return s.cost
}

// Get mirrors get(long leadCost) of the anonymous ScorerSupplier that
// PointInSetQuery.createWeight returns in the n-dimension case of Apache
// Lucene 10.5.0 (PointInSetQuery.java:242-251): re-walk the KD tree once per
// queried point, then build one ConstantScoreScorer over the union.
//
// Java's get declares `throws IOException`, so the Go rendering returns
// (Scorer, error) and propagates the failures of intersect and build instead
// of discarding them.
func (s *pointInSetScorerSupplierND) Get(leadCost int64) (Scorer, error) {
	result := util.NewDocIdSetBuilder(s.reader.MaxDoc())
	visitor := &singlePointVisitor{
		result:      result,
		bytesPerDim: s.query.bytesPerDim,
		numDims:     s.query.numDims,
	}
	iterator := s.query.sortedPackedPoints.Iterator()
	for point := iterator.Next(); point != nil; point = iterator.Next() {
		visitor.setPoint(point)
		if err := s.values.Intersect(visitor); err != nil {
			return nil, err
		}
	}
	set, err := result.Build()
	if err != nil {
		return nil, err
	}
	return NewConstantScoreScorer(s.weight.score, s.weight.sMode, set.Iterator()), nil
}

type mergePointVisitor struct {
	result             *util.DocIdSetBuilder
	iterator           *index.PrefixCodedTermIterator
	nextQueryPoint     []byte
	sortedPackedPoints *index.PrefixCodedTerms
	adder              util.BulkAdder
}

func newMergePointVisitor(sortedPackedPoints *index.PrefixCodedTerms, result *util.DocIdSetBuilder) index.PointTreeIntersectVisitor {
	it := sortedPackedPoints.Iterator()
	var nextQueryPoint []byte
	if p := it.Next(); p != nil {
		nextQueryPoint = p
	}
	return &mergePointVisitor{
		result:             result,
		iterator:           it,
		nextQueryPoint:     nextQueryPoint,
		sortedPackedPoints: sortedPackedPoints,
	}
}

func (v *mergePointVisitor) Grow(count int) {
	v.adder = v.result.Grow(count)
}

func (v *mergePointVisitor) Visit(docID int) error {
	v.adder.Add(docID)
	return nil
}

func (v *mergePointVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	if v.matches(packedValue) {
		v.adder.Add(docID)
	}
	return nil
}

func (v *mergePointVisitor) Compare(minPackedValue, maxPackedValue []byte) int {
	for v.nextQueryPoint != nil {
		cmpMin := bytes.Compare(v.nextQueryPoint, minPackedValue)
		if cmpMin < 0 {
			v.nextQueryPoint = v.iterator.Next()
			continue
		}
		cmpMax := bytes.Compare(v.nextQueryPoint, maxPackedValue)
		if cmpMax > 0 {
			return 0 // CELL_OUTSIDE_QUERY
		}
		if cmpMin == 0 && cmpMax == 0 {
			return 1 // CELL_INSIDE_QUERY
		}
		return 2 // CELL_CROSSES_QUERY
	}
	return 0 // CELL_OUTSIDE_QUERY
}

func (v *mergePointVisitor) matches(packedValue []byte) bool {
	for v.nextQueryPoint != nil {
		cmp := bytes.Compare(v.nextQueryPoint, packedValue)
		if cmp == 0 {
			return true
		} else if cmp < 0 {
			v.nextQueryPoint = v.iterator.Next()
		} else {
			break
		}
	}
	return false
}

type singlePointVisitor struct {
	result      *util.DocIdSetBuilder
	bytesPerDim int
	numDims     int
	pointBytes  []byte
	adder       util.BulkAdder
}

func (v *singlePointVisitor) setPoint(point []byte) {
	v.pointBytes = make([]byte, len(point))
	copy(v.pointBytes, point)
}

func (v *singlePointVisitor) Grow(count int) {
	v.adder = v.result.Grow(count)
}

func (v *singlePointVisitor) Visit(docID int) error {
	v.adder.Add(docID)
	return nil
}

func (v *singlePointVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	if bytes.Equal(packedValue, v.pointBytes) {
		v.adder.Add(docID)
	}
	return nil
}

func (v *singlePointVisitor) Compare(minPackedValue, maxPackedValue []byte) int {
	crosses := false
	for dim := 0; dim < v.numDims; dim++ {
		offset := dim * v.bytesPerDim
		cmpMin := bytes.Compare(v.pointBytes[offset:], minPackedValue[offset:])
		if cmpMin > 0 {
			return 0 // CELL_OUTSIDE_QUERY
		}
		cmpMax := bytes.Compare(v.pointBytes[offset:], maxPackedValue[offset:])
		if cmpMax < 0 {
			return 0 // CELL_OUTSIDE_QUERY
		}
		if cmpMin != 0 || cmpMax != 0 {
			crosses = true
		}
	}
	if crosses {
		return 2 // CELL_CROSSES_QUERY
	}
	return 1 // CELL_INSIDE_QUERY
}

func (q *PointInSetQuery) GetPackedPoints() [][]byte {
	size := int(q.sortedPackedPoints.Size())
	out := make([][]byte, 0, size)
	it := q.sortedPackedPoints.Iterator()
	for p := it.Next(); p != nil; p = it.Next() {
		cp := make([]byte, len(p))
		copy(cp, p)
		out = append(out, cp)
	}
	return out
}

func (q *PointInSetQuery) GetField() string {
	return q.field
}

func (q *PointInSetQuery) GetNumDims() int {
	return q.numDims
}

func (q *PointInSetQuery) GetBytesPerDim() int {
	return q.bytesPerDim
}

func (q *PointInSetQuery) RamBytesUsed() int64 {
	return q.ramBytesUsed
}

// Rewrite mirrors the Query.rewrite(IndexSearcher) that PointInSetQuery
// inherits unchanged from Query, whose body is `return this`.
func (q *PointInSetQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	return q, nil
}

func (q *PointInSetQuery) Equals(other spi.Query) bool {
	if other == nil {
		return false
	}
	o, ok := other.(*PointInSetQuery)
	if !ok {
		return false
	}
	return o.field == q.field &&
		o.numDims == q.numDims &&
		o.bytesPerDim == q.bytesPerDim &&
		o.sortedPackedPointsHashCode == q.sortedPackedPointsHashCode &&
		q.sortedPackedPoints.Equals(o.sortedPackedPoints)
}

func (q *PointInSetQuery) ToString(field string) string {
	var sb bytes.Buffer
	if q.field != field {
		sb.WriteString(q.field)
		sb.WriteByte(':')
	}
	sb.WriteByte('{')
	it := q.sortedPackedPoints.Iterator()
	first := true
	for p := it.Next(); p != nil; p = it.Next() {
		if !first {
			sb.WriteByte(' ')
		}
		first = false
		sb.WriteString(q.toStringValue(p))
	}
	sb.WriteByte('}')
	return sb.String()
}

func (q *PointInSetQuery) HashCode() int {
	hash := 0
	hash = 31*hash + len(q.field)
	hash = 31*hash + q.sortedPackedPointsHashCode
	hash = 31*hash + q.numDims
	hash = 31*hash + q.bytesPerDim
	return hash
}

// BulkScorer mirrors the concrete body of ScorerSupplier.bulkScorer() in Apache
// Lucene 10.5.0: new DefaultBulkScorer(get(Long.MAX_VALUE)).
func (p *pointInSetScorerSupplier1D) BulkScorer() (BulkScorer, error) {
	return DefaultScorerSupplierBulkScorer(p)
}

// SetTopLevelScoringClause mirrors ScorerSupplier.setTopLevelScoringClause(),
// whose body in Apache Lucene 10.5.0 is empty.
func (p *pointInSetScorerSupplier1D) SetTopLevelScoringClause() error {
	return nil
}

// BulkScorer mirrors the concrete body of ScorerSupplier.bulkScorer() in Apache
// Lucene 10.5.0: new DefaultBulkScorer(get(Long.MAX_VALUE)).
func (p *pointInSetScorerSupplierND) BulkScorer() (BulkScorer, error) {
	return DefaultScorerSupplierBulkScorer(p)
}

// SetTopLevelScoringClause mirrors ScorerSupplier.setTopLevelScoringClause(),
// whose body in Apache Lucene 10.5.0 is empty.
func (p *pointInSetScorerSupplierND) SetTopLevelScoringClause() error {
	return nil
}
