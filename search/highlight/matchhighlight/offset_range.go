package matchhighlight

import "fmt"

// OffsetRange represents a non-empty range of offset positions.
type OffsetRange struct {
	From int // Start index, inclusive.
	To   int // End index, exclusive.
}

// NewOffsetRange creates a new OffsetRange.
func NewOffsetRange(from, to int) OffsetRange {
	return OffsetRange{From: from, To: to}
}

// Length returns the length of the range.
func (o OffsetRange) Length() int {
	return o.To - o.From
}

func (o OffsetRange) String() string {
	return fmt.Sprintf("[from=%d, to=%d]", o.From, o.To)
}

// Slice returns a sub-range of this range (a copy).
func (o OffsetRange) Slice(from, to int) OffsetRange {
	return OffsetRange{From: from, To: to}
}

// Contains returns true if this range contains or is equal to other.
func (o OffsetRange) Contains(other OffsetRange) bool {
	return o.From <= other.From && o.To >= other.To
}
