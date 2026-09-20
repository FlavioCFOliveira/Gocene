package vectorhighlight

import (
	"sort"
)

// WeightedPhraseInfo represents the list of term offsets and boost for some text.
type WeightedPhraseInfo struct {
	TermsOffsets []Toffs
	Boost        float32
	Seqnum       int
	termsInfos   []*TermInfo
}

type Toffs struct {
	StartOffset int
	EndOffset   int
}

func (wpi *WeightedPhraseInfo) getText() string {
	var text string
	for _, ti := range wpi.termsInfos {
		text += ti.Text
	}
	return text
}

func (wpi *WeightedPhraseInfo) GetTermsOffsets() []Toffs {
	return wpi.TermsOffsets
}

func (wpi *WeightedPhraseInfo) GetBoost() float32 {
	return wpi.Boost
}

func (wpi *WeightedPhraseInfo) GetTermsInfos() []*TermInfo {
	return wpi.termsInfos
}

func NewWeightedPhraseInfo(terms []*TermInfo, boost float32, seqnum int) *WeightedPhraseInfo {
	wpi := &WeightedPhraseInfo{
		Boost:  boost,
		Seqnum: seqnum,
	}

	wpi.termsInfos = make([]*TermInfo, len(terms))
	copy(wpi.termsInfos, terms)

	wpi.TermsOffsets = make([]Toffs, 0, len(terms))
	ti := terms[0]
	wpi.TermsOffsets = append(wpi.TermsOffsets, Toffs{ti.StartOffset, ti.EndOffset})
	if len(terms) == 1 {
		return wpi
	}

	pos := ti.Position
	for i := 1; i < len(terms); i++ {
		ti = terms[i]
		if ti.Position-pos == 1 {
			last := &wpi.TermsOffsets[len(wpi.TermsOffsets)-1]
			last.EndOffset = ti.EndOffset
		} else {
			wpi.TermsOffsets = append(wpi.TermsOffsets, Toffs{ti.StartOffset, ti.EndOffset})
		}
		pos = ti.Position
	}
	return wpi
}

func NewWeightedPhraseInfoFromMerge(toMerge []*WeightedPhraseInfo) *WeightedPhraseInfo {
	if len(toMerge) == 0 {
		panic("toMerge must contain at least one WeightedPhraseInfo")
	}

	first := toMerge[0]
	var termsInfos []*TermInfo
	seqnum := first.Seqnum
	boost := float32(0)

	allToffs := make([][]Toffs, len(toMerge))
	for i, info := range toMerge {
		boost += info.Boost
		termsInfos = append(termsInfos, info.termsInfos...)
		allToffs[i] = info.TermsOffsets
	}

	// Merge overlapping Toffs
	var mergedToffs []Toffs

	// We need to sort all Toffs from all sources.
	// In Java, it uses MergedIterator. We'll flatten and sort.
	var flatToffs []Toffs
	for _, offsets := range allToffs {
		flatToffs = append(flatToffs, offsets...)
	}
	sort.Slice(flatToffs, func(i, j int) bool {
		if flatToffs[i].StartOffset != flatToffs[j].StartOffset {
			return flatToffs[i].StartOffset < flatToffs[j].StartOffset
		}
		return flatToffs[i].EndOffset < flatToffs[j].EndOffset
	})

	if len(flatToffs) > 0 {
		work := flatToffs[0]
		for i := 1; i < len(flatToffs); i++ {
			current := flatToffs[i]
			if current.StartOffset <= work.EndOffset {
				if current.EndOffset > work.EndOffset {
					work.EndOffset = current.EndOffset
				}
			} else {
				mergedToffs = append(mergedToffs, work)
				work = current
			}
		}
		mergedToffs = append(mergedToffs, work)
	}

	return &WeightedPhraseInfo{
		TermsOffsets: mergedToffs,
		Boost:        boost,
		Seqnum:       seqnum,
		termsInfos:   termsInfos,
	}
}

func (wpi *WeightedPhraseInfo) IsOffsetOverlap(other *WeightedPhraseInfo) bool {
	so := wpi.GetStartOffset()
	eo := wpi.GetEndOffset()
	oso := other.GetStartOffset()
	oeo := other.GetEndOffset()
	if so <= oso && oso < eo {
		return true
	}
	if so < oeo && oeo <= eo {
		return true
	}
	if oso <= so && so < oeo {
		return true
	}
	if oso < eo && eo <= oeo {
		return true
	}
	return false
}

func (wpi *WeightedPhraseInfo) GetStartOffset() int {
	return wpi.TermsOffsets[0].StartOffset
}

func (wpi *WeightedPhraseInfo) GetEndOffset() int {
	return wpi.TermsOffsets[len(wpi.TermsOffsets)-1].EndOffset
}

// FieldPhraseList has a list of WeightedPhraseInfo that is used by FragListBuilder to create a FieldFragList object.
type FieldPhraseList struct {
	PhraseList []*WeightedPhraseInfo
}

