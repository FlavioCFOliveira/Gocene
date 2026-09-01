// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

import (
	"fmt"
	"strings"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	textgramsFieldName   = "textgrams"
	textFieldName        = "text"
	exactTextFieldName   = "exacttext"
	contextsFieldName    = "contexts"
	weightFieldName      = "weight"
	payloadsFieldName    = "payloads"
	defaultMinPrefixChars = 4
	defaultAllTermsReq    = true
	defaultHighlight      = true
	defaultCloseIWOnBuild = true
)

// AnalyzingInfixSuggester analyzes the input text and then suggests matches based on prefix matches to any tokens in the
// indexed text. This also highlights the tokens that match.
type AnalyzingInfixSuggester struct {
	queryAnalyzer  analysis.Analyzer
	indexAnalyzer  analysis.Analyzer
	dir            store.Directory
	minPrefixChars int
	allTermsRequired bool
	highlight      bool
	commitOnBuild  bool
	closeIWOnBuild bool

	// resultsCreator allows subclasses to override how results are created.
	resultsCreator func(searcher *search.IndexSearcher, hits *search.TopFieldDocs, num int, key string, doHighlight bool, matchedTokens []string, prefixToken string) []*LookupResult
	// fieldTypeCreator allows subclasses to override the text field type.
	fieldTypeCreator func() *index.FieldType

	writer     *index.IndexWriter
	writerLock sync.Mutex
	searcherMgr *search.SearcherManager
	smLock      sync.RWMutex
}

// NewAnalyzingInfixSuggester creates a new instance, loading from a previously built AnalyzingInfixSuggester directory, if it
// exists.
func NewAnalyzingInfixSuggester(dir store.Directory, analyzer analysis.Analyzer) (*AnalyzingInfixSuggester, error) {
	return NewAnalyzingInfixSuggesterAdvanced(
		dir,
		analyzer,
		analyzer,
		defaultMinPrefixChars,
		false,
		defaultAllTermsReq,
		defaultHighlight,
		defaultCloseIWOnBuild,
	)
}

// NewAnalyzingInfixSuggesterAdvanced creates a new instance with advanced configuration.
func NewAnalyzingInfixSuggesterAdvanced(
	dir store.Directory,
	indexAnalyzer, queryAnalyzer analysis.Analyzer,
	minPrefixChars int,
	commitOnBuild bool,
	allTermsRequired bool,
	highlight bool,
	closeIWOnBuild bool,
) (*AnalyzingInfixSuggester, error) {
	if minPrefixChars < 0 {
		return nil, fmt.Errorf("minPrefixChars must be >= 0; got: %d", minPrefixChars)
	}

	s := &AnalyzingInfixSuggester{
		queryAnalyzer:    queryAnalyzer,
		indexAnalyzer:    indexAnalyzer,
		dir:              dir,
		minPrefixChars:   minPrefixChars,
		commitOnBuild:    commitOnBuild,
		allTermsRequired: allTermsRequired,
		highlight:        highlight,
		closeIWOnBuild:   closeIWOnBuild,
	}
	s.resultsCreator = s.createResults
	s.fieldTypeCreator = s.getTextFieldType

	if index.IndexExists(dir) {
		sm, err := search.NewSearcherManager(dir, nil)
		if err != nil {
			return nil, err
		}
		s.searcherMgr = sm
	}

	return s, nil
}

func (s *AnalyzingInfixSuggester) getIndexWriterConfig(openMode index.OpenMode) *index.IndexWriterConfig {
	iwc := index.NewIndexWriterConfig(s.getGramAnalyzer())
	iwc.SetOpenMode(openMode)
	iwc.SetIndexSort(search.NewSort(search.NewSortField(weightFieldName, search.SortFieldLong, true)))
	return iwc
}

func (s *AnalyzingInfixSuggester) getGramAnalyzer() analysis.Analyzer {
	return analysis.NewAnalyzerWrapper(analysis.PerFieldReuseStrategy, func(fieldName string) analysis.Analyzer {
		return s.indexAnalyzer
	}, func(fieldName string, components analysis.TokenStreamComponents) analysis.TokenStreamComponents {
		if fieldName == textgramsFieldName && s.minPrefixChars > 0 {
			filter := analysis.NewEdgeNGramTokenFilter(components.TokenStream, 1, s.minPrefixChars, false)
			return analysis.TokenStreamComponents{
				Source: components.Source,
				Filter: filter,
			}
		}
		return components
	})
}

