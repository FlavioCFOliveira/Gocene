package matchhighlight

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/api"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
)

// This file carries the transliteration helpers that Java expresses inline:
// TokenStream#getAttribute(Class) is a generic lookup with no Go equivalent,
// and Analyzer#getOffsetGap / #getPositionIncrementGap are base-class methods
// that the analysis.Analyzer interface does not declare. Both are resolved
// here exactly as the rest of Gocene resolves them, so the strategies below
// read like their Lucene 10.5.0 counterparts.

// offsetGap returns the analyzer's offset gap for field. Mirrors
// org.apache.lucene.analysis.Analyzer#getOffsetGap, whose base implementation
// returns 1.
func offsetGap(a analysis.Analyzer, field string) int {
	if gap, ok := a.(interface{ GetOffsetGap(string) int }); ok {
		return gap.GetOffsetGap(field)
	}
	return 1
}

// positionIncrementGap returns the analyzer's position increment gap for
// field. Mirrors org.apache.lucene.analysis.Analyzer#getPositionIncrementGap,
// whose base implementation returns 0.
func positionIncrementGap(a analysis.Analyzer, field string) int {
	if gap, ok := a.(interface{ GetPositionIncrementGap(string) int }); ok {
		return gap.GetPositionIncrementGap(field)
	}
	return 0
}

// offsetAttribute resolves the stream's OffsetAttribute. Mirrors
// ts.getAttribute(OffsetAttribute.class).
func offsetAttribute(ts api.TokenStream) (analysis.OffsetAttribute, error) {
	a, _ := ts.GetAttributeSource().GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	if a == nil {
		return nil, fmt.Errorf("token stream exposes no OffsetAttribute")
	}
	return a, nil
}

// positionIncrementAttribute resolves the stream's PositionIncrementAttribute.
// Mirrors ts.getAttribute(PositionIncrementAttribute.class).
func positionIncrementAttribute(ts api.TokenStream) (tokenattributes.PositionIncrementAttribute, error) {
	a, _ := ts.GetAttributeSource().GetAttribute(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
	if a == nil {
		return nil, fmt.Errorf("token stream exposes no PositionIncrementAttribute")
	}
	return a, nil
}

// termToBytesRefAttribute resolves the stream's TermToBytesRefAttribute.
// Mirrors ts.getAttribute(TermToBytesRefAttribute.class).
func termToBytesRefAttribute(ts api.TokenStream) (analysis.TermToBytesRefAttribute, error) {
	a, _ := ts.GetAttributeSource().GetAttribute(analysis.TermToBytesRefAttributeType).(analysis.TermToBytesRefAttribute)
	if a == nil {
		return nil, fmt.Errorf("token stream exposes no TermToBytesRefAttribute")
	}
	return a, nil
}
