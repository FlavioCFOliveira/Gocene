package uhighlight

import "github.com/FlavioCFOliveira/Gocene/index"

// PostingsOffsetStrategy uses offsets in postings --
// IndexOptions.DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS. This does not support
// multi-term queries; the highlighter will fallback on analysis for that.
//
// This is the Go port of
// org.apache.lucene.search.uhighlight.PostingsOffsetStrategy from Apache
// Lucene 10.5.0.
type PostingsOffsetStrategy struct{ BaseFieldOffsetStrategy }

// NewPostingsOffsetStrategy renders `PostingsOffsetStrategy(UHComponents
// components)` (PostingsOffsetStrategy.java:31).
func NewPostingsOffsetStrategy(components *UHComponents) *PostingsOffsetStrategy {
	return &PostingsOffsetStrategy{BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(components)}
}

// GetOffsetsEnum renders
// PostingsOffsetStrategy.getOffsetsEnum(LeafReader, int, String)
// (PostingsOffsetStrategy.java:36).
func (s *PostingsOffsetStrategy) GetOffsetsEnum(reader index.LeafReader, docID int, _ string) (OffsetsEnum, error) {
	return s.CreateOffsetsEnumFromReader(reader, docID)
}

// GetOffsetSource renders PostingsOffsetStrategy.getOffsetSource()
// (PostingsOffsetStrategy.java:42).
func (s *PostingsOffsetStrategy) GetOffsetSource() OffsetSource { return OffsetSourcePostings }

var _ FieldOffsetStrategy = (*PostingsOffsetStrategy)(nil)
