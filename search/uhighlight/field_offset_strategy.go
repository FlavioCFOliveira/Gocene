package uhighlight

// OffsetSource enumerates the mechanisms a FieldOffsetStrategy can use to
// locate offset positions within a document. Mirrors the inner enum
// org.apache.lucene.search.uhighlight.UnifiedHighlighter.OffsetSource.
type OffsetSource int

const (
	// OffsetSourcePostings reads offsets from indexed postings.
	OffsetSourcePostings OffsetSource = iota
	// OffsetSourcePostingsWithTermVectors reads from postings and falls back
	// to term vectors.
	OffsetSourcePostingsWithTermVectors
	// OffsetSourceTermVectors reads offsets from stored term vectors.
	OffsetSourceTermVectors
	// OffsetSourceAnalysis re-runs the field analyzer to derive offsets.
	OffsetSourceAnalysis
	// OffsetSourceNone signals that no offset extraction takes place.
	OffsetSourceNone
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

// BaseFieldOffsetStrategy provides the field-name plumbing shared by every
// concrete strategy.  It mirrors the protected field and getField() accessor
// of the Java abstract class.
type BaseFieldOffsetStrategy struct {
	field string
}

// NewBaseFieldOffsetStrategy builds the embed.
func NewBaseFieldOffsetStrategy(field string) BaseFieldOffsetStrategy {
	return BaseFieldOffsetStrategy{field: field}
}

// Field returns the field name.
func (s BaseFieldOffsetStrategy) Field() string { return s.field }
