// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analyzing

import (
	"fmt"
	"strings"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/suggest"
)

const (
	textgramsFieldName    = "textgrams"
	textFieldName         = "text"
	exactTextFieldName    = "exacttext"
	contextsFieldName     = "contexts"
	weightFieldName       = "weight"
	payloadsFieldName     = "payloads"
	defaultMinPrefixChars = 4
	defaultAllTermsReq    = true
	defaultHighlight      = true
	defaultCloseIWOnBuild = true
)

// AnalyzingInfixSuggester analyzes the input text and then suggests matches based on prefix matches to any tokens in the
// indexed text. This also highlights the tokens that match.
type AnalyzingInfixSuggester struct {
	queryAnalyzer    analysis.Analyzer
	indexAnalyzer    analysis.Analyzer
	dir              store.Directory
	minPrefixChars   int
	allTermsRequired bool
	highlight        bool
	commitOnBuild    bool
	closeIWOnBuild   bool

	// resultsCreator allows subclasses to override how results are created.
	resultsCreator func(searcher *search.IndexSearcher, hits *search.TopFieldDocs, num int, key string, doHighlight bool, matchedTokens []string, prefixToken string) []*suggest.LookupResult
	// fieldTypeCreator allows subclasses to override the text field type.
	fieldTypeCreator func() *document.FieldType

	writer      *index.IndexWriter
	writerLock  sync.Mutex
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

	exists, err := index.IndexExists(dir)
	if err != nil {
		return nil, err
	}
	if exists {
		sm, err := search.NewSearcherManagerFromDir(dir, nil)
		if err != nil {
			return nil, err
		}
		s.searcherMgr = sm
	}

	return s, nil
}

func (s *AnalyzingInfixSuggester) getIndexWriterConfig(openMode index.OpenMode) *index.IndexWriterConfig {
	iwc := index.NewIndexWriterConfigWithAnalyzer(s.getGramAnalyzer())
	iwc.SetOpenMode(openMode)
	iwc.SetIndexSort(search.NewSort(search.NewSortFieldWithReverse(weightFieldName, spi.SortFieldTypeLong, true)))
	return iwc
}

func (s *AnalyzingInfixSuggester) getGramAnalyzer() analysis.Analyzer {
	w := analysis.NewAnalyzerWrapper(func(string) analysis.Analyzer {
		return s.indexAnalyzer
	})
	w.WrapTokenStream = func(fieldName string, in analysis.TokenStream) analysis.TokenStream {
		if fieldName == textgramsFieldName && s.minPrefixChars > 0 {
			filter, err := analysis.NewEdgeNGramTokenFilter(in, 1, s.minPrefixChars, false)
			if err != nil {
				// Unreachable: NewEdgeNGramTokenFilter only rejects minGram < 1
				// or minGram > maxGram, and this branch fixes minGram at 1 with
				// maxGram = minPrefixChars > 0.
				return in
			}
			return filter
		}
		return in
	}
	return w
}

func (s *AnalyzingInfixSuggester) ensureOpen() error {
	s.writerLock.Lock()
	defer s.writerLock.Unlock()

	if s.writer == nil {
		var mode index.OpenMode = index.Create
		exists, err := index.IndexExists(s.dir)
		if err != nil {
			return err
		}
		if exists {
			mode = index.Append
		}

		iw, err := index.NewIndexWriter(s.dir, s.getIndexWriterConfig(mode))
		if err != nil {
			return err
		}
		s.writer = iw

		sm, err := search.NewSearcherManager(s.writer, nil)
		if err != nil {
			closeErr := s.writer.Close()
			s.writer = nil
			if closeErr != nil {
				return fmt.Errorf("%w (closing writer: %v)", err, closeErr)
			}
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

func (s *AnalyzingInfixSuggester) Build(iter suggest.InputIterator) error {
	s.writerLock.Lock()
	if s.writer != nil {
		if err := s.writer.Close(); err != nil {
			s.writer = nil
			s.writerLock.Unlock()
			return err
		}
		s.writer = nil
	}
	s.writerLock.Unlock()

	success := false
	defer func() {
		if !success {
			s.writerLock.Lock()
			if s.writer != nil {
				_ = s.writer.Rollback()
				_ = s.writer.Close()
				s.writer = nil
			}
			s.writerLock.Unlock()
		}
	}()

	iw, err := index.NewIndexWriter(s.dir, s.getIndexWriterConfig(index.Create))
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
		closeErr := s.writer.Close()
		s.writer = nil
		s.writerLock.Unlock()
		if closeErr != nil {
			return closeErr
		}
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
	_, err := s.writer.Commit()
	return err
}

func (s *AnalyzingInfixSuggester) add(text []byte, contexts [][]byte, weight int64, payload []byte) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}

	textString := string(text)
	doc := document.NewDocument()

	ft := s.fieldTypeCreator()
	if err := addDocFields(doc, s.minPrefixChars > 0, textString, text, ft, weight, payload, contexts); err != nil {
		return err
	}

	_, err := s.writer.AddDocument(doc)
	return err
}

func (s *AnalyzingInfixSuggester) Update(text []byte, contexts [][]byte, weight int64, payload []byte) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}

	textString := string(text)
	doc := document.NewDocument()

	ft := s.fieldTypeCreator()
	if err := addDocFields(doc, s.minPrefixChars > 0, textString, text, ft, weight, payload, contexts); err != nil {
		return err
	}

	_, err := s.writer.UpdateDocument(index.NewTerm(exactTextFieldName, textString), doc)
	return err
}

