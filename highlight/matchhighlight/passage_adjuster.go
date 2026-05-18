package matchhighlight

// PassageAdjuster is the contract used by MatchHighlighter to widen or trim
// the boundaries of a passage. Mirrors
// org.apache.lucene.search.matchhighlight.PassageAdjuster.
type PassageAdjuster interface {
	// Adjust returns the adjusted boundary applied to text within the
	// supplied OffsetRange.
	Adjust(text string, region OffsetRange) OffsetRange
}

// IdentityPassageAdjuster returns the passage range unchanged.
type IdentityPassageAdjuster struct{}

// Adjust returns region verbatim.
func (IdentityPassageAdjuster) Adjust(_ string, region OffsetRange) OffsetRange { return region }

var _ PassageAdjuster = IdentityPassageAdjuster{}
