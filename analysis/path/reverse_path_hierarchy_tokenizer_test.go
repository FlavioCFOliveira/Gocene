// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package path_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/path"
)

// tokenizeReverse is a helper that runs t through its full lifecycle and
// returns terms, start offsets, end offsets, and position increments.
func tokenizeReverse(t *testing.T, tok *path.ReversePathHierarchyTokenizer, input string) (
	terms []string, starts, ends, posIncrs []int,
) {
	t.Helper()
	if err := tok.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	attrSrc := tok.GetAttributeSource()
	for {
		ok, err := tok.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		term := attrSrc.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute).String()
		off := attrSrc.GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
		pi := attrSrc.GetAttribute(analysis.PositionIncrementAttributeType).(analysis.PositionIncrementAttribute)
		terms = append(terms, term)
		starts = append(starts, off.StartOffset())
		ends = append(ends, off.EndOffset())
		posIncrs = append(posIncrs, pi.GetPositionIncrement())
	}
	return
}

// assertStream checks that the tokenizer produces exactly the expected tokens
// with matching offsets and position increments.
func assertStream(
	t *testing.T,
	tok *path.ReversePathHierarchyTokenizer,
	input string,
	wantTerms []string,
	wantStarts, wantEnds, wantPosIncrs []int,
	wantFinalOffset int,
) {
	t.Helper()
	terms, starts, ends, posIncrs := tokenizeReverse(t, tok, input)

	if len(terms) != len(wantTerms) {
		t.Fatalf("got %d tokens %v, want %d %v", len(terms), terms, len(wantTerms), wantTerms)
	}
	for i := range wantTerms {
		if terms[i] != wantTerms[i] {
			t.Errorf("[%d] term %q want %q", i, terms[i], wantTerms[i])
		}
		if starts[i] != wantStarts[i] {
			t.Errorf("[%d] start %d want %d", i, starts[i], wantStarts[i])
		}
		if ends[i] != wantEnds[i] {
			t.Errorf("[%d] end %d want %d", i, ends[i], wantEnds[i])
		}
		if posIncrs[i] != wantPosIncrs[i] {
			t.Errorf("[%d] posIncr %d want %d", i, posIncrs[i], wantPosIncrs[i])
		}
	}
	// Check End() sets final offset.
	if err := tok.End(); err != nil {
		t.Fatalf("End: %v", err)
	}
	off := tok.GetAttributeSource().GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	if off.EndOffset() != wantFinalOffset {
		t.Errorf("finalOffset %d want %d", off.EndOffset(), wantFinalOffset)
	}
	if err := tok.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
}

// TestReversePathHierarchyTokenizer_BasicReverse mirrors testBasicReverse.
func TestReversePathHierarchyTokenizer_BasicReverse(t *testing.T) {
	tok := path.NewReversePathHierarchyTokenizerFull('/', '/', path.DefaultSkip)
	assertStream(t, tok, "/a/b/c",
		[]string{"/a/b/c", "a/b/c", "b/c", "c"},
		[]int{0, 1, 3, 5},
		[]int{6, 6, 6, 6},
		[]int{1, 1, 1, 1},
		6)
}

// TestReversePathHierarchyTokenizer_EndOfDelimiter mirrors testEndOfDelimiterReverse.
func TestReversePathHierarchyTokenizer_EndOfDelimiter(t *testing.T) {
	tok := path.NewReversePathHierarchyTokenizerFull('/', '/', path.DefaultSkip)
	assertStream(t, tok, "/a/b/c/",
		[]string{"/a/b/c/", "a/b/c/", "b/c/", "c/"},
		[]int{0, 1, 3, 5},
		[]int{7, 7, 7, 7},
		[]int{1, 1, 1, 1},
		7)
}

// TestReversePathHierarchyTokenizer_StartOfChar mirrors testStartOfCharReverse.
func TestReversePathHierarchyTokenizer_StartOfChar(t *testing.T) {
	tok := path.NewReversePathHierarchyTokenizerFull('/', '/', path.DefaultSkip)
	assertStream(t, tok, "a/b/c",
		[]string{"a/b/c", "b/c", "c"},
		[]int{0, 2, 4},
		[]int{5, 5, 5},
		[]int{1, 1, 1},
		5)
}

// TestReversePathHierarchyTokenizer_StartOfCharEndOfDelimiter mirrors
// testStartOfCharEndOfDelimiterReverse.
func TestReversePathHierarchyTokenizer_StartOfCharEndOfDelimiter(t *testing.T) {
	tok := path.NewReversePathHierarchyTokenizerFull('/', '/', path.DefaultSkip)
	assertStream(t, tok, "a/b/c/",
		[]string{"a/b/c/", "b/c/", "c/"},
		[]int{0, 2, 4},
		[]int{6, 6, 6},
		[]int{1, 1, 1},
		6)
}

// TestReversePathHierarchyTokenizer_OnlyDelimiter mirrors testOnlyDelimiterReverse.
func TestReversePathHierarchyTokenizer_OnlyDelimiter(t *testing.T) {
	tok := path.NewReversePathHierarchyTokenizerFull('/', '/', path.DefaultSkip)
	assertStream(t, tok, "/",
		[]string{"/"},
		[]int{0},
		[]int{1},
		[]int{1},
		1)
}

