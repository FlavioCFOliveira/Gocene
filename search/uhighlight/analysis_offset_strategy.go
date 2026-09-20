package uhighlight

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// AnalysisOffsetStrategy provides a base class for analysis based offset strategies to extend from.
// Requires an Analyzer and provides an override-able method for altering how the TokenStream is created.
// Mirrors org.apache.lucene.search.uhighlight.AnalysisOffsetStrategy.
type AnalysisOffsetStrategy struct {
	BaseFieldOffsetStrategy
	analyzer analysis.Analyzer
}

// NewAnalysisOffsetStrategy builds the strategy. Mirrors
// AnalysisOffsetStrategy(UHComponents, Analyzer)
// (AnalysisOffsetStrategy.java:36).
func NewAnalysisOffsetStrategy(components *UHComponents, analyzer analysis.Analyzer) *AnalysisOffsetStrategy {
	s := &AnalysisOffsetStrategy{
		BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(components),
		analyzer:                analyzer,
	}
	if analyzerOffsetGap(analyzer, s.Field()) != 1 { // note: 1 is the default. It is RARELY changed.
		panic(fmt.Sprintf("offset gap of the provided analyzer should be 1 (field %s)", s.Field()))
	}
	return s
}

// analyzerOffsetGap renders Analyzer.getOffsetGap(String)
// (Analyzer.java), whose body returns 1 unless a subclass overrides it.
// Gocene's analysis.Analyzer interface does not declare the method, so it is
// reached through the optional-interface probe the analysis package itself
// uses (analysis/delegating_analyzer_wrapper.go:67).
func analyzerOffsetGap(analyzer analysis.Analyzer, fieldName string) int {
	if gap, ok := analyzer.(interface {
		GetOffsetGap(string) int
	}); ok {
		return gap.GetOffsetGap(fieldName)
	}
	return 1
}

// analyzerPositionIncrementGap renders
// Analyzer.getPositionIncrementGap(String) (Analyzer.java), whose body returns
// 0 unless a subclass overrides it. See analyzerOffsetGap for why it is a
// probe.
func analyzerPositionIncrementGap(analyzer analysis.Analyzer, fieldName string) int {
	if gap, ok := analyzer.(interface {
		GetPositionIncrementGap(string) int
	}); ok {
		return gap.GetPositionIncrementGap(fieldName)
	}
	return 0
}

// GetOffsetSource returns the OffsetSource that characterises how this strategy resolves document offsets.
func (s *AnalysisOffsetStrategy) GetOffsetSource() OffsetSource {
	return OffsetSourceAnalysis
}

// TokenStream creates a TokenStream for the supplied content.
func (s *AnalysisOffsetStrategy) TokenStream(content string) (analysis.TokenStream, error) {
	// If there is no splitChar in content then we needn't wrap:
	splitCharIdx := -1
	for i, r := range content {
		if r == MultivalSepChar {
			splitCharIdx = i
			break
		}
	}

	if splitCharIdx == -1 {
		return s.analyzer.TokenStream(s.Field(), strings.NewReader(content))
	}

	subTokenStream, err := s.analyzer.TokenStream(s.Field(), strings.NewReader(content[:splitCharIdx]))
	if err != nil {
		return nil, err
	}

	return NewMultiValueTokenStream(
		subTokenStream,
		s.Field(),
		s.analyzer,
		content,
		MultivalSepChar,
		splitCharIdx), nil
}

// multiValueTokenStream wraps an Analyzer and string text that represents multiple values delimited by a
// specified character. This exposes a TokenStream that matches what would get indexed considering the
// Analyzer.getPositionIncrementGap(String).
type multiValueTokenStream struct {
	fieldName     string
	indexAnalyzer analysis.Analyzer
	content       string
	splitChar     rune
	input         analysis.TokenStream

	attrSource *util.AttributeSource
	posIncAtt  tokenattributes.PositionIncrementAttribute
	offsetAtt  analysis.OffsetAttribute

	startValIdx     int
	endValIdx       int
	remainingPosInc int
}

