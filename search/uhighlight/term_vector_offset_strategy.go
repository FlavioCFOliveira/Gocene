package uhighlight

import (
	"github.com/FlavioCFOliveira/Gocene/highlight"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// TermVectorOffsetStrategy uses term vectors that contain offsets.
//
// This is the Go port of
// org.apache.lucene.search.uhighlight.TermVectorOffsetStrategy from Apache
// Lucene 10.5.0.
type TermVectorOffsetStrategy struct{ BaseFieldOffsetStrategy }

// NewTermVectorOffsetStrategy renders
// `TermVectorOffsetStrategy(UHComponents components)`
// (TermVectorOffsetStrategy.java:33).
func NewTermVectorOffsetStrategy(components *UHComponents) *TermVectorOffsetStrategy {
	return &TermVectorOffsetStrategy{BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(components)}
}

// GetOffsetSource renders TermVectorOffsetStrategy.getOffsetSource()
// (TermVectorOffsetStrategy.java:38).
func (s *TermVectorOffsetStrategy) GetOffsetSource() OffsetSource { return OffsetSourceTermVectors }

// GetOffsetsEnum renders
// TermVectorOffsetStrategy.getOffsetsEnum(LeafReader, int, String)
// (TermVectorOffsetStrategy.java:43).
func (s *TermVectorOffsetStrategy) GetOffsetsEnum(reader index.LeafReader, docID int, _ string) (OffsetsEnum, error) {
	termVectors, err := reader.TermVectors()
	if err != nil {
		return nil, err
	}
	tvTerms, err := termVectors.GetField(docID, s.Field())
	if err != nil {
		return nil, err
	}
	if tvTerms == nil {
		return OffsetsEnumEMPTY, nil
	}

	singleDocReader := highlight.NewTermVectorLeafReader(s.Field(), tvTerms)
	return s.CreateOffsetsEnumFromReader(
		NewOverlaySingleDocTermsLeafReader(reader, singleDocReader, s.Field(), docID), docID)
}

var _ FieldOffsetStrategy = (*TermVectorOffsetStrategy)(nil)
