package uhighlight

import "github.com/FlavioCFOliveira/Gocene/index"

// PostingsWithTermVectorsOffsetStrategy is like PostingsOffsetStrategy but
// also uses term vectors (only terms needed) for multi-term queries.
//
// This is the Go port of
// org.apache.lucene.search.uhighlight.PostingsWithTermVectorsOffsetStrategy
// from Apache Lucene 10.5.0.
type PostingsWithTermVectorsOffsetStrategy struct{ BaseFieldOffsetStrategy }

// NewPostingsWithTermVectorsOffsetStrategy renders
// `PostingsWithTermVectorsOffsetStrategy(UHComponents components)`
// (PostingsWithTermVectorsOffsetStrategy.java:31).
func NewPostingsWithTermVectorsOffsetStrategy(components *UHComponents) *PostingsWithTermVectorsOffsetStrategy {
	return &PostingsWithTermVectorsOffsetStrategy{BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(components)}
}

// GetOffsetsEnum renders
// PostingsWithTermVectorsOffsetStrategy.getOffsetsEnum(LeafReader, int,
// String) (PostingsWithTermVectorsOffsetStrategy.java:36).
func (s *PostingsWithTermVectorsOffsetStrategy) GetOffsetsEnum(leafReader index.LeafReader, docID int, _ string) (OffsetsEnum, error) {
	termVectors, err := leafReader.TermVectors()
	if err != nil {
		return nil, err
	}
	docTerms, err := termVectors.GetField(docID, s.Field())
	if err != nil {
		return nil, err
	}
	if docTerms == nil {
		return OffsetsEnumEMPTY, nil
	}
	filtered := NewTermVectorFilteredLeafReader(leafReader, docTerms, s.Field())

	return s.CreateOffsetsEnumFromReader(filtered, docID)
}

// GetOffsetSource renders
// PostingsWithTermVectorsOffsetStrategy.getOffsetSource()
// (PostingsWithTermVectorsOffsetStrategy.java:47).
func (s *PostingsWithTermVectorsOffsetStrategy) GetOffsetSource() OffsetSource {
	return OffsetSourcePostingsWithTermVectors
}

var _ FieldOffsetStrategy = (*PostingsWithTermVectorsOffsetStrategy)(nil)
