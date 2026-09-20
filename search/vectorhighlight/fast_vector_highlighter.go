package vectorhighlight

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/highlight"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

const (
	DefaultPhraseHighlight = true
	DefaultFieldMatch      = true
)

// FastVectorHighlighter is another highlighter implementation.
type FastVectorHighlighter struct {
	phraseHighlight  bool
	fieldMatch       bool
	fragListBuilder  FragListBuilder
	fragmentsBuilder FragmentsBuilder
	phraseLimit      int
}

func NewFastVectorHighlighter() *FastVectorHighlighter {
	return NewFastVectorHighlighterWithParams(DefaultPhraseHighlight, DefaultFieldMatch)
}

func NewFastVectorHighlighterWithParams(phraseHighlight, fieldMatch bool) *FastVectorHighlighter {
	return NewFastVectorHighlighterWithBuilders(
		phraseHighlight,
		fieldMatch,
		NewSimpleFragListBuilder(),
		NewScoreOrderFragmentsBuilder(),
	)
}

func NewFastVectorHighlighterWithBuilders(
	phraseHighlight, fieldMatch bool,
	fragListBuilder FragListBuilder,
	fragmentsBuilder FragmentsBuilder) *FastVectorHighlighter {
	return &FastVectorHighlighter{
		phraseHighlight:  phraseHighlight,
		fieldMatch:       fieldMatch,
		fragListBuilder:  fragListBuilder,
		fragmentsBuilder: fragmentsBuilder,
		phraseLimit:      2147483647,
	}
}

func (f *FastVectorHighlighter) GetFieldQuery(query search.Query) *FieldQuery {
	q, err := f.GetFieldQueryWithReader(query, nil)
	if err != nil {
		panic(err)
	}
	return q
}

func (f *FastVectorHighlighter) GetFieldQueryWithReader(query search.Query, reader index.IndexReader) (*FieldQuery, error) {
	return NewFieldQuery(query, reader, f.phraseHighlight, f.fieldMatch)
}

func (f *FastVectorHighlighter) GetBestFragment(
	fieldQuery *FieldQuery,
	reader index.IndexReader,
	docID int,
	fieldName string,
	fragCharSize int) (string, error) {
	fieldFragList, err := f.getFieldFragList(f.fragListBuilder, fieldQuery, reader, docID, fieldName, fragCharSize)
	if err != nil {
		return "", err
	}
	return f.fragmentsBuilder.CreateFragment(reader, docID, fieldName, fieldFragList)
}

func (f *FastVectorHighlighter) GetBestFragments(
	fieldQuery *FieldQuery,
	reader index.IndexReader,
	docID int,
	fieldName string,
	fragCharSize int,
	maxNumFragments int) ([]string, error) {
	fieldFragList, err := f.getFieldFragList(f.fragListBuilder, fieldQuery, reader, docID, fieldName, fragCharSize)
	if err != nil {
		return nil, err
	}
	return f.fragmentsBuilder.CreateFragments(reader, docID, fieldName, fieldFragList, maxNumFragments)
}

func (f *FastVectorHighlighter) GetBestFragmentWithTags(
	fieldQuery *FieldQuery,
	reader index.IndexReader,
	docID int,
	fieldName string,
	fragCharSize int,
	fragListBuilder FragListBuilder,
	fragmentsBuilder FragmentsBuilder,
	preTags, postTags []string,
	encoder highlight.Encoder) (string, error) {
	fieldFragList, err := f.getFieldFragList(fragListBuilder, fieldQuery, reader, docID, fieldName, fragCharSize)
	if err != nil {
		return "", err
	}
	return fragmentsBuilder.CreateFragmentWithTags(reader, docID, fieldName, fieldFragList, preTags, postTags, encoder)
}

func (f *FastVectorHighlighter) GetBestFragmentsWithTags(
	fieldQuery *FieldQuery,
	reader index.IndexReader,
	docID int,
	fieldName string,
	fragCharSize int,
	maxNumFragments int,
	fragListBuilder FragListBuilder,
	fragmentsBuilder FragmentsBuilder,
	preTags, postTags []string,
	encoder highlight.Encoder) ([]string, error) {
	fieldFragList, err := f.getFieldFragList(fragListBuilder, fieldQuery, reader, docID, fieldName, fragCharSize)
	if err != nil {
		return nil, err
	}
	return fragmentsBuilder.CreateFragmentsWithTags(reader, docID, fieldName, fieldFragList, maxNumFragments, preTags, postTags, encoder)
}

func (f *FastVectorHighlighter) GetBestFragmentsMultiField(
	fieldQuery *FieldQuery,
	reader index.IndexReader,
	docID int,
	storedField string,
	matchedFields map[string]struct{},
	fragCharSize int,
	maxNumFragments int,
	fragListBuilder FragListBuilder,
	fragmentsBuilder FragmentsBuilder,
	preTags, postTags []string,
	encoder highlight.Encoder) ([]string, error) {
	fieldFragList, err := f.getFieldFragListMultiField(fragListBuilder, fieldQuery, reader, docID, matchedFields, fragCharSize)
	if err != nil {
		return nil, err
	}
	return fragmentsBuilder.CreateFragmentsWithTags(reader, docID, storedField, fieldFragList, maxNumFragments, preTags, postTags, encoder)
}

func (f *FastVectorHighlighter) getFieldFragList(
	fragListBuilder FragListBuilder,
	fieldQuery *FieldQuery,
	reader index.IndexReader,
	docID int,
	matchedField string,
	fragCharSize int) (FieldFragList, error) {
	fieldTermStack, err := NewFieldTermStack(reader, docID, matchedField, fieldQuery)
	if err != nil {
		return nil, err
	}
	fieldPhraseList := NewFieldPhraseListWithLimit(fieldTermStack, fieldQuery, f.phraseLimit)
	return fragListBuilder.CreateFieldFragList(fieldPhraseList, fragCharSize), nil
}

func (f *FastVectorHighlighter) getFieldFragListMultiField(
	fragListBuilder FragListBuilder,
	fieldQuery *FieldQuery,
	reader index.IndexReader,
	docID int,
	matchedFields map[string]struct{},
	fragCharSize int) (FieldFragList, error) {
	if len(matchedFields) == 0 {
		return nil, fmt.Errorf("matchedFields must contain at least one field name")
	}
	toMerge := make([]*FieldPhraseList, 0, len(matchedFields))
	for field := range matchedFields {
		stack, err := NewFieldTermStack(reader, docID, field, fieldQuery)
		if err != nil {
			return nil, err
		}
		toMerge = append(toMerge, NewFieldPhraseListWithLimit(stack, fieldQuery, f.phraseLimit))
	}
	return fragListBuilder.CreateFieldFragList(NewFieldPhraseListFromMerge(toMerge), fragCharSize), nil
}

func (f *FastVectorHighlighter) IsPhraseHighlight() bool {
	return f.phraseHighlight
}

func (f *FastVectorHighlighter) IsFieldMatch() bool {
	return f.fieldMatch
}

func (f *FastVectorHighlighter) GetPhraseLimit() int {
	return f.phraseLimit
}

func (f *FastVectorHighlighter) SetPhraseLimit(phraseLimit int) {
	f.phraseLimit = phraseLimit
}
