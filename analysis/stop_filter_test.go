// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis_test

// Port of lucene/core/src/test/org/apache/lucene/analysis/TestStopFilter.java
// (Apache Lucene 10.5.0).
//
// StopFilter.makeStopSet is not ported in Gocene. Its Java body is
// `new CharArraySet(stopWords.size(), ignoreCase)` followed by `addAll`, which
// is what NewCharArraySetFromCollection does; the port builds the stop sets
// that way.

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/testutil"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	testsanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

const stopFilterMaxNumberOfTokens = 50

// stopFilterVerbose mirrors LuceneTestCase.VERBOSE (false by default).
const stopFilterVerbose = false

func newStopFilterTestRandom(t *testing.T) *rand.Rand {
	t.Helper()
	seed := uint64(time.Now().UnixNano())
	t.Logf("random seed: %d", seed)
	return rand.New(rand.NewPCG(seed, 0x632BE59BD9B4E019))
}

// newWhitespaceMockTokenizer mirrors
// new MockTokenizer(MockTokenizer.WHITESPACE, false), whose maximum token
// length is MockTokenizer.DEFAULT_MAX_TOKEN_LENGTH = Integer.MAX_VALUE.
func newWhitespaceMockTokenizer() *testsanalysis.MockTokenizer {
	return testsanalysis.NewMockTokenizer(testsanalysis.WHITESPACE, false, math.MaxInt32)
}

func TestStopFilter_ExactCase(t *testing.T) {
	reader := strings.NewReader("Now is The Time")
	stopWords := analysis.NewCharArraySetFromCollection([]string{"is", "the", "Time"}, false)
	in := newWhitespaceMockTokenizer()
	in.SetReader(reader)
	stream := analysis.NewStopFilterWithWords(in, stopWords)
	testutil.AssertTokenStreamContentsSimple(t, stream, []string{"Now", "The"})
}

func TestStopFilter_StopFilter(t *testing.T) {
	reader := strings.NewReader("Now is The Time")
	stopWords := []string{"is", "the", "Time"}
	stopSet := analysis.NewCharArraySetFromCollection(stopWords, false)
	in := newWhitespaceMockTokenizer()
	in.SetReader(reader)
	stream := analysis.NewStopFilterWithWords(in, stopSet)
	testutil.AssertTokenStreamContentsSimple(t, stream, []string{"Now", "The"})
}

func stopFilterLogStopwords(t *testing.T, name string, stopwords []string) {
	t.Helper()
	if len(stopwords) == 0 {
		stopFilterLog(t, fmt.Sprintf("stopword list [%s]: Empty", name))
	} else {
		stopFilterLog(t, fmt.Sprintf("stopword list [%s]: [%s]", name, strings.Join(stopwords, ", ")))
	}
}

// generateTestSetWithStopwordsAndStopwordPositions randomly generates a
// document and a list of stopwords to apply.
func generateTestSetWithStopwordsAndStopwordPositions(
	t *testing.T,
	rand *rand.Rand,
	numberOfTokens int,
	sb *strings.Builder,
	stopwords *[]string,
	stopwordPositions *[]int,
) {
	t.Helper()
	for i := 0; i < numberOfTokens; i++ {
		token := strings.TrimSpace(intToEnglish(i))
		sb.WriteString(token)
		sb.WriteByte(' ')
		if i == 0 || rand.IntN(2) == 0 {
			// with probability 0.5 will tell if this is a stopword or
			// no - adding always the first token to make sure that the
			// list of stopwords is not empty;
			*stopwords = append(*stopwords, token)
			*stopwordPositions = append(*stopwordPositions, i)
		}
	}
	stopFilterLog(t, fmt.Sprintf("Number of tokens : %d", numberOfTokens))
	stopFilterLog(t, "Document : "+sb.String())
	stopFilterLogStopwords(t, "Stopwords", *stopwords)
}

