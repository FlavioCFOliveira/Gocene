package uhighlight

import "github.com/FlavioCFOliveira/Gocene/index"

// MultiFieldsOffsetStrategy is a FieldOffsetStrategy that combines offsets
// from multiple fields. It is used to highlight a single field based on
// matches from multiple fields.
//
// This is the Go port of
// org.apache.lucene.search.uhighlight.MultiFieldsOffsetStrategy from Apache
// Lucene 10.5.0.
type MultiFieldsOffsetStrategy struct {
	BaseFieldOffsetStrategy
	fieldsOffsetStrategies []FieldOffsetStrategy
}

// NewMultiFieldsOffsetStrategy renders
// `MultiFieldsOffsetStrategy(List<FieldOffsetStrategy>
// fieldsOffsetStrategies)` (MultiFieldsOffsetStrategy.java:34), which passes
// null components up to FieldOffsetStrategy.
func NewMultiFieldsOffsetStrategy(fieldsOffsetStrategies []FieldOffsetStrategy) *MultiFieldsOffsetStrategy {
	return &MultiFieldsOffsetStrategy{
		BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(nil),
		fieldsOffsetStrategies:  fieldsOffsetStrategies,
	}
}

// Field renders MultiFieldsOffsetStrategy.getField()
// (MultiFieldsOffsetStrategy.java:39), which throws IllegalStateException
// because this strategy spans several fields.
func (s *MultiFieldsOffsetStrategy) Field() string {
	panic("MultiFieldsOffsetStrategy does not have a single field.")
}

// GetOffsetSource renders MultiFieldsOffsetStrategy.getOffsetSource()
// (MultiFieldsOffsetStrategy.java:44).
func (s *MultiFieldsOffsetStrategy) GetOffsetSource() OffsetSource {
	// TODO (from Java): what should be returned here as offset source?
	return s.fieldsOffsetStrategies[0].GetOffsetSource()
}

// GetOffsetsEnum renders
// MultiFieldsOffsetStrategy.getOffsetsEnum(LeafReader, int, String)
// (MultiFieldsOffsetStrategy.java:50).
func (s *MultiFieldsOffsetStrategy) GetOffsetsEnum(reader index.LeafReader, docID int, content string) (OffsetsEnum, error) {
	fieldsOffsetsEnums := make([]OffsetsEnum, 0, len(s.fieldsOffsetStrategies))
	for _, fieldOffsetStrategy := range s.fieldsOffsetStrategies {
		offsetsEnum, err := fieldOffsetStrategy.GetOffsetsEnum(reader, docID, content)
		if err != nil {
			return nil, err
		}
		if offsetsEnum != OffsetsEnumEMPTY {
			fieldsOffsetsEnums = append(fieldsOffsetsEnums, offsetsEnum)
		}
	}
	return NewMultiOffsetsEnum(fieldsOffsetsEnums)
}

var _ FieldOffsetStrategy = (*MultiFieldsOffsetStrategy)(nil)
