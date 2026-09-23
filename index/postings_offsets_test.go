// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestPostingsOffsets.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"io"
	"math"
	"math/rand"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

// postingsOffsetsTest renders the instance state of TestPostingsOffsets: iwc
// is built by setUp().
type postingsOffsetsTest struct {
	iwc *index.IndexWriterConfig
}

func newPostingsOffsetsTest() *postingsOffsetsTest {
	return &postingsOffsetsTest{iwc: newIndexWriterConfigWithAnalyzer(newMockAnalyzer())}
}

func offsetsFieldType(base *document.FieldType, randomVectors bool) *document.FieldType {
	ft := document.NewFieldTypeFrom(base)
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	if randomVectors && rand.Intn(2) == 0 {
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorPositions(rand.Intn(2) == 0)
		ft.SetStoreTermVectorOffsets(rand.Intn(2) == 0)
	}
	return ft
}

func cannedTokenField(t testing.TB, name string, ft *document.FieldType, tokens ...testanalysis.Token) *document.Field {
	t.Helper()
	f, err := document.NewField(name, testanalysis.NewCannedTokenStream(tokens...), ft)
	if err != nil {
		t.Fatalf("Field: %v", err)
	}
	return f
}

func TestPostingsOffsetsBasic(t *testing.T) {
	s := newPostingsOffsetsTest()
	dir := newDirectory()

	w := newRandomIndexWriterWithConfig(t, dir, s.iwc)
	doc := document.NewDocument()

	ft := offsetsFieldType(document.TextFieldTypeNotStored, true)
	tokens := []testanalysis.Token{
		postingsOffsetsMakeToken("a", 1, 0, 6),
		postingsOffsetsMakeToken("b", 1, 8, 9),
		postingsOffsetsMakeToken("a", 1, 9, 17),
		postingsOffsetsMakeToken("c", 1, 19, 50),
	}
	doc.Add(cannedTokenField(t, "content", ft, tokens...))

	if _, err := w.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	r := mustGetReaderRIW(t, w)
	mustClose(t, w)
	defer mustClose(t, r, dir)

	t.Fatal(multiTermsGetTermPostingsEnumMissing)
}

func TestPostingsOffsetsSkipping(t *testing.T) {
	newPostingsOffsetsTest().doTestNumbers(t, false)
}

func TestPostingsOffsetsPayloads(t *testing.T) {
	newPostingsOffsetsTest().doTestNumbers(t, true)
}

func (s *postingsOffsetsTest) doTestNumbers(t *testing.T, withPayloads bool) {
	t.Helper()
	dir := newDirectory()
	var analyzer analysis.Analyzer
	if withPayloads {
		analyzer = testanalysis.NewMockPayloadAnalyzer()
	} else {
		analyzer = newMockAnalyzer()
	}
	s.iwc = newIndexWriterConfigWithAnalyzer(analyzer)
	s.iwc.SetMergePolicy(newLogMergePolicy()) // will rely on docids a bit for skipping
	w := newRandomIndexWriterWithConfig(t, dir, s.iwc)
	defer mustClose(t, w, dir)

	ft := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	if rand.Intn(2) == 0 {
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorOffsets(rand.Intn(2) == 0)
		ft.SetStoreTermVectorPositions(rand.Intn(2) == 0)
	}

	t.Fatal("org.apache.lucene.tests.util.English#intToEnglish(int) is not ported")
}

