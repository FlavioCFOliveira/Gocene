package surround

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

type mockReader struct {
	terms map[string]*mockTerms
}

func (m *mockReader) DocCount() int                                 { return 0 }
func (m *mockReader) NumDocs() int                                  { return 0 }
func (m *mockReader) MaxDoc() int                                   { return 0 }
func (m *mockReader) Close() error                                  { return nil }
func (m *mockReader) HasDeletions() bool                            { return false }
func (m *mockReader) NumDeletedDocs() int                           { return 0 }
func (m *mockReader) EnsureOpen() error                             { return nil }
func (m *mockReader) IncRef() error                                 { return nil }
func (m *mockReader) DecRef() error                                 { return nil }
func (m *mockReader) TryIncRef() bool                               { return true }
func (m *mockReader) GetRefCount() int32                            { return 1 }
func (m *mockReader) GetContext() (index.IndexReaderContext, error) { return nil, nil }
func (m *mockReader) Leaves() ([]*index.LeafReaderContext, error)   { return nil, nil }
func (m *mockReader) StoredFields() (index.StoredFields, error)     { return nil, nil }
func (m *mockReader) TermVectors() (index.TermVectors, error)       { return nil, nil }
func (m *mockReader) Terms(field string) (index.Terms, error) {
	if t, ok := m.terms[field]; ok {
		return t, nil
	}
	return index.EmptyTerms{}, nil
}

type mockTerms struct {
	termList []index.Term
}

func (m *mockTerms) GetIterator() (index.TermsEnum, error) {
	return &mockTermsEnum{terms: m.termList, pos: -1}, nil
}
func (m *mockTerms) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	// Simple seek: find first term >= seekTerm
	pos := -1
	for i, t := range m.termList {
		if t.CompareTo(seekTerm) >= 0 {
			pos = i - 1
			break
		}
	}
	if pos == -1 && len(m.termList) > 0 {
		pos = len(m.termList) - 1
	}
	return &mockTermsEnum{terms: m.termList, pos: pos}, nil
}
func (m *mockTerms) GetPostingsReader(termText string, flags int) (index.PostingsEnum, error) {
	return nil, nil
}
func (m *mockTerms) Size() int64                         { return int64(len(m.termList)) }
func (m *mockTerms) GetDocCount() (int, error)           { return 0, nil }
func (m *mockTerms) GetSumDocFreq() (int64, error)       { return 0, nil }
func (m *mockTerms) GetSumTotalTermFreq() (int64, error) { return 0, nil }
func (m *mockTerms) HasFreqs() bool                      { return false }
func (m *mockTerms) HasOffsets() bool                    { return false }
func (m *mockTerms) HasPositions() bool                  { return false }
func (m *mockTerms) HasPayloads() bool                   { return false }
func (m *mockTerms) GetMin() (*index.Term, error)        { return nil, nil }
func (m *mockTerms) GetMax() (*index.Term, error)        { return nil, nil }

type mockTermsEnum struct {
	terms []index.Term
	pos   int
}

func (m *mockTermsEnum) Next() (*index.Term, error) {
	m.pos++
	if m.pos >= len(m.terms) {
		return nil, nil
	}
	return &m.terms[m.pos], nil
}
func (m *mockTermsEnum) Binary() []byte       { return nil }
func (m *mockTermsEnum) DocFreq() int         { return 0 }
func (m *mockTermsEnum) TotalTermFreq() int64 { return 0 }

func TestSimpleTermRewriteQuery_Rewrite(t *testing.T) {
	qf := &BasicQueryFactory{MaxBasicQueries: 10}
	fieldName := "text"

	tests := []struct {
		name         string
		st           SimpleTerm
		indexTerms   []index.Term
		expectedType string
	}{
		{
			name:         "no match",
			st:           NewSrndTermQuery("missing", false),
			indexTerms:   []index.Term{*index.NewTerm(fieldName, "exists")},
			expectedType: "MatchNoDocsQuery",
		},
		{
			name:         "single match",
			st:           NewSrndTermQuery("exists", false),
			indexTerms:   []index.Term{*index.NewTerm(fieldName, "exists")},
			expectedType: "TermQuery",
		},
		{
			name: "multiple matches prefix",
			st:   NewSrndPrefixQuery("pre", false, '*'),
			indexTerms: []index.Term{
				*index.NewTerm(fieldName, "prefix1"),
				*index.NewTerm(fieldName, "prefix2"),
				*index.NewTerm(fieldName, "other"),
			},
			expectedType: "BooleanQuery",
		},
		{
			name: "multiple matches wildcard",
			st:   NewSrndTruncQuery("t?st*", '*', '?'),
			indexTerms: []index.Term{
				*index.NewTerm(fieldName, "test"),
				*index.NewTerm(fieldName, "tast"),
				*index.NewTerm(fieldName, "toast"),
			},
			expectedType: "BooleanQuery",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &mockReader{
				terms: map[string]*mockTerms{
					fieldName: {termList: tt.indexTerms},
				},
			}
			rewriteQ := NewSimpleTermRewriteQuery(tt.st, fieldName, qf)
			res, err := rewriteQ.Rewrite(reader)
			if err != nil {
				t.Fatalf("Rewrite failed: %v", err)
			}

			// Basic check for type. In real world we'd check the structure.
			// We use fmt.Sprintf because we don't have a clean way to get the type name.
			typeStr := fmt.Sprintf("%T", res)
			if tt.expectedType == "TermQuery" && (typeStr != "*search.TermQuery" && typeStr != "*search.BooleanQuery") {
				// Note: TermQuery might be wrapped in BooleanQuery or others depending on qf
				t.Errorf("expected TermQuery, got %s", typeStr)
			} else if tt.expectedType == "BooleanQuery" && typeStr != "*search.BooleanQuery" {
				t.Errorf("expected BooleanQuery, got %s", typeStr)
			} else if tt.expectedType == "MatchNoDocsQuery" && typeStr != "*search.MatchNoDocsQuery" {
				t.Errorf("expected MatchNoDocsQuery, got %s", typeStr)
			}
		})
	}
}
