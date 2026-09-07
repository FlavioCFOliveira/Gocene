package search

import (
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
		current := packedPoints.Next()
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

func (q *PointInSetQuery) CreateWeight(searcher IndexSearcher, scoreMode ScoreMode, boost float32) Weight {
	return &pointInSetWeight{
		query:  q,
		boost:  boost,
		score:  boost, // Constant score
		sMode:  scoreMode,
	}
}

type pointInSetWeight struct {
	query  *PointInSetQuery
	boost  float32
	score  float32
	sMode  ScoreMode
}

func (w *pointInSetWeight) IsCacheable(ctx index.LeafReaderContext) bool {
	return true
}

func (w *pointInSetWeight) ScorerSupplier(ctx index.LeafReaderContext) ScorerSupplier {
	reader := ctx.Reader()
	values := reader.GetPointValues(w.query.field)
	if values == nil {
		return nil
	}

	if values.GetNumDimensions() != w.query.numDims {
		panic(fmt.Sprintf("field=%q was indexed with numIndexDims=%d but this query has numIndexDims=%d",
			w.query.field, values.GetNumDimensions(), w.query.numDims))
	}
	if values.GetBytesPerDimension() != w.query.bytesPerDim {
		panic(fmt.Sprintf("field=%q was indexed with bytesPerDim=%d but this query has bytesPerDim=%d",
			w.query.field, values.GetBytesPerDimension(), w.query.bytesPerDim))
	}

	if values.GetDocCount() == 0 {
		return nil
	} else if w.query.lowerPoint != nil {
		// Fast overlap check
		minPacked, _ := values.GetMinPackedValue()
		maxPacked, _ := values.GetMaxPackedValue()
		for i := 0; i < w.query.numDims; i++ {
			offset := i * w.query.bytesPerDim
			if bytes.Compare(w.query.lowerPoint[offset:], maxPacked[offset:]) > 0 ||
				bytes.Compare(w.query.upperPoint[offset:], minPacked[offset:]) < 0 {
				return nil
			}
		}
	}

	if w.query.numDims == 1 {
		return &pointInSetScorerSupplier1D{
			query:  w.query,
			reader: reader,
			values: values,
			weight: w,
		}
	}

	return &pointInSetScorerSupplierND{
		query:  w.query,
		reader: reader,
		values: values,
		weight: w,
	}
}

type pointInSetScorerSupplier1D struct {
	query  *PointInSetQuery
	reader index.LeafReader
	values index.PointValues
	weight *pointInSetWeight
}

func (s *pointInSetScorerSupplier1D) Cost() int64 {
	result := util.NewDocIdSetBuilder(s.reader.MaxDoc())
	cost := s.values.Intersect(newMergePointVisitor(s.query.sortedPackedPoints, result))
	return int64(cost) // Simplified: assume Intersect returns count or use builder count
}

func (s *pointInSetScorerSupplier1D) Get(leadCost int64) Scorer {
	result := util.NewDocIdSetBuilder(s.reader.MaxDoc())
	_ = s.values.Intersect(newMergePointVisitor(s.query.sortedPackedPoints, result))
	return NewConstantScoreScorer(s.weight.score, s.weight.sMode, result.Build())
}

type pointInSetScorerSupplierND struct {
	query  *PointInSetQuery
	reader index.LeafReader
	values index.PointValues
	weight *pointInSetWeight
}

func (s *pointInSetScorerSupplierND) Cost() int64 {
	var cost int64
	result := util.NewDocIdSetBuilder(s.reader.MaxDoc())
	visitor := &singlePointVisitor{
		result: result,
		bytesPerDim: s.query.bytesPerDim,
		numDims: s.query.numDims,
	}
	iterator := s.query.sortedPackedPoints.Iterator()
	for point := iterator.Next(); point != nil; point = iterator.Next() {
		visitor.setPoint(point)
		cost += int64(s.values.Intersect(visitor)) // Assume Intersect returns count for estimate
	}
	return cost
}

func (s *pointInSetScorerSupplierND) Get(leadCost int64) Scorer {
	result := util.NewDocIdSetBuilder(s.reader.MaxDoc())
	visitor := &singlePointVisitor{
		result: result,
		bytesPerDim: s.query.bytesPerDim,
		numDims: s.query.numDims,
	}
	iterator := s.query.sortedPackedPoints.Iterator()
	for point := iterator.Next(); point != nil; point = iterator.Next() {
		visitor.setPoint(point)
		_ = s.values.Intersect(visitor)
	}
	return NewConstantScoreScorer(s.weight.score, s.weight.sMode, result.Build())
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

func (q *PointInSetQuery) Equals(other any) bool {
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
		q.sortedPackedPoints.equals(o.sortedPackedPoints)
}

// equals is a helper for PrefixCodedTerms comparison.
func (p *PrefixCodedTerms) equals(other *PrefixCodedTerms) bool {
	if len(p.terms) != len(other.terms) {
		return false
	}
	for i := range p.terms {
		if p.terms[i].field != other.terms[i].field || !bytes.Equal(p.terms[i].bytes, other.terms[i].bytes) {
			return false
		}
	}
	return true
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
