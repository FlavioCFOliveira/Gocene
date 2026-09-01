package vectorhighlight

import (
	"fmt"
)

// WeightedFragInfo represents a fragment's info.
type WeightedFragInfo struct {
	SubInfos    []SubInfo
	TotalBoost  float32
	StartOffset int
	EndOffset   int
}

func (wfi *WeightedFragInfo) String() string {
	var subInfosStr string
	for _, si := range wfi.SubInfos {
		subInfosStr += si.String()
	}
	return fmt.Sprintf("subInfos(%s)/%f(%d,%d)", subInfosStr, wfi.TotalBoost, wfi.StartOffset, wfi.EndOffset)
}

type SubInfo struct {
	Text         string
	TermsOffsets []Toffs
	Seqnum       int
	Boost        float32
}

func (si SubInfo) String() string {
	var toffsStr string
	for _, to := range si.TermsOffsets {
		toffsStr += fmt.Sprintf("(%d,%d)", to.StartOffset, to.EndOffset)
	}
	return fmt.Sprintf("%s%s", si.Text, toffsStr)
}

// FieldFragList has a list of "frag info" that is used by FragmentsBuilder class to create fragments (snippets).
type FieldFragList interface {
	Add(startOffset, endOffset int, phraseInfoList []*WeightedPhraseInfo)
	GetFragInfos() []*WeightedFragInfo
}

type baseFieldFragList struct {
	fragInfos []*WeightedFragInfo
}

func (b *baseFieldFragList) GetFragInfos() []*WeightedFragInfo {
	return b.fragInfos
}
