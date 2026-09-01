package uhighlight

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// AnalysisOffsetStrategy provides a base class for analysis based offset strategies to extend from.
// Requires an Analyzer and provides an override-able method for altering how the TokenStream is created.
// Mirrors org.apache.lucene.search.uhighlight.AnalysisOffsetStrategy.
type AnalysisOffsetStrategy struct {
	BaseFieldOffsetStrategy
	analyzer analysis.Analyzer
}

// NewAnalysisOffsetStrategy builds the strategy.
func NewAnalysisOffsetStrategy(field string, analyzer analysis.Analyzer) *AnalysisOffsetStrategy {
	if analyzer.GetOffsetGap(field) != 1 { // note: 1 is the default. It is RARELY changed.
		panic(fmt.Sprintf("offset gap of the provided analyzer should be 1 (field %s)", field))
	}
	return &AnalysisOffsetStrategy{
		BaseFieldOffsetStrategy: NewBaseFieldOffsetStrategy(field),
		analyzer:                analyzer,
	}
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
		return s.analyzer.TokenStream(s.Field(), content), nil
	}

	subTokenStream, err := s.analyzer.TokenStream(s.Field(), content[:splitCharIdx])
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

	posIncAtt analysis.PositionIncrementAttribute
	offsetAtt analysis.OffsetAttribute

	startValIdx      int
	endValIdx        int
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
		indexAnalyzer:  indexAnalyzer,
		content:       content,
		splitChar:     splitChar,
		input:         subTokenStream,
		startValIdx:    0,
		endValIdx:      splitCharIdx,
	}

	src := attributeSourceFor(subTokenStream)
	if src == nil {
		panic("subTokenStream must provide an AttributeSource")
	}

	ts.posIncAtt, _ = src.GetAttribute(analysis.PositionIncrementAttributeType).(analysis.PositionIncrementAttribute)
	ts.offsetAtt, _ = src.GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)

	return ts
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
				ts.startValIdx+ts.offsetAtt.GetStartOffset(),
				ts.startValIdx+ts.offsetAtt.GetEndOffset())
			return true, nil
		}

		if ts.endValIdx == len(ts.content) { // no more
			return false, nil
		}

		ts.input.End()
		ts.remainingPosInc += ts.posIncAtt.GetPositionIncrement()
		ts.input.Close()
		ts.remainingPosInc += ts.indexAnalyzer.GetPositionIncrementGap(ts.fieldName)

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

		tokenStream, err := ts.indexAnalyzer.TokenStream(ts.fieldName, ts.content[ts.startValIdx:ts.endValIdx])
		if err != nil {
			return false, err
		}

		if tokenStream != ts.input {
			ts.input = tokenStream
			src := attributeSourceFor(tokenStream)
			if src != nil {
				ts.posIncAtt, _ = src.GetAttribute(analysis.PositionIncrementAttributeType).(analysis.PositionIncrementAttribute)
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
		ts.startValIdx+ts.offsetAtt.GetStartOffset(),
		ts.startValIdx+ts.offsetAtt.GetEndOffset())
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