// NewMultiValueTokenStream builds the multi-value token stream.
func NewMultiValueTokenStream(
	subTokenStream analysis.TokenStream,
	fieldName string,
	indexAnalyzer analysis.Analyzer,
	content string,
	splitChar rune,
	splitCharIdx int) analysis.TokenStream {

	ts := &multiValueTokenStream{
		fieldName:     fieldName,
		indexAnalyzer: indexAnalyzer,
		content:       content,
		splitChar:     splitChar,
		input:         subTokenStream,
		startValIdx:   0,
		endValIdx:     splitCharIdx,
	}

	src := attributeSourceFor(subTokenStream)
	if src == nil {
		panic("subTokenStream must provide an AttributeSource")
	}

	ts.attrSource = src
	ts.posIncAtt, _ = src.GetAttribute(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
	ts.offsetAtt, _ = src.GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)

	return ts
}

// GetAttributeSource renders the AttributeSource a Java TokenFilter inherits
// from the TokenStream it wraps: MultiValueTokenStream extends TokenFilter, so
// it shares the attributes of its sub-token-stream.
func (ts *multiValueTokenStream) GetAttributeSource() *util.AttributeSource {
	return ts.attrSource
}

func (ts *multiValueTokenStream) Reset() error {
	if ts.startValIdx != 0 {
		return fmt.Errorf("this TokenStream wasn't developed to be re-used")
	}
	return ts.input.Reset()
}

func (ts *multiValueTokenStream) IncrementToken() (bool, error) {
	for {
		more, err := ts.input.IncrementToken()
		if err != nil {
			return false, err
		}
		if more {
			// Position tracking:
			if ts.remainingPosInc > 0 {
				ts.posIncAtt.SetPositionIncrement(ts.remainingPosInc + ts.posIncAtt.GetPositionIncrement())
				ts.remainingPosInc = 0 // reset
			}
			// Offset tracking:
			ts.offsetAtt.SetOffset(
				ts.startValIdx+ts.offsetAtt.StartOffset(),
				ts.startValIdx+ts.offsetAtt.EndOffset())
			return true, nil
		}

		if ts.endValIdx == len(ts.content) { // no more
			return false, nil
		}

		ts.input.End()
		ts.remainingPosInc += ts.posIncAtt.GetPositionIncrement()
		ts.input.Close()
		ts.remainingPosInc += analyzerPositionIncrementGap(ts.indexAnalyzer, ts.fieldName)

		// Get new tokenStream based on next segment divided by the splitChar
		ts.startValIdx = ts.endValIdx + 1

		// Find next splitChar
		nextSplitIdx := -1
		for i := ts.startValIdx; i < len(ts.content); i++ {
			if rune(ts.content[i]) == ts.splitChar {
				nextSplitIdx = i
				break
			}
		}

		if nextSplitIdx == -1 {
			ts.endValIdx = len(ts.content)
		} else {
			ts.endValIdx = nextSplitIdx
		}

		tokenStream, err := ts.indexAnalyzer.TokenStream(ts.fieldName, strings.NewReader(ts.content[ts.startValIdx:ts.endValIdx]))
		if err != nil {
			return false, err
		}

		if tokenStream != ts.input {
			ts.input = tokenStream
			if src := attributeSourceFor(tokenStream); src != nil {
				ts.attrSource = src
				ts.posIncAtt, _ = src.GetAttribute(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
				ts.offsetAtt, _ = src.GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
			}
		}

		if err := ts.input.Reset(); err != nil {
			return false, err
		}
	}
}

func (ts *multiValueTokenStream) End() error {
	err := ts.input.End()
	// Offset tracking:
	ts.offsetAtt.SetOffset(
		ts.startValIdx+ts.offsetAtt.StartOffset(),
		ts.startValIdx+ts.offsetAtt.EndOffset())
	return err
}

func (ts *multiValueTokenStream) Close() error {
	return ts.input.Close()
}

func attributeSourceFor(stream analysis.TokenStream) *util.AttributeSource {
	type attrSrc interface{ GetAttributeSource() *util.AttributeSource }
	if a, ok := stream.(attrSrc); ok {
		return a.GetAttributeSource()
	}
	return nil
}
