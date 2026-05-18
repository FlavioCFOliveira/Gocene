package vectorhighlight

// FieldPhraseList is the in-document phrase index produced when the
// vectorhighlight pipeline walks a FieldTermStack against a FieldQuery.
// Mirrors org.apache.lucene.search.vectorhighlight.FieldPhraseList.
type FieldPhraseList struct {
	Phrases []WeightedPhraseInfo
}

// WeightedPhraseInfo is a single in-document phrase with its boost.
type WeightedPhraseInfo struct {
	StartOffset int
	EndOffset   int
	TotalBoost  float32
	TermsUsed   []string
}

// NewFieldPhraseList builds the list from term-stack/query intersection.
func NewFieldPhraseList(stack *FieldTermStack, query *FieldQuery) *FieldPhraseList {
	out := &FieldPhraseList{}
	if stack == nil || query == nil {
		return out
	}
	terms := query.Terms(stack.Field)
	for {
		occ, ok := stack.Pop()
		if !ok {
			break
		}
		if w, found := terms[occ.Term]; found {
			out.Phrases = append(out.Phrases, WeightedPhraseInfo{
				StartOffset: occ.StartOffset,
				EndOffset:   occ.EndOffset,
				TotalBoost:  w,
				TermsUsed:   []string{occ.Term},
			})
		}
	}
	return out
}

// Size returns the number of phrases.
func (l *FieldPhraseList) Size() int { return len(l.Phrases) }