// postingsOffsetsCheckPostings renders the per-term verification loops of
// testRandom.
func postingsOffsetsCheckPostings(t *testing.T, termsEnum index.TermsEnum, term string, actualTokens map[string]map[int][]testanalysis.Token, docIDToID []int) {
	t.Helper()
	found, err := termsEnum.SeekExact(spi.NewTerm("content", term))
	if err != nil {
		t.Fatalf("seekExact(%q): %v", term, err)
	}
	if !found {
		return
	}
	docs, err := termsEnum.Postings(spi.PostingsFlagFreqs)
	if err != nil || docs == nil {
		t.Fatalf("assertNotNull(docs): %v", err)
	}
	for {
		doc, err := docs.NextDoc()
		if err != nil {
			t.Fatalf("nextDoc: %v", err)
		}
		if doc == spi.NO_MORE_DOCS {
			break
		}
		expected := actualTokens[term][docIDToID[doc]]
		if expected == nil {
			t.Fatalf("assertNotNull(expected) for %q doc %d", term, doc)
		}
		if freq, err := docs.Freq(); err != nil || freq != len(expected) {
			t.Fatalf("freq: expected %d, got %d (%v)", len(expected), freq, err)
		}
	}

	for _, withOffsets := range []bool{false, true} {
		// explicitly exclude offsets here (the Java test still asks for ALL)
		docsAndPositions, err := termsEnum.Postings(spi.PostingsFlagAll)
		if err != nil || docsAndPositions == nil {
			t.Fatalf("assertNotNull(docsAndPositions): %v", err)
		}
		for {
			doc, err := docsAndPositions.NextDoc()
			if err != nil {
				t.Fatalf("nextDoc: %v", err)
			}
			if doc == spi.NO_MORE_DOCS {
				break
			}
			expected := actualTokens[term][docIDToID[doc]]
			if expected == nil {
				t.Fatalf("assertNotNull(expected) for %q doc %d", term, doc)
			}
			if freq, err := docsAndPositions.Freq(); err != nil || freq != len(expected) {
				t.Fatalf("freq: expected %d, got %d (%v)", len(expected), freq, err)
			}
			for _, token := range expected {
				pos, err := strconv.Atoi(token.Type)
				if err != nil {
					t.Fatalf("Integer.parseInt(%q): %v", token.Type, err)
				}
				if got, err := docsAndPositions.NextPosition(); err != nil || got != pos {
					t.Fatalf("nextPosition: expected %d, got %d (%v)", pos, got, err)
				}
				if withOffsets {
					if got, err := docsAndPositions.StartOffset(); err != nil || got != token.StartOffset {
						t.Fatalf("startOffset: expected %d, got %d (%v)", token.StartOffset, got, err)
					}
					if got, err := docsAndPositions.EndOffset(); err != nil || got != token.EndOffset {
						t.Fatalf("endOffset: expected %d, got %d (%v)", token.EndOffset, got, err)
					}
				}
			}
		}
	}
}

func TestPostingsOffsetsRandom(t *testing.T) {
	s := newPostingsOffsetsTest()
	// token -> docID -> tokens
	actualTokens := map[string]map[int][]testanalysis.Token{}

	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, s.iwc)

	numDocs := atLeast(20)

	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)

	// TODO: randomize what IndexOptions we use; also test
	// changing this up in one IW buffered segment...:
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	if rand.Intn(2) == 0 {
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorOffsets(rand.Intn(2) == 0)
		ft.SetStoreTermVectorPositions(rand.Intn(2) == 0)
	}

	for docCount := 0; docCount < numDocs; docCount++ {
		doc := document.NewDocument()
		doc.Add(numericDVField(t, "id", int64(docCount)))
		var tokens []testanalysis.Token
		numTokens := atLeast(100)
		pos := -1
		offset := 0
		for tokenCount := 0; tokenCount < numTokens; tokenCount++ {
			var text string
			if rand.Intn(2) == 0 {
				text = "a"
			} else if rand.Intn(2) == 0 {
				text = "b"
			} else if rand.Intn(2) == 0 {
				text = "c"
			} else {
				text = "d"
			}

			posIncr := rand.Intn(5)
			if rand.Intn(2) == 0 {
				posIncr = 1
			}
			if tokenCount == 0 && posIncr == 0 {
				posIncr = 1
			}
			offIncr := rand.Intn(5)
			if rand.Intn(2) == 0 {
				offIncr = 0
			}
			tokenOffset := rand.Intn(5)

			token := postingsOffsetsMakeToken(text, posIncr, offset+offIncr, offset+offIncr+tokenOffset)
			pos += posIncr
			// stuff abs position into type:
			token = token.WithType(strconv.Itoa(pos))
			if actualTokens[text] == nil {
				actualTokens[text] = map[int][]testanalysis.Token{}
			}
			actualTokens[text][docCount] = append(actualTokens[text][docCount], token)
			tokens = append(tokens, token)
			offset += offIncr + tokenOffset
		}
		doc.Add(cannedTokenField(t, "content", ft, tokens...))
		if _, err := w.AddDocument(doc); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	r := mustGetReaderRIW(t, w)
	mustClose(t, w)

	terms := []string{"a", "b", "c", "d"}
	leaves, err := r.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	for _, ctx := range leaves {
		// TODO: improve this
		sub := ctx.LeafReader()
		contentTerms, err := sub.Terms("content")
		if err != nil {
			t.Fatalf("terms: %v", err)
		}
		termsEnum, err := contentTerms.Iterator()
		if err != nil {
			t.Fatalf("iterator: %v", err)
		}
		docIDToID := make([]int, sub.MaxDoc())
		values, err := index.GetNumeric(sub, "id")
		if err != nil {
			t.Fatalf("DocValues.getNumeric: %v", err)
		}
		for i := 0; i < sub.MaxDoc(); i++ {
			if doc, err := values.NextDoc(); err != nil || doc != i {
				t.Fatalf("nextDoc: expected %d, got %d (%v)", i, doc, err)
			}
			v, err := values.LongValue()
			if err != nil {
				t.Fatalf("longValue: %v", err)
			}
			docIDToID[i] = int(v)
		}

		for _, term := range terms {
			postingsOffsetsCheckPostings(t, termsEnum, term, actualTokens, docIDToID)
		}
		// TODO: test advance:
	}
	mustClose(t, r, dir)
}

