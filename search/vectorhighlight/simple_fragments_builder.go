package vectorhighlight

// SimpleFragmentsBuilder is a simple implementation of FragmentsBuilder.
type SimpleFragmentsBuilder struct {
	*BaseFragmentsBuilder
}

func NewSimpleFragmentsBuilder() *SimpleFragmentsBuilder {
	return &SimpleFragmentsBuilder{
		BaseFragmentsBuilder: NewBaseFragmentsBuilder(nil, nil, nil),
	}
}

func NewSimpleFragmentsBuilderWithTags(preTags, postTags []string) *SimpleFragmentsBuilder {
	return &SimpleFragmentsBuilder{
		BaseFragmentsBuilder: NewBaseFragmentsBuilder(preTags, postTags, nil),
	}
}

func NewSimpleFragmentsBuilderWithBoundaryScanner(bs BoundaryScanner) *SimpleFragmentsBuilder {
	return &SimpleFragmentsBuilder{
		BaseFragmentsBuilder: NewBaseFragmentsBuilder(nil, nil, bs),
	}
}

func NewSimpleFragmentsBuilderWithTagsAndBoundaryScanner(preTags, postTags []string, bs BoundaryScanner) *SimpleFragmentsBuilder {
	return &SimpleFragmentsBuilder{
		BaseFragmentsBuilder: NewBaseFragmentsBuilder(preTags, postTags, bs),
	}
}

func init() {
	// In SimpleFragmentsBuilder, GetWeightedFragInfoList does nothing.
	// We can just leave it as nil since BaseFragmentsBuilder checks for nil.
}
