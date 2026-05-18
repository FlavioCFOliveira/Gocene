package uhighlight

// FieldOffsetStrategy is the contract the unified highlighter uses to build
// an OffsetsEnum for a (field, doc) pair. Mirrors the abstract
// org.apache.lucene.search.uhighlight.FieldOffsetStrategy.
type FieldOffsetStrategy interface {
	// Field returns the field name this strategy targets.
	Field() string

	// GetOffsetsEnum returns the OffsetsEnum for the supplied document.
	// The Go port keeps the doc reference opaque so concrete strategies can
	// supply whichever per-segment state they need.
	GetOffsetsEnum(docContext any) (OffsetsEnum, error)
}

// BaseFieldOffsetStrategy provides the field-name plumbing shared by every
// concrete strategy.
type BaseFieldOffsetStrategy struct {
	field string
}

// NewBaseFieldOffsetStrategy builds the embed.
func NewBaseFieldOffsetStrategy(field string) BaseFieldOffsetStrategy {
	return BaseFieldOffsetStrategy{field: field}
}

// Field returns the field name.
func (s BaseFieldOffsetStrategy) Field() string { return s.field }