func TestPostingsOffsetsAddFieldTwice(t *testing.T) {
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	doc := document.NewDocument()
	customType3 := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	customType3.SetStoreTermVectors(true)
	customType3.SetStoreTermVectorPositions(true)
	customType3.SetStoreTermVectorOffsets(true)
	customType3.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	doc.Add(newField(t, "content3", "here is more content with aaa aaa aaa", customType3))
	doc.Add(newField(t, "content3", "here is more content with aaa aaa aaa", customType3))
	if _, err := iw.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	mustClose(t, iw, dir) // checkindex
}

// NOTE: the next two tests aren't that good as we need an EvilToken...
func TestPostingsOffsetsNegativeOffsets(t *testing.T) {
	if err := newPostingsOffsetsTest().checkTokens(t, []testanalysis.Token{postingsOffsetsMakeToken("foo", 1, -1, -1)}); err == nil {
		t.Fatal("expected IllegalArgumentException")
	}
}

func TestPostingsOffsetsIllegalOffsets(t *testing.T) {
	if err := newPostingsOffsetsTest().checkTokens(t, []testanalysis.Token{postingsOffsetsMakeToken("foo", 1, 1, 0)}); err == nil {
		t.Fatal("expected IllegalArgumentException")
	}
}

func TestPostingsOffsetsIllegalOffsetsAcrossFieldInstances(t *testing.T) {
	err := newPostingsOffsetsTest().checkTokens(t,
		[]testanalysis.Token{postingsOffsetsMakeToken("use", 1, 150, 160)},
		[]testanalysis.Token{postingsOffsetsMakeToken("use", 1, 50, 60)})
	if err == nil {
		t.Fatal("expected IllegalArgumentException")
	}
}

func TestPostingsOffsetsBackwardsOffsets(t *testing.T) {
	err := newPostingsOffsetsTest().checkTokens(t, []testanalysis.Token{
		postingsOffsetsMakeToken("foo", 1, 0, 3), postingsOffsetsMakeToken("foo", 1, 4, 7), postingsOffsetsMakeToken("foo", 0, 3, 6),
	})
	if err == nil {
		t.Fatal("expected IllegalArgumentException")
	}
}

func TestPostingsOffsetsStackedTokens(t *testing.T) {
	err := newPostingsOffsetsTest().checkTokens(t, []testanalysis.Token{
		postingsOffsetsMakeToken("foo", 1, 0, 3), postingsOffsetsMakeToken("foo", 0, 0, 3), postingsOffsetsMakeToken("foo", 0, 0, 3),
	})
	if err != nil {
		t.Fatalf("checkTokens: %v", err)
	}
}

