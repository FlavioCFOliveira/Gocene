package vectorhighlight

type WeightedFragListBuilder struct {
	*BaseFragListBuilder
}

func NewWeightedFragListBuilder() *WeightedFragListBuilder {
	return &WeightedFragListBuilder{
		BaseFragListBuilder: NewBaseFragListBuilder(MarginDefault),
	}
}

func NewWeightedFragListBuilderWithMargin(margin int) *WeightedFragListBuilder {
	return &WeightedFragListBuilder{
		BaseFragListBuilder: NewBaseFragListBuilder(margin),
	}
}

func (w *WeightedFragListBuilder) CreateFieldFragList(fieldPhraseList *FieldPhraseList, fragCharSize int) FieldFragList {
	return w.createFieldFragList(
		fieldPhraseList, NewWeightedFieldFragList(fragCharSize), fragCharSize)
}
