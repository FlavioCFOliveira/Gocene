package uhighlight

// OffsetSource enumerates the mechanisms a FieldOffsetStrategy can use to
// locate offset positions within a document. Mirrors the inner enum
// org.apache.lucene.search.uhighlight.UnifiedHighlighter.OffsetSource.
type OffsetSource int

const (
	// OffsetSourcePostings reads offsets from indexed postings.
	OffsetSourcePostings OffsetSource = iota
	// OffsetSourceTermVectors reads offsets from stored term vectors.
	OffsetSourceTermVectors
	// OffsetSourceAnalysis re-runs the field analyzer to derive offsets.
	OffsetSourceAnalysis
	// OffsetSourcePostingsWithTermVectors reads from postings and falls back
	// to term vectors.
	OffsetSourcePostingsWithTermVectors
	// OffsetSourceNoneNeeded signals that no offset extraction is needed.
	OffsetSourceNoneNeeded
)

// FieldOffsetStrategy is the contract the unified highlighter uses to build
// an OffsetsEnum for a (field, doc) pair. Mirrors the abstract class
// org.apache.lucene.search.uhighlight.FieldOffsetStrategy.
//
// Concrete implementations embed BaseFieldOffsetStrategy to inherit the
// field-name plumbing, and must implement GetOffsetsEnum and GetOffsetSource.
type FieldOffsetStrategy interface {
	// Field returns the field name this strategy targets.
	Field() string

	// GetOffsetSource returns the OffsetSource that characterises how this
	// strategy resolves document offsets.
	GetOffsetSource() OffsetSource

	// GetOffsetsEnum returns the OffsetsEnum for the supplied document
	// context.  The Go port keeps the doc reference opaque so concrete
	// strategies can supply whichever per-segment state they need.
	GetOffsetsEnum(docContext any) (OffsetsEnum, error)
}

// BaseFieldOffsetStrategy carries the UHComponents shared by every concrete
// strategy. It mirrors the protected `components` field and the getField()
// accessor of the Java abstract class
// (FieldOffsetStrategy.java:42 and :48).
type BaseFieldOffsetStrategy struct {
	components *UHComponents
}

// NewBaseFieldOffsetStrategy builds the embed.
func NewBaseFieldOffsetStrategy(components *UHComponents) BaseFieldOffsetStrategy {
	return BaseFieldOffsetStrategy{components: components}
}

// Components returns the UHComponents this strategy was built with. It renders
// access to the protected `components` field of the Java abstract class, which
// subclasses read directly.
func (s BaseFieldOffsetStrategy) Components() *UHComponents { return s.components }

// Field returns the field name. Mirrors FieldOffsetStrategy.getField().
func (s BaseFieldOffsetStrategy) Field() string { return s.components.Field }