func NewFieldPhraseList(fieldTermStack *FieldTermStack, fieldQuery *FieldQuery) *FieldPhraseList {
	return NewFieldPhraseListWithLimit(fieldTermStack, fieldQuery, 2147483647)
}

func NewFieldPhraseListWithLimit(fieldTermStack *FieldTermStack, fieldQuery *FieldQuery, phraseLimit int) *FieldPhraseList {
	fpl := &FieldPhraseList{
		PhraseList: make([]*WeightedPhraseInfo, 0),
	}
	field := fieldTermStack.GetFieldName()

	var phraseCandidate []*TermInfo
	var currMap *QueryPhraseMap
	var nextMap *QueryPhraseMap

	for !fieldTermStack.IsEmpty() && len(fpl.PhraseList) < phraseLimit {
		phraseCandidate = phraseCandidate[:0]

		var ti *TermInfo
		var first *TermInfo

		ti = fieldTermStack.Pop()
		first = ti
		currMap = fieldQuery.GetFieldTermMap(field, ti.Text)
		for currMap == nil && ti.Next != first {
			ti = ti.Next
			currMap = fieldQuery.GetFieldTermMap(field, ti.Text)
		}

		if currMap == nil {
			continue
		}

		phraseCandidate = append(phraseCandidate, ti)
		for {
			ti = fieldTermStack.Pop()
			nextMap = nil
			if ti != nil {
				nextMap = currMap.GetTermMap(ti.Text)
				for nextMap == nil && ti.Next != first {
					ti = ti.Next
					nextMap = currMap.GetTermMap(ti.Text)
				}
			}

			if ti == nil || nextMap == nil {
				if ti != nil {
					fieldTermStack.Push(ti)
				}
				if currMap.IsValidTermOrPhrase(phraseCandidate) {
					fpl.addIfNoOverlap(NewWeightedPhraseInfo(phraseCandidate, currMap.Boost, currMap.TermOrPhraseNumber))
				} else {
					for len(phraseCandidate) > 1 {
						tiLast := phraseCandidate[len(phraseCandidate)-1]
						phraseCandidate = phraseCandidate[:len(phraseCandidate)-1]
						fieldTermStack.Push(tiLast)
						currMap = fieldQuery.SearchPhrase(field, phraseCandidate)
						if currMap != nil {
							fpl.addIfNoOverlap(NewWeightedPhraseInfo(phraseCandidate, currMap.Boost, currMap.TermOrPhraseNumber))
							break
						}
					}
				}
				break
			} else {
				phraseCandidate = append(phraseCandidate, ti)
				currMap = nextMap
			}
		}
	}

	return fpl
}

func NewFieldPhraseListFromMerge(toMerge []*FieldPhraseList) *FieldPhraseList {
	var allInfos []*WeightedPhraseInfo
	for _, fpl := range toMerge {
		allInfos = append(allInfos, fpl.PhraseList...)
	}

	sort.Slice(allInfos, func(i, j int) bool {
		if allInfos[i].GetStartOffset() != allInfos[j].GetStartOffset() {
			return allInfos[i].GetStartOffset() < allInfos[j].GetStartOffset()
		}
		if allInfos[i].GetEndOffset() != allInfos[j].GetEndOffset() {
			return allInfos[i].GetEndOffset() < allInfos[j].GetEndOffset()
		}
		return allInfos[i].Boost < allInfos[j].Boost
	})

	fpl := &FieldPhraseList{
		PhraseList: make([]*WeightedPhraseInfo, 0),
	}

	if len(allInfos) == 0 {
		return fpl
	}

	var work []*WeightedPhraseInfo
	first := allInfos[0]
	work = append(work, first)
	workEndOffset := first.GetEndOffset()

	for i := 1; i < len(allInfos); i++ {
		current := allInfos[i]
		if current.GetStartOffset() <= workEndOffset {
			if current.GetEndOffset() > workEndOffset {
				workEndOffset = current.GetEndOffset()
			}
			work = append(work, current)
		} else {
			if len(work) == 1 {
				fpl.PhraseList = append(fpl.PhraseList, work[0])
				work = []*WeightedPhraseInfo{current}
			} else {
				fpl.PhraseList = append(fpl.PhraseList, NewWeightedPhraseInfoFromMerge(work))
				work = []*WeightedPhraseInfo{current}
			}
			workEndOffset = current.GetEndOffset()
		}
	}

	if len(work) == 1 {
		fpl.PhraseList = append(fpl.PhraseList, work[0])
	} else if len(work) > 1 {
		fpl.PhraseList = append(fpl.PhraseList, NewWeightedPhraseInfoFromMerge(work))
	}

	return fpl
}

func (fpl *FieldPhraseList) addIfNoOverlap(wpi *WeightedPhraseInfo) {
	for _, existWpi := range fpl.PhraseList {
		if existWpi.IsOffsetOverlap(wpi) {
			existWpi.termsInfos = append(existWpi.termsInfos, wpi.termsInfos...)
			existWpi.Boost += wpi.Boost
			return
		}
	}
	fpl.PhraseList = append(fpl.PhraseList, wpi)
}
