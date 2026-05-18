// Package matchhighlight implements
// org.apache.lucene.search.matchhighlight: the matches API-driven highlighter
// that consumes per-document matches produced by Weight#matches and renders
// passages around them.
package matchhighlight

// OffsetRange is an inclusive-start, exclusive-end character interval.
// Mirrors org.apache.lucene.search.matchhighlight.OffsetRange.
type OffsetRange struct {
	From int
	To   int
}

// NewOffsetRange builds an OffsetRange.
func NewOffsetRange(from, to int) OffsetRange {
	if from > to {
		from, to = to, from
	}
	return OffsetRange{From: from, To: to}
}

// Length returns the size of the interval.
func (r OffsetRange) Length() int { return r.To - r.From }

// Contains reports whether pos lies inside [From, To).
func (r OffsetRange) Contains(pos int) bool { return pos >= r.From && pos < r.To }

// Overlaps reports whether two ranges intersect.
func (r OffsetRange) Overlaps(other OffsetRange) bool {
	return r.From < other.To && other.From < r.To
}