// crazyOffsetGapAnalyzer renders the anonymous Analyzer of
// testCrazyOffsetGap: a non-lower-casing keyword MockTokenizer with an offset
// gap of -10.
type crazyOffsetGapAnalyzer struct {
	*analysis.BaseAnalyzer
}

func (a *crazyOffsetGapAnalyzer) GetOffsetGap(string) int { return -10 }

func newCrazyOffsetGapAnalyzer() *crazyOffsetGapAnalyzer {
	base := analysis.NewAnalyzer(nil)
	base.CreateComponents = func(string) *analysis.TokenStreamComponents {
		src := testanalysis.NewMockTokenizer(testanalysis.KEYWORD, false, testanalysis.DefaultMaxTokenLength)
		return &analysis.TokenStreamComponents{
			Source: func(r io.Reader) error {
				src.SetReader(r)
				return nil
			},
			Sink: src,
		}
	}
	return &crazyOffsetGapAnalyzer{BaseAnalyzer: base}
}

func TestPostingsOffsetsCrazyOffsetGap(t *testing.T) {
	dir := newDirectory()
	analyzer := newCrazyOffsetGapAnalyzer()
	iw := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(analyzer))
	// add good document
	doc := document.NewDocument()
	mustAddDocument(t, iw, doc)
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	doc.Add(newField(t, "foo", "bar", ft))
	doc.Add(newField(t, "foo", "bar", ft))
	if _, err := iw.AddDocument(doc); err == nil {
		t.Fatal("expected IllegalArgumentException from addDocument")
	}
	mustCommit(t, iw)
	mustClose(t, iw)

	// make sure we see our good doc
	r := mustOpenDirectoryReader(t, dir)
	assertNumDocs(t, 1, r)
	mustClose(t, r, dir)
}

func TestPostingsOffsetsLegalbutVeryLargeOffsets(t *testing.T) {
	dir := newDirectory()
	iw := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(nil))
	doc := document.NewDocument()
	t1 := testanalysis.NewToken("foo", 0, math.MaxInt32-500)
	if rand.Intn(2) == 0 {
		t1 = t1.WithPayload([]byte("test"))
	}
	t2 := testanalysis.NewToken("foo", math.MaxInt32-500, math.MaxInt32)
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	// store some term vectors for the checkindex cross-check
	ft.SetStoreTermVectors(true)
	ft.SetStoreTermVectorPositions(true)
	ft.SetStoreTermVectorOffsets(true)
	doc.Add(cannedTokenField(t, "foo", ft, t1, t2))
	mustAddDocument(t, iw, doc)
	mustClose(t, iw, dir)
}

// checkTokens renders the private checkTokens(Token[]...) overloads; the
// returned error is the exception the Java body lets propagate.
func (s *postingsOffsetsTest) checkTokens(t *testing.T, fields ...[]testanalysis.Token) (err error) {
	t.Helper()
	dir := newDirectory()
	riw, err := newRandomIndexWriterOrError(dir, s.iwc)
	if err != nil {
		mustClose(t, dir)
		return err
	}
	success := false
	defer func() {
		if success {
			mustClose(t, dir)
		} else {
			// IOUtils.closeWhileHandlingException(riw, dir)
			closeWhileHandlingException(riw, dir)
		}
	}()
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	// store some term vectors for the checkindex cross-check
	ft.SetStoreTermVectors(true)
	ft.SetStoreTermVectorPositions(true)
	ft.SetStoreTermVectorOffsets(true)

	doc := document.NewDocument()
	for _, tokens := range fields {
		doc.Add(cannedTokenField(t, "body", ft, tokens...))
	}
	if _, err := riw.AddDocument(doc); err != nil {
		return err
	}
	if err := riw.Close(); err != nil {
		return err
	}
	success = true
	return nil
}

// postingsOffsetsMakeToken renders the private makeToken(String, int, int, int).
func postingsOffsetsMakeToken(text string, posIncr, startOffset, endOffset int) testanalysis.Token {
	return testanalysis.NewTokenWithPosInc(text, posIncr, startOffset, endOffset)
}