// testTokenPositionWithStopwordFilter checks that the positions of the terms
// in a document keep into account the fact that some of the words were
// filtered by the StopwordFilter.
func TestStopFilter_TokenPositionWithStopwordFilter(t *testing.T) {
	random := newStopFilterTestRandom(t)
	// at least 1 token
	numberOfTokens := random.IntN(stopFilterMaxNumberOfTokens-1) + 1
	var sb strings.Builder
	stopwords := make([]string, 0, numberOfTokens)
	stopwordPositions := make([]int, 0, numberOfTokens)
	generateTestSetWithStopwordsAndStopwordPositions(t, random, numberOfTokens, &sb, &stopwords, &stopwordPositions)

	stopSet := analysis.NewCharArraySetFromCollection(stopwords, false)
	stopFilterLogStopwords(t, "All stopwords", stopwords)
	// with increments
	reader := strings.NewReader(sb.String())
	in := newWhitespaceMockTokenizer()
	in.SetReader(reader)
	stopfilter := analysis.NewStopFilterWithWords(in, stopSet)
	doTestStopwordsPositions(t, stopfilter, stopwordPositions, numberOfTokens)
}

// testTokenPositionsWithConcatenatedStopwordFilters checks that the positions
// of the terms in a document keep into account the fact that some of the
// words were filtered by two StopwordFilters concatenated together.
func TestStopFilter_TokenPositionsWithConcatenatedStopwordFilters(t *testing.T) {
	random := newStopFilterTestRandom(t)
	// at least 1 token
	numberOfTokens := random.IntN(stopFilterMaxNumberOfTokens-1) + 1
	var sb strings.Builder
	stopwords := make([]string, 0, numberOfTokens)
	var stopwordPositions []int
	generateTestSetWithStopwordsAndStopwordPositions(t, random, numberOfTokens, &sb, &stopwords, &stopwordPositions)

	// we want to make sure that concatenating two list of stopwords
	// produce the same results of using one unique list of stopwords.
	// So we first generate a list of stopwords:
	// e.g.: [a, b, c, d, e]
	// and then we split the list in two disjoint partitions
	// e.g. [a, c, e] [b, d]
	partition := random.IntN(len(stopwords))
	random.Shuffle(len(stopwords), func(i, j int) { stopwords[i], stopwords[j] = stopwords[j], stopwords[i] })
	stopwordsRandomPartition := stopwords[:partition]
	stopwordsRemaining := make(map[string]struct{}, len(stopwords))
	for _, w := range stopwords {
		stopwordsRemaining[w] = struct{}{}
	}
	// remove the first partition from all the stopwords
	for _, w := range stopwordsRandomPartition {
		delete(stopwordsRemaining, w)
	}
	remaining := make([]string, 0, len(stopwordsRemaining))
	for w := range stopwordsRemaining {
		remaining = append(remaining, w)
	}

	firstStopSet := analysis.NewCharArraySetFromCollection(stopwordsRandomPartition, false)
	stopFilterLogStopwords(t, "Stopwords-first", stopwordsRandomPartition)
	secondStopSet := analysis.NewCharArraySetFromCollection(remaining, false)
	stopFilterLogStopwords(t, "Stopwords-second", remaining)

	reader := strings.NewReader(sb.String())
	in1 := newWhitespaceMockTokenizer()
	in1.SetReader(reader)

	// Here we create a stopFilter with the stopwords in the first partition
	// and then we concatenate it with the stopFilter created with the
	// stopwords in the second partition
	stopFilter := analysis.NewStopFilterWithWords(in1, firstStopSet)                     // first part of the set
	concatenatedStopFilter := analysis.NewStopFilterWithWords(stopFilter, secondStopSet) // two stop filters concatenated!

	// ... and finally we check that the positions of the filtered tokens
	// matched using the concatenated stopFilters match the positions of the
	// filtered tokens using the unique original list of stopwords
	doTestStopwordsPositions(t, concatenatedStopFilter, stopwordPositions, numberOfTokens)
}

// LUCENE-3849: make sure after .end() we see the "ending" posInc
func TestStopFilter_EndStopword(t *testing.T) {
	stopSet := analysis.NewCharArraySetFromCollection([]string{"of"}, false)
	in := newWhitespaceMockTokenizer()
	in.SetReader(strings.NewReader("test of"))
	stopfilter := analysis.NewStopFilterWithWords(in, stopSet)
	testutil.AssertTokenStreamContents(t, stopfilter, testutil.TokenStreamExpectations{
		Terms:                  []string{"test"},
		StartOffsets:           []int{0},
		EndOffsets:             []int{4},
		PositionIncrements:     []int{1},
		FinalOffset:            testutil.IntPtr(7),
		FinalPositionIncrement: testutil.IntPtr(1),
	}.WithGraphOffsetsAreCorrect(true))
}