func (s *AnalyzingInfixSuggester) ensureOpen() error {
	s.writerLock.Lock()
	defer s.writerLock.Unlock()

	if s.writer == nil {
		var mode index.OpenMode = index.OpenModeCreate
		if index.IndexExists(s.dir) {
			mode = index.OpenModeAppend
		}

		iw, err := index.NewIndexWriter(s.dir, s.getIndexWriterConfig(mode))
		if err != nil {
			return err
		}
		s.writer = iw

		sm, err := search.NewSearcherManager(s.writer, nil)
		if err != nil {
			s.writer.Close()
			s.writer = nil
			return err
		}
		s.setSearcherManager(sm)
	}
	return nil
}

func (s *AnalyzingInfixSuggester) setSearcherManager(newSM *search.SearcherManager) {
	s.smLock.Lock()
	defer s.smLock.Unlock()
	if s.searcherMgr != nil {
		s.searcherMgr.Close()
	}
	s.searcherMgr = newSM
}

func (s *AnalyzingInfixSuggester) Build(iter InputIterator) error {
	s.writerLock.Lock()
	if s.writer != nil {
		s.writer.Close()
		s.writer = nil
	}
	s.writerLock.Unlock()

	success := false
	defer func() {
		if !success {
			s.writerLock.Lock()
			if s.writer != nil {
				s.writer.Rollback()
				s.writer.Close()
				s.writer = nil
			}
			s.writerLock.Unlock()
		}
	}()

	iw, err := index.NewIndexWriter(s.dir, s.getIndexWriterConfig(index.OpenModeCreate))
	if err != nil {
		return err
	}
	s.writerLock.Lock()
	s.writer = iw
	s.writerLock.Unlock()

	for {
		text, weight, payload, contexts, ok, err := iter.Next()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		if err := s.add(text, contexts, weight, payload); err != nil {
			return err
		}
	}

	if s.commitOnBuild || s.closeIWOnBuild {
		if err := s.Commit(); err != nil {
			return err
		}
	}

	sm, err := search.NewSearcherManager(s.writer, nil)
	if err != nil {
		return err
	}
	s.setSearcherManager(sm)

	if s.closeIWOnBuild {
		s.writerLock.Lock()
		s.writer.Close()
		s.writer = nil
		s.writerLock.Unlock()
	}

	success = true
	return nil
}

func (s *AnalyzingInfixSuggester) Commit() error {
	s.writerLock.Lock()
	defer s.writerLock.Unlock()
	if s.writer == nil {
		if s.searcherMgr == nil || !s.closeIWOnBuild {
			return fmt.Errorf("cannot commit on a closed writer")
		}
		return nil
	}
	return s.writer.Commit()
}

func (s *AnalyzingInfixSuggester) add(text []byte, contexts [][]byte, weight int64, payload []byte) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}

	textString := string(text)
	doc := index.NewDocument()

	ft := s.fieldTypeCreator()
	doc.Add(index.NewField(textFieldName, textString, ft))
	if s.minPrefixChars > 0 {
		doc.Add(index.NewField(textgramsFieldName, textString, ft))
	}
	doc.Add(index.NewStringField(exactTextFieldName, textString, index.StoreNo))
	doc.Add(index.NewBinaryDocValuesField(textFieldName, text))
	doc.Add(index.NewNumericDocValuesField(weightFieldName, weight))
	if payload != nil {
		doc.Add(index.NewBinaryDocValuesField(payloadsFieldName, payload))
	}
	if contexts != nil {
		for _, ctx := range contexts {
			doc.Add(index.NewStringField(contextsFieldName, string(ctx), index.StoreNo))
			doc.Add(index.NewSortedSetDocValuesField(contextsFieldName, ctx))
		}
	}

	return s.writer.AddDocument(doc)
}

