package vectorhighlight

import "math"

// WeightedFieldFragList is a weighted implementation of FieldFragList.
type WeightedFieldFragList struct {
	baseFieldFragList
}

func NewWeightedFieldFragList(fragCharSize int) *WeightedFieldFragList {
	return &WeightedFieldFragList{}
}

func (w *WeightedFieldFragList) Add(startOffset, endOffset int, phraseInfoList []*WeightedPhraseInfo) {
	tempSubInfos := make([]SubInfo, 0, len(phraseInfoList))
	distinctTerms := make(map[string]struct{})
	length := 0

	for _, phraseInfo := range phraseInfoList {
		phraseTotalBoost := float32(0)
		for _, ti := range phraseInfo.GetTermsInfos() {
			if _, ok := distinctTerms[ti.Text]; !ok {
				distinctTerms[ti.Text] = struct{}{}
				phraseTotalBoost += ti.Weight * phraseInfo.GetBoost()
			}
			length++
		}
		tempSubInfos = append(tempSubInfos, SubInfo{
			Text:         phraseInfo.getText(),
			TermsOffsets: phraseInfo.GetTermsOffsets(),
			Seqnum:       phraseInfo.Seqnum,
			Boost:        phraseTotalBoost,
		})
	}

	norm := float32(float64(length) * (1.0 / math.Sqrt(float64(length))))

	totalBoost := float32(0)
	realSubInfos := make([]SubInfo, 0, len(tempSubInfos))
	for _, tempSubInfo := range tempSubInfos {
		subInfoBoost := tempSubInfo.Boost * norm
		realSubInfos = append(realSubInfos, SubInfo{
			Text:         tempSubInfo.Text,
			TermsOffsets: tempSubInfo.TermsOffsets,
			Seqnum:       tempSubInfo.Seqnum,
			Boost:        subInfoBoost,
		})
		totalBoost += subInfoBoost
	}

	w.fragInfos = append(w.fragInfos, &WeightedFragInfo{
		StartOffset: startOffset,
		EndOffset:   endOffset,
		SubInfos:    realSubInfos,
		TotalBoost:  totalBoost,
	})
}
