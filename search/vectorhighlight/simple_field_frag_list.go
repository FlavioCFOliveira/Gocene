package vectorhighlight

// SimpleFieldFragList is a simple implementation of FieldFragList.
type SimpleFieldFragList struct {
	baseFieldFragList
}

func NewSimpleFieldFragList(fragCharSize int) *SimpleFieldFragList {
	return &SimpleFieldFragList{}
}

func (s *SimpleFieldFragList) Add(startOffset, endOffset int, phraseInfoList []*WeightedPhraseInfo) {
	totalBoost := float32(0)
	subInfos := make([]SubInfo, 0, len(phraseInfoList))
	for _, phraseInfo := range phraseInfoList {
		subInfos = append(subInfos, SubInfo{
			Text:         phraseInfo.getText(),
			TermsOffsets: phraseInfo.GetTermsOffsets(),
			Seqnum:       phraseInfo.Seqnum,
			Boost:        phraseInfo.GetBoost(),
		})
		totalBoost += phraseInfo.GetBoost()
	}
	s.fragInfos = append(s.fragInfos, &WeightedFragInfo{
		StartOffset: startOffset,
		EndOffset:   endOffset,
		SubInfos:    subInfos,
		TotalBoost:  totalBoost,
	})
}
