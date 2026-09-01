package vectorhighlight

import (
	"fmt"
)

const (
	MarginDefault           = 6
	MinFragCharSizeFactor   = 3
)

type BaseFragListBuilder struct {
	Margin         int
	MinFragCharSize int
}

func NewBaseFragListBuilder(margin int) *BaseFragListBuilder {
	if margin < 0 {
		panic(fmt.Sprintf("margin(%d) is too small. It must be 0 or higher.", margin))
	}
	return &BaseFragListBuilder{
		Margin:          margin,
		MinFragCharSize: max(1, margin*MinFragCharSizeFactor),
	}
}

func (b *BaseFragListBuilder) createFieldFragList(fieldPhraseList *FieldPhraseList, fieldFragList FieldFragList, fragCharSize int) FieldFragList {
	if fragCharSize < b.MinFragCharSize {
		panic(fmt.Sprintf("fragCharSize(%d) is too small. It must be %d or higher.", fragCharSize, b.MinFragCharSize))
	}

	var wpil []*WeightedPhraseInfo
	queue := newIteratorQueue(fieldPhraseList.PhraseList)
	var phraseInfo *WeightedPhraseInfo
	startOffset := 0

	for {
		phraseInfo = queue.top()
		if phraseInfo == nil {
			break
		}

		if phraseInfo.GetStartOffset() < startOffset {
			queue.removeTop()
			continue
		}

		wpil = wpil[:0]
		currentPhraseStartOffset := phraseInfo.GetStartOffset()
		currentPhraseEndOffset := phraseInfo.GetEndOffset()
		spanStart := max(currentPhraseStartOffset-b.Margin, startOffset)
		spanEnd := max(currentPhraseEndOffset, spanStart+fragCharSize)

		if b.acceptPhrase(queue.removeTop(), currentPhraseEndOffset-currentPhraseStartOffset, fragCharSize) {
			wpil = append(wpil, phraseInfo)
		}

		for {
			phraseInfo = queue.top()
			if phraseInfo == nil {
				break
			}
			if phraseInfo.GetEndOffset() <= spanEnd {
				currentPhraseEndOffset = phraseInfo.GetEndOffset()
				if b.acceptPhrase(queue.removeTop(), currentPhraseEndOffset-currentPhraseStartOffset, fragCharSize) {
					wpil = append(wpil, phraseInfo)
				}
			} else {
				break
			}
		}

		if len(wpil) == 0 {
			continue
		}

		matchLen := currentPhraseEndOffset - currentPhraseStartOffset
		newMargin := max(0, (fragCharSize-matchLen)/2)
		spanStart = currentPhraseStartOffset - newMargin
		if spanStart < startOffset {
			spanStart = startOffset
		}
		spanEnd = spanStart + max(matchLen, fragCharSize)
		startOffset = spanEnd
		fieldFragList.Add(spanStart, spanEnd, wpil)
	}

	return fieldFragList
}

func (b *BaseFragListBuilder) acceptPhrase(info *WeightedPhraseInfo, matchLength, fragCharSize int) bool {
	return len(info.TermsOffsets) <= 1 || matchLength <= fragCharSize
}

type iteratorQueue struct {
	iter   []*WeightedPhraseInfo
	cursor int
	top    *WeightedPhraseInfo
}

func newIteratorQueue(list []*WeightedPhraseInfo) *iteratorQueue {
	iq := &iteratorQueue{
		iter: list,
	}
	iq.removeTop()
	return iq
}

func (iq *iteratorQueue) top() *WeightedPhraseInfo {
	return iq.top
}

func (iq *iteratorQueue) removeTop() *WeightedPhraseInfo {
	currentTop := iq.top
	if iq.cursor < len(iq.iter) {
		iq.top = iq.iter[iq.cursor]
		iq.cursor++
	} else {
		iq.top = nil
	}
	return currentTop
}
