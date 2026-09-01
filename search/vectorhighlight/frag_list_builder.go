package vectorhighlight

// FragListBuilder is an interface for FieldFragList builder classes.
type FragListBuilder interface {
	CreateFieldFragList(fieldPhraseList *FieldPhraseList, fragCharSize int) FieldFragList
}