func doTestStopwordsPositions(t *testing.T, stopfilter *analysis.StopFilter, stopwordPositions []int, numberOfTokens int) {
	t.Helper()
	termAtt := stopfilter.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	stopfilter.GetAttribute(tokenattributes.PositionIncrementAttributeType)
	if err := stopfilter.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}
	stopFilterLog(t, "Test stopwords positions:")
	for i := 0; i < numberOfTokens; i++ {
		if slices.Contains(stopwordPositions, i) {
			// if i is in stopwordPosition it is a stopword and we skip this position
			continue
		}
		ok, err := stopfilter.IncrementToken()
		if err != nil {
			t.Fatalf("incrementToken: %v", err)
		}
		if !ok {
			t.Fatalf("expected a token at position %d", i)
		}
		stopFilterLog(t, fmt.Sprintf("token %d: %s", i, termAtt.String()))
		token := strings.TrimSpace(intToEnglish(i))
		if got := termAtt.String(); got != token {
			t.Fatalf("expecting token %d to be %s, got %s", i, token, got)
		}
	}
	ok, err := stopfilter.IncrementToken()
	if err != nil {
		t.Fatalf("incrementToken: %v", err)
	}
	if ok {
		t.Fatalf("expected end of stream, got %q", termAtt.String())
	}
	if err := stopfilter.End(); err != nil {
		t.Fatalf("end: %v", err)
	}
	if err := stopfilter.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	stopFilterLog(t, "----------")
}

// stopFilterLog prints debug info depending on VERBOSE.
func stopFilterLog(t *testing.T, s string) {
	t.Helper()
	if stopFilterVerbose {
		t.Log(s)
	}
}

// intToEnglish mirrors org.apache.lucene.tests.util.English.intToEnglish(int).
func intToEnglish(i int) string {
	var result strings.Builder
	longToEnglish(int64(i), &result)
	return result.String()
}

// longToEnglish mirrors English.longToEnglish(long, StringBuilder).
func longToEnglish(i int64, result *strings.Builder) {
	if i == 0 {
		result.WriteString("zero")
		return
	}
	if i < 0 {
		result.WriteString("minus ")
		i = -i
	}
	if i >= 1000000000000000000 { // quadrillion
		longToEnglish(i/1000000000000000000, result)
		result.WriteString("quintillion, ")
		i = i % 1000000000000000000
	}
	if i >= 1000000000000000 { // quadrillion
		longToEnglish(i/1000000000000000, result)
		result.WriteString("quadrillion, ")
		i = i % 1000000000000000
	}
	if i >= 1000000000000 { // trillions
		longToEnglish(i/1000000000000, result)
		result.WriteString("trillion, ")
		i = i % 1000000000000
	}
	if i >= 1000000000 { // billions
		longToEnglish(i/1000000000, result)
		result.WriteString("billion, ")
		i = i % 1000000000
	}
	if i >= 1000000 { // millions
		longToEnglish(i/1000000, result)
		result.WriteString("million, ")
		i = i % 1000000
	}
	if i >= 1000 { // thousands
		longToEnglish(i/1000, result)
		result.WriteString("thousand, ")
		i = i % 1000
	}
	if i >= 100 { // hundreds
		longToEnglish(i/100, result)
		result.WriteString("hundred ")
		i = i % 100
	}
	// we know we are smaller here so we can cast
	if i >= 20 {
		switch int(i) / 10 {
		case 9:
			result.WriteString("ninety")
		case 8:
			result.WriteString("eighty")
		case 7:
			result.WriteString("seventy")
		case 6:
			result.WriteString("sixty")
		case 5:
			result.WriteString("fifty")
		case 4:
			result.WriteString("forty")
		case 3:
			result.WriteString("thirty")
		case 2:
			result.WriteString("twenty")
		}
		i = i % 10
		if i == 0 {
			result.WriteString(" ")
		} else {
			result.WriteString("-")
		}
	}
	units := [...]string{
		"", "one ", "two ", "three ", "four ", "five ", "six ", "seven ", "eight ", "nine ",
		"ten ", "eleven ", "twelve ", "thirteen ", "fourteen ", "fifteen ", "sixteen ",
		"seventeen ", "eighteen ", "nineteen ",
	}
	result.WriteString(units[int(i)])
}
