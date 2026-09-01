package vectorhighlight

type SimpleFragListBuilder struct {
	*BaseFragListBuilder
}

func NewSimpleFragListBuilder() *SimpleFragListBuilder {
	return &SimpleFragListBuilder{
		BaseFragListBuilder: NewBaseFragListBuilder(MarginDefault),
	}
}

func NewSimpleFragListBuilderWithMargin(margin int) *SimpleFragListBuilder {
	return &SimpleFragListBuilder{
		BaseFragListBuilder: NewBaseFragListBuilder(margin),
	}
}

func (s *SimpleFragListBuilder) CreateFieldFragList(fieldPhraseList *FieldPhraseList, fragCharSize int) FieldFragList {
	return s.createFieldFragList(
		fieldPhraseList, NewSimpleFieldFragList(fragCharSize), fragCharSize)
}