func (s *AnalyzingInfixSuggester) Update(text []byte, contexts [][]byte, weight int64, payload []byte) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}

	textString := string(text)
	doc := index.NewDocument()
	ft := s.fieldTypeCreator()
	doc.Add(index.NewField(textFieldName, textString, ft))
	if s.minPrefixChars > 0 {
		doc.Add(index.NewField(textgramsFieldName, textString, ft))
	}
	doc.Add(index.NewStringField(exactTextFieldName, textString, index.StoreNo))
	doc.Add(index.NewBinaryDocValuesField(textFieldName, text))
	doc.Add(index.NewNumericDocValuesField(weightFieldName, weight))
	if payload != nil {
		doc.Add(index.NewBinaryDocValuesField(payloadsFieldName, payload))
	}
	if contexts != nil {
		for _, ctx := range contexts {
			doc.Add(index.NewStringField(contextsFieldName, string(ctx), index.StoreNo))
			doc.Add(index.NewSortedSetDocValuesField(contextsFieldName, ctx))
		}
	}

	return s.writer.UpdateDocument(search.NewTerm(exactTextFieldName, textString), doc)
}

func (s *AnalyzingInfixSuggester) Refresh() error {
	s.smLock.RLock()
	sm := s.searcherMgr
	s.smLock.RUnlock()
	if sm == nil {
		return fmt.Errorf("suggester was not built")
	}
	if s.writer != nil {
		return sm.MaybeRefreshBlocking()
	}
	return nil
}