// TestReversePathHierarchyTokenizer_OnlyDelimiters mirrors testOnlyDelimitersReverse.
func TestReversePathHierarchyTokenizer_OnlyDelimiters(t *testing.T) {
	tok := path.NewReversePathHierarchyTokenizerFull('/', '/', path.DefaultSkip)
	assertStream(t, tok, "//",
		[]string{"//", "/"},
		[]int{0, 1},
		[]int{2, 2},
		[]int{1, 1},
		2)
}

// TestReversePathHierarchyTokenizer_EndOfDelimiterSkip mirrors
// testEndOfDelimiterReverseSkip.
func TestReversePathHierarchyTokenizer_EndOfDelimiterSkip(t *testing.T) {
	tok := path.NewReversePathHierarchyTokenizerFull('/', '/', 1)
	assertStream(t, tok, "/a/b/c/",
		[]string{"/a/b/", "a/b/", "b/"},
		[]int{0, 1, 3},
		[]int{5, 5, 5},
		[]int{1, 1, 1},
		7)
}

// TestReversePathHierarchyTokenizer_StartOfCharSkip mirrors
// testStartOfCharReverseSkip.
func TestReversePathHierarchyTokenizer_StartOfCharSkip(t *testing.T) {
	tok := path.NewReversePathHierarchyTokenizerFull('/', '/', 1)
	assertStream(t, tok, "a/b/c",
		[]string{"a/b/", "b/"},
		[]int{0, 2},
		[]int{4, 4},
		[]int{1, 1},
		5)
}

// TestReversePathHierarchyTokenizer_StartOfCharEndOfDelimiterSkip mirrors
// testStartOfCharEndOfDelimiterReverseSkip.
func TestReversePathHierarchyTokenizer_StartOfCharEndOfDelimiterSkip(t *testing.T) {
	tok := path.NewReversePathHierarchyTokenizerFull('/', '/', 1)
	assertStream(t, tok, "a/b/c/",
		[]string{"a/b/", "b/"},
		[]int{0, 2},
		[]int{4, 4},
		[]int{1, 1},
		6)
}

// TestReversePathHierarchyTokenizer_OnlyDelimiterSkip mirrors
// testOnlyDelimiterReverseSkip.
func TestReversePathHierarchyTokenizer_OnlyDelimiterSkip(t *testing.T) {
	tok := path.NewReversePathHierarchyTokenizerFull('/', '/', 1)
	assertStream(t, tok, "/",
		nil, nil, nil, nil,
		1)
}

// TestReversePathHierarchyTokenizer_OnlyDelimitersSkip mirrors
// testOnlyDelimitersReverseSkip.
func TestReversePathHierarchyTokenizer_OnlyDelimitersSkip(t *testing.T) {
	tok := path.NewReversePathHierarchyTokenizerFull('/', '/', 1)
	assertStream(t, tok, "//",
		[]string{"/"},
		[]int{0},
		[]int{1},
		[]int{1},
		2)
}

// TestReversePathHierarchyTokenizer_Skip2 mirrors testReverseSkip2.
func TestReversePathHierarchyTokenizer_Skip2(t *testing.T) {
	tok := path.NewReversePathHierarchyTokenizerFull('/', '/', 2)
	assertStream(t, tok, "/a/b/c/",
		[]string{"/a/", "a/"},
		[]int{0, 1},
		[]int{3, 3},
		[]int{1, 1},
		7)
}

// TestReversePathHierarchyTokenizer_ViaAnalyzerOutput mirrors testTokenizerViaAnalyzerOutput.
func TestReversePathHierarchyTokenizer_ViaAnalyzerOutput(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"a/b/c", []string{"a/b/c", "b/c", "c"}},
		{"a/b/c/", []string{"a/b/c/", "b/c/", "c/"}},
		{"/a/b/c", []string{"/a/b/c", "a/b/c", "b/c", "c"}},
		{"/a/b/c/", []string{"/a/b/c/", "a/b/c/", "b/c/", "c/"}},
	}
	for _, c := range cases {
		tok := path.NewReversePathHierarchyTokenizer()
		terms, _, _, _ := tokenizeReverse(t, tok, c.input)
		if len(terms) != len(c.want) {
			t.Errorf("input %q: got %v, want %v", c.input, terms, c.want)
			continue
		}
		for i := range c.want {
			if terms[i] != c.want[i] {
				t.Errorf("input %q [%d]: got %q want %q", c.input, i, terms[i], c.want[i])
			}
		}
	}
}

// TestReversePathHierarchyTokenizer_Reset verifies the tokenizer can be reused.
func TestReversePathHierarchyTokenizer_Reset(t *testing.T) {
	tok := path.NewReversePathHierarchyTokenizer()

	terms1, _, _, _ := tokenizeReverse(t, tok, "/a/b")
	if err := tok.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	terms2, _, _, _ := tokenizeReverse(t, tok, "/x/y")
	if err := tok.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	want1 := []string{"/a/b", "a/b", "b"}
	want2 := []string{"/x/y", "x/y", "y"}
	for i, w := range want1 {
		if terms1[i] != w {
			t.Errorf("first run [%d] %q want %q", i, terms1[i], w)
		}
	}
	for i, w := range want2 {
		if terms2[i] != w {
			t.Errorf("second run [%d] %q want %q", i, terms2[i], w)
		}
	}
}
