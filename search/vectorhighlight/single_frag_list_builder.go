package vectorhighlight

// SingleFragListBuilder generates one WeightedFragInfo object.
type SingleFragListBuilder struct{}

func NewSingleFragListBuilder() *SingleFragListBuilder {
	return &SingleFragListBuilder{}
}

func (s *SingleFragListBuilder) CreateFieldFragList(fieldPhraseList *FieldPhraseList, fragCharSize int) FieldFragList {
	ffl := NewSimpleFieldFragList(fragCharSize)

	wpil := make([]*WeightedPhraseInfo, 0, len(fieldPhraseList.PhraseList))
	for _, phraseInfo := range fieldPhraseList.PhraseList {
		if phraseInfo == nil {
			break
		}
		wpil = append(wpil, phraseInfo)
	}
	if len(wpil) > 0 {
		ffl.Add(0, 2147483647, wpil)
	}
	return ffl
}
