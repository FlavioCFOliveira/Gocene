package util

// FieldInvertState aggregates per-document inversion statistics for a field.
// Mirrors org.apache.lucene.index.FieldInvertState from Apache Lucene 10.4.0.
//
// It is mutable: the inversion pipeline updates the fields directly as it
// walks each token.
type FieldInvertState struct {
	IndexCreatedVersionMajor int
	Name                     string
	IndexOptions             any // Use any to avoid importing schema/index here, or use a common type

	Position         int
	Length           int
	NumOverlap       int
	Offset           int
	MaxTermFrequency int
	UniqueTermCount  int
}

// NewFieldInvertState constructs a zero-valued FieldInvertState for the given
// name and index options.
func NewFieldInvertState(indexCreatedVersionMajor int, name string, opts any) *FieldInvertState {
	return &FieldInvertState{
		IndexCreatedVersionMajor: indexCreatedVersionMajor,
		Name:                     name,
		IndexOptions:             opts,
	}
}

// NewFieldInvertStateFull constructs a pre-populated FieldInvertState.
func NewFieldInvertStateFull(indexCreatedVersionMajor int, name string, opts any,
	position, length, numOverlap, offset, maxTermFrequency, uniqueTermCount int) *FieldInvertState {
	return &FieldInvertState{
		IndexCreatedVersionMajor: indexCreatedVersionMajor,
		Name:                     name,
		IndexOptions:             opts,
		Position:                 position,
		Length:                   length,
		NumOverlap:               numOverlap,
		Offset:                   offset,
		MaxTermFrequency:         maxTermFrequency,
		UniqueTermCount:          uniqueTermCount,
	}
}

func (s *FieldInvertState) GetName() string { return s.Name }

func (s *FieldInvertState) GetIndexOptions() any { return s.IndexOptions }

func (s *FieldInvertState) GetIndexCreatedVersionMajor() int { return s.IndexCreatedVersionMajor }

func (s *FieldInvertState) GetPosition() int { return s.Position }

func (s *FieldInvertState) SetPosition(p int) { s.Position = p }

func (s *FieldInvertState) GetLength() int { return s.Length }

func (s *FieldInvertState) SetLength(l int) { s.Length = l }

func (s *FieldInvertState) GetNumOverlap() int { return s.NumOverlap }

func (s *FieldInvertState) SetNumOverlap(n int) { s.NumOverlap = n }

func (s *FieldInvertState) GetOffset() int { return s.Offset }

func (s *FieldInvertState) SetOffset(o int) { s.Offset = o }

func (s *FieldInvertState) GetMaxTermFrequency() int { return s.MaxTermFrequency }

func (s *FieldInvertState) SetMaxTermFrequency(f int) { s.MaxTermFrequency = f }

func (s *FieldInvertState) GetUniqueTermCount() int { return s.UniqueTermCount }

func (s *FieldInvertState) SetUniqueTermCount(c int) { s.UniqueTermCount = c }