func (s *AnalyzingInfixSuggester) getTextFieldType() *index.FieldType {
	ft := index.NewFieldType(index.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	ft.SetOmitNorms(true)
	return ft
}

func (s *AnalyzingInfixSuggester) LookupResults(key string, contexts [][]byte, onlyMorePopular bool, num int) ([]*LookupResult, error) {
	return s.lookup(key, contexts, num, s.allTermsRequired, s.highlight)
}

func (s *AnalyzingInfixSuggester) GetCount() int64 {
	s.smLock.RLock()
	sm := s.searcherMgr
	s.smLock.RUnlock()
	if sm == nil {
		return 0
	}
	searcher, err := sm.Acquire()
	if err != nil {
		return 0
	}
	defer sm.Release(searcher)
	return int64(searcher.IndexReader().NumDocs())
}

func (s *AnalyzingInfixSuggester) lookup(key string, contexts [][]byte, num int, allTermsRequired bool, doHighlight bool) ([]*LookupResult, error) {
	s.smLock.RLock()
	sm := s.searcherMgr
	s.smLock.RUnlock()
	if sm == nil {
		return nil, fmt.Errorf("suggester was not built")
	}

	occur := search.OccurShould
	if allTermsRequired {
		occur = search.OccurMust
	}

	var matchedTokens []string
	var prefixToken string

	ts := s.queryAnalyzer.TokenStream("", key)
	defer ts.Close()

	termAtt := ts.CharTermAttribute()
	offsetAtt := ts.OffsetAttribute()

	var lastToken string
	bq := search.NewBooleanQuery()
	maxEndOffset := -1

	for ts.IncrementToken() {
		if lastToken != "" {
			matchedTokens = append(matchedTokens, lastToken)
			bq.Add(search.NewTermQuery(search.NewTerm(textFieldName, lastToken)), occur)
		}
		lastToken = termAtt.String()
		if lastToken != "" {
			maxEndOffset = max(maxEndOffset, offsetAtt.EndOffset())
		}
	}
	ts.End()

	if lastToken != "" {
		var lastQuery search.Query
		if maxEndOffset == offsetAtt.EndOffset() {
			lastQuery = s.getLastTokenQuery(lastToken)
			prefixToken = lastToken
		} else {
			matchedTokens = append(matchedTokens, lastToken)
			lastQuery = search.NewTermQuery(search.NewTerm(textFieldName, lastToken))
		}
		if lastQuery != nil {
			bq.Add(lastQuery, occur)
		}
	}

	if len(contexts) > 0 {
		ctxQuery := search.NewBooleanQuery()
		for _, ctx := range contexts {
			ctxQuery.Add(search.NewTermQuery(search.NewTerm(contextsFieldName, string(ctx))), search.OccurShould)
		}

		if !allTermsRequired {
			mainQuery := search.NewBooleanQuery()
			mainQuery.Add(bq, search.OccurMust)
			mainQuery.Add(ctxQuery, search.OccurMust)
			bq = mainQuery
		} else {
			bq.Add(ctxQuery, search.OccurMust)
		}
	}

	finalQuery := bq.Build()

	searcher, err := sm.Acquire()
	if err != nil {
		return nil, err
	}
	defer sm.Release(searcher)

	sort := search.NewSort(search.NewSortField(weightFieldName, search.SortFieldLong, true))
	hits, err := searcher.Search(finalQuery, sort, num)
	if err != nil {
		return nil, err
	}

	return s.resultsCreator(searcher, hits, num, key, doHighlight, matchedTokens, prefixToken), nil
}

func (s *AnalyzingInfixSuggester) getLastTokenQuery(token string) search.Query {
	if len(token) < s.minPrefixChars {
		return search.NewTermQuery(search.NewTerm(textgramsFieldName, token))
	}
	return search.NewPrefixQuery(search.NewTerm(textFieldName, token))
}

func (s *AnalyzingInfixSuggester) createResults(searcher *search.IndexSearcher, hits *search.TopFieldDocs, num int, key string, doHighlight bool, matchedTokens []string, prefixToken string) []*LookupResult {
	results := make([]*LookupResult, 0, len(hits.ScoreDocs))

	textDV := searcher.IndexReader().MultiDocValues().BinaryValues(textFieldName)
	payloadsDV := searcher.IndexReader().MultiDocValues().BinaryValues(payloadsFieldName)

	for _, sd := range hits.ScoreDocs {
		textDV.Advance(sd.Doc)
		text := string(textDV.BinaryValue())
		weight := sd.Fields[0].(int64)

		var payload []byte
		if payloadsDV != nil {
			if payloadsDV.Advance(sd.Doc) == sd.Doc {
				payload = append([]byte(nil), payloadsDV.BinaryValue()...)
			}
		}

		var contexts [][]byte
		leaves := searcher.IndexReader().Leaves()
		segment := search.ReaderUtilSubIndex(sd.Doc, leaves)
		contextsDV := leaves[segment].Reader().SortedSetDocValues(contextsFieldName)
		if contextsDV != nil {
			targetDocID := sd.Doc - leaves[segment].DocBase
			if contextsDV.Advance(targetDocID) == targetDocID {
				for i := 0; i < contextsDV.DocValueCount(); i++ {
					contexts = append(contexts, append([]byte(nil), contextsDV.LookupOrd(contextsDV.NextOrd())...))
				}
			}
		}

		var resultKey string
		if doHighlight {
			resultKey = s.highlight(text, matchedTokens, prefixToken)
		} else {
			resultKey = text
		}

		results = append(results, &LookupResult{
			Key:      resultKey,
			Value:    weight,
			Payload:  payload,
			Contexts: contexts,
		})
	}

	return results
}

func (s *AnalyzingInfixSuggester) highlight(text string, matchedTokens []string, prefixToken string) string {
	ts := s.queryAnalyzer.TokenStream("text", text)
	defer ts.Close()

	termAtt := ts.CharTermAttribute()
	offsetAtt := ts.OffsetAttribute()

	var sb strings.Builder
	upto := 0

	for ts.IncrementToken() {
		token := termAtt.String()
		startOffset := offsetAtt.StartOffset()
		endOffset := offsetAtt.EndOffset()

		if upto < startOffset {
			sb.WriteString(text[upto:startOffset])
			upto = startOffset
		} else if upto > startOffset {
			continue
		}

		isMatch := false
		for _, mt := range matchedTokens {
			if mt == token {
				isMatch = true
				break
			}
		}

		if isMatch {
			sb.WriteString("<b>")
			sb.WriteString(text[startOffset:endOffset])
			sb.WriteString("</b>")
			upto = endOffset
		} else if prefixToken != "" && strings.HasPrefix(token, prefixToken) {
			sb.WriteString("<b>")
			if len(prefixToken) < (endOffset - startOffset) {
				sb.WriteString(text[startOffset : startOffset+len(prefixToken)])
				sb.WriteString("</b>")
				sb.WriteString(text[startOffset+len(prefixToken) : endOffset])
			} else {
				sb.WriteString(text[startOffset:endOffset])
				sb.WriteString("</b>")
			}
			upto = endOffset
		}
	}
	ts.End()

	if upto < len(text) {
		sb.WriteString(text[upto:])
	}

	return sb.String()
}

func (s *AnalyzingInfixSuggester) Close() error {
	s.smLock.Lock()
	if s.searcherMgr != nil {
		s.searcherMgr.Close()
		s.searcherMgr = nil
	}
	s.smLock.Unlock()

	s.writerLock.Lock()
	if s.writer != nil {
		s.writer.Close()
		s.writer = nil
	}
	s.writerLock.Unlock()

	if s.dir != nil {
		s.dir.Close()
	}
	return nil
}
