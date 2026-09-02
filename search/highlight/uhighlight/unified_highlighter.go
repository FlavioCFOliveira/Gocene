package uhighlight

import (
	"fmt"
	"io"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

type UnifiedHighlighter struct {
	searcher       *search.IndexSearcher
	indexAnalyzer  search.Analyzer
	fieldInfos     *index.FieldInfos
	fieldMatcher   func(string) bool
	maskedFields   func(string) []string
	flags          FlagSet
	handleMultiTerm bool
	highlightStrict bool
	weightMatches   bool
	relevancySpeed  bool
	maxLength      int
	breakIterator   func() BreakIterator
	scorer         *PassageScorer
	formatter      PassageFormatter
	maxNoHighlight  int
	cacheThreshold  int
	passageSort     func([]*Passage)
}

type Builder struct {
	searcher       *search.IndexSearcher
	indexAnalyzer  search.Analyzer
	fieldMatcher   func(string) bool
	maskedFields   func(string) []string
	flags          FlagSet
	handleMultiTerm bool
	highlightStrict bool
	weightMatches   bool
	relevancySpeed  bool
	maxLength      int
	breakIterator   func() BreakIterator
	scorer         *PassageScorer
	formatter      PassageFormatter
	maxNoHighlight  int
	cacheThreshold  int
	passageSort     func([]*Passage)
}

func NewBuilder(searcher *search.IndexSearcher, analyzer search.Analyzer) *Builder {
	return &Builder{
		searcher:       searcher,
		indexAnalyzer:  analyzer,
		handleMultiTerm: true,
		highlightStrict: true,
		weightMatches:   true,
		relevancySpeed:  true,
		maxLength:      10000,
		breakIterator: func() BreakIterator {
			return NewSimpleBreakIterator("") // Content will be set in a real impl
		},
		scorer: NewPassageScorer(),
		formatter: NewDefaultPassageFormatter(),
		maxNoHighlight: -1,
		cacheThreshold: 524288,
		passageSort: func(p []*Passage) {
			sort.Slice(p, func(i, j int) bool {
				return p[i].StartOffset() < p[j].StartOffset()
			})
		},
	}
}

func (b *Builder) WithFlags(flags FlagSet) *Builder {
	b.flags = flags
	return b
}

func (b *Builder) WithMaxLength(max int) *Builder {
	b.maxLength = max
	return b
}

func (b *Builder) Build() *UnifiedHighlighter {
	return &UnifiedHighlighter{
		searcher:       b.searcher,
		indexAnalyzer:  b.indexAnalyzer,
		fieldMatcher:   b.fieldMatcher,
		maskedFields:   b.maskedFields,
		flags:          b.flags,
		handleMultiTerm: b.handleMultiTerm,
		highlightStrict: b.highlightStrict,
		weightMatches:   b.weightMatches,
		relevancySpeed:  b.relevancySpeed,
		maxLength:      b.maxLength,
		breakIterator:   b.breakIterator,
		scorer:         b.scorer,
		formatter:      b.formatter,
		maxNoHighlight: b.maxNoHighlight,
		cacheThreshold:  b.cacheThreshold,
		passageSort:     b.passageSort,
	}
}

func (uh *UnifiedHighlighter) Highlight(field string, query search.Query, topDocs search.TopDocs) ([]string, error) {
	return uh.HighlightFields([]string{field}, query, topDocs, []int{1})
}

func (uh *UnifiedHighlighter) HighlightFields(fields []string, query search.Query, topDocs search.TopDocs, maxPassages []int) (map[string][]string, error) {
	docIDs := make([]int, len(topDocs.ScoreDocs))
	for i, sd := range topDocs.ScoreDocs {
		docIDs[i] = sd.Doc
	}
	return uh.highlightFieldsInternal(fields, query, docIDs, maxPassages)
}

func (uh *UnifiedHighlighter) highlightFieldsInternal(fields []string, query search.Query, docIDs []int, maxPassages []int) (map[string][]string, error) {
	results := make(map[string][]string)
	for i, field := range fields {
		fh := uh.getFieldHighlighter(field, query, maxPassages[i])

		snippets := make([]string, len(docIDs))
		for j, docID := range docIDs {
			// This requires a LeafReader. We'll use the searcher's reader.
			reader := uh.searcher.IndexReader()
			// Simplified: get leaf reader for docID
			leaf := reader.Leaves()[0].Reader() // Very simplified

			// In real Lucene, we'd fetch the stored field first
			content := "sample content" // Simplified

			res := fh.HighlightFieldForDoc(leaf, docID, content)
			if res != nil {
				snippets[j] = fmt.Sprintf("%v", res)
			}
		}
		results[field] = snippets
	}
	return results, nil
}

func (uh *UnifiedHighlighter) getFieldHighlighter(field string, query search.Query, maxPassages int) *FieldHighlighter {
	// Simplified: create a simple strategy
	strategy := NewNoOpOffsetStrategy()

	return NewFieldHighlighter(
		field,
		strategy,
		uh.breakIterator(),
		uh.scorer,
		maxPassages,
		uh.maxNoHighlight,
		uh.formatter,
		uh.passageSort,
	)
}

func (uh *UnifiedHighlighter) HighlightWithoutSearcher(field string, query search.Query, content string, maxPassages int) interface{} {
	// Implement as in Java
	return nil
}