func (s *AnalyzingInfixSuggester) Refresh() error {
	s.smLock.RLock()
	sm := s.searcherMgr
	s.smLock.RUnlock()
	if sm == nil {
		return fmt.Errorf("suggester was not built")
	}
	if s.writer != nil {
		_, err := sm.MaybeRefresh()
		return err
	}
	return nil
}

func (s *AnalyzingInfixSuggester) getTextFieldType() *document.FieldType {
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(spi.IndexOptionsDocs)
	ft.SetOmitNorms(true)
	return ft
}

// addDocFields builds the document body shared by add() and Update(), mirroring
// the field set AnalyzingInfixSuggester indexes for each suggestion.
func addDocFields(
	doc *document.Document,
	withGrams bool,
	textString string,
	text []byte,
	ft *document.FieldType,
	weight int64,
	payload []byte,
	contexts [][]byte,
) error {
	textField, err := document.NewField(textFieldName, textString, ft)
	if err != nil {
		return err
	}
	doc.Add(textField)
	if withGrams {
		gramField, err := document.NewField(textgramsFieldName, textString, ft)
		if err != nil {
			return err
		}
		doc.Add(gramField)
	}
	exact, err := document.NewStringField(exactTextFieldName, textString, false)
	if err != nil {
		return err
	}
	doc.Add(exact)
	textDV, err := document.NewBinaryDocValuesField(textFieldName, text)
	if err != nil {
		return err
	}
	doc.Add(textDV)
	weightDV, err := document.NewNumericDocValuesField(weightFieldName, weight)
	if err != nil {
		return err
	}
	doc.Add(weightDV)
	if payload != nil {
		payloadDV, err := document.NewBinaryDocValuesField(payloadsFieldName, payload)
		if err != nil {
			return err
		}
		doc.Add(payloadDV)
	}
	for _, ctx := range contexts {
		ctxField, err := document.NewStringField(contextsFieldName, string(ctx), false)
		if err != nil {
			return err
		}
		doc.Add(ctxField)
		ctxDV, err := document.NewSortedSetDocValuesField(contextsFieldName, [][]byte{ctx})
		if err != nil {
			return err
		}
		doc.Add(ctxDV)
	}
	return nil
}

func (s *AnalyzingInfixSuggester) LookupResults(key string, contexts [][]byte, onlyMorePopular bool, num int) ([]*suggest.LookupResult, error) {
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
	return int64(searcher.GetIndexReader().NumDocs())
}

func (s *AnalyzingInfixSuggester) lookup(key string, contexts [][]byte, num int, allTermsRequired bool, doHighlight bool) ([]*suggest.LookupResult, error) {
	s.smLock.RLock()
	sm := s.searcherMgr
	s.smLock.RUnlock()
	if sm == nil {
		return nil, fmt.Errorf("suggester was not built")
	}

	occur := search.SHOULD
	if allTermsRequired {
		occur = search.MUST
	}

	var matchedTokens []string
	var prefixToken string

	ts, err := s.queryAnalyzer.TokenStream("", strings.NewReader(key))
	if err != nil {
		return nil, err
	}
	defer ts.Close()

	src := ts.GetAttributeSource()
	termAtt, _ := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	offsetAtt, _ := src.GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	if termAtt == nil || offsetAtt == nil {
		return nil, fmt.Errorf("query analyzer does not expose the term and offset attributes")
	}

	var lastToken string
	bq := search.NewBooleanQueryBuilder()
	maxEndOffset := -1

	for {
		more, err := ts.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !more {
			break
		}
		if lastToken != "" {
			matchedTokens = append(matchedTokens, lastToken)
			bq.Add(search.NewTermQuery(index.NewTerm(textFieldName, lastToken)), occur)
		}
		lastToken = termAtt.String()
		if lastToken != "" {
			maxEndOffset = max(maxEndOffset, offsetAtt.EndOffset())
		}
	}
	if err := ts.End(); err != nil {
		return nil, err
	}

	if lastToken != "" {
		var lastQuery search.Query
		if maxEndOffset == offsetAtt.EndOffset() {
			lastQuery = s.getLastTokenQuery(lastToken)
			prefixToken = lastToken
		} else {
			matchedTokens = append(matchedTokens, lastToken)
			lastQuery = search.NewTermQuery(index.NewTerm(textFieldName, lastToken))
		}
		if lastQuery != nil {
			bq.Add(lastQuery, occur)
		}
	}

	if len(contexts) > 0 {
		ctxQuery := search.NewBooleanQueryBuilder()
		for _, ctx := range contexts {
			ctxQuery.Add(search.NewTermQuery(index.NewTerm(contextsFieldName, string(ctx))), search.SHOULD)
		}

		if !allTermsRequired {
			mainQuery := search.NewBooleanQueryBuilder()
			mainQuery.Add(bq.Build(), search.MUST)
			mainQuery.Add(ctxQuery.Build(), search.MUST)
			bq = mainQuery
		} else {
			bq.Add(ctxQuery.Build(), search.MUST)
		}
	}

	finalQuery := bq.Build()

	searcher, err := sm.Acquire()
	if err != nil {
		return nil, err
	}
	defer sm.Release(searcher)

	sort := search.NewSort(search.NewSortFieldWithReverse(weightFieldName, spi.SortFieldTypeLong, true))
	hits, err := searcher.SearchWithSortNoScores(finalQuery, num, sort)
	if err != nil {
		return nil, err
	}

	return s.resultsCreator(searcher, hits, num, key, doHighlight, matchedTokens, prefixToken), nil
}

func (s *AnalyzingInfixSuggester) getLastTokenQuery(token string) search.Query {
	if len(token) < s.minPrefixChars {
		return search.NewTermQuery(index.NewTerm(textgramsFieldName, token))
	}
	return search.NewPrefixQuery(index.NewTerm(textFieldName, token))
}

func (s *AnalyzingInfixSuggester) createResults(searcher *search.IndexSearcher, hits *search.TopFieldDocs, num int, key string, doHighlight bool, matchedTokens []string, prefixToken string) []*suggest.LookupResult {
	results := make([]*suggest.LookupResult, 0, len(hits.FieldDocs))

	reader := searcher.GetIndexReader()
	textDV, err := index.MultiDocValuesGetBinaryValues(reader, textFieldName)
	if err != nil {
		return nil
	}
	payloadsDV, err := index.MultiDocValuesGetBinaryValues(reader, payloadsFieldName)
	if err != nil {
		return nil
	}
	leaves, err := reader.Leaves()
	if err != nil {
		return nil
	}

	for _, fd := range hits.FieldDocs {
		if _, err := textDV.Advance(fd.Doc); err != nil {
			return nil
		}
		raw, err := textDV.BinaryValue()
		if err != nil {
			return nil
		}
		text := string(raw)

		var weight int64
		if len(fd.Fields) > 0 {
			if w, ok := fd.Fields[0].(int64); ok {
				weight = w
			}
		}

		var payload []byte
		if payloadsDV != nil {
			target, err := payloadsDV.Advance(fd.Doc)
			if err != nil {
				return nil
			}
			if target == fd.Doc {
				pv, err := payloadsDV.BinaryValue()
				if err != nil {
					return nil
				}
				payload = append([]byte(nil), pv...)
			}
		}

		contexts, err := readContexts(leaves, fd.Doc)
		if err != nil {
			return nil
		}

		var resultKey string
		if doHighlight {
			resultKey = s.doHighlight(text, matchedTokens, prefixToken)
		} else {
			resultKey = text
		}

		results = append(results, &suggest.LookupResult{
			Key:      resultKey,
			Value:    weight,
			Payload:  payload,
			Contexts: contexts,
		})
	}

	return results
}

// readContexts collects the context values indexed alongside the suggestion at
// the supplied global doc id.
func readContexts(leaves []*index.LeafReaderContext, docID int) ([][]byte, error) {
	segment := index.ReaderUtilSubIndexLeaves(docID, leaves)
	if segment < 0 || segment >= len(leaves) {
		return nil, nil
	}
	ctx := leaves[segment]
	contextsDV, err := ctx.LeafReader().GetSortedSetDocValues(contextsFieldName)
	if err != nil || contextsDV == nil {
		return nil, err
	}
	targetDocID := docID - ctx.DocBase
	target, err := contextsDV.Advance(targetDocID)
	if err != nil {
		return nil, err
	}
	if target != targetDocID {
		return nil, nil
	}
	var contexts [][]byte
	count := contextsDV.DocValueCount()
	for i := 0; i < count; i++ {
		ord, err := contextsDV.NextOrd()
		if err != nil {
			return nil, err
		}
		value, err := contextsDV.LookupOrd(ord)
		if err != nil {
			return nil, err
		}
		contexts = append(contexts, append([]byte(nil), value...))
	}
	return contexts, nil
}

func (s *AnalyzingInfixSuggester) doHighlight(text string, matchedTokens []string, prefixToken string) string {
	ts, err := s.queryAnalyzer.TokenStream("text", strings.NewReader(text))
	if err != nil {
		return text
	}
	defer ts.Close()

	src := ts.GetAttributeSource()
	termAtt, _ := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	offsetAtt, _ := src.GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	if termAtt == nil || offsetAtt == nil {
		return text
	}

	var sb strings.Builder
	upto := 0

	for {
		more, err := ts.IncrementToken()
		if err != nil {
			return text
		}
		if !more {
			break
		}
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
	if err := ts.End(); err != nil {
		return text
	}

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
