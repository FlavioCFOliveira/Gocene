package vectorhighlight

import "sort"

// ScoreOrderFragmentsBuilder is an implementation of FragmentsBuilder that outputs score-order fragments.
type ScoreOrderFragmentsBuilder struct {
	*BaseFragmentsBuilder
}

func NewScoreOrderFragmentsBuilder() *ScoreOrderFragmentsBuilder {
	sfb := &ScoreOrderFragmentsBuilder{
		BaseFragmentsBuilder: NewBaseFragmentsBuilder(nil, nil, nil),
	}
	sfb.BaseFragmentsBuilder.SetGetWeightedFragInfoList(scoreOrderFragmentsBuilderGetWeightedFragInfoList)
	return sfb
}

func NewScoreOrderFragmentsBuilderWithTags(preTags, postTags []string) *ScoreOrderFragmentsBuilder {
	sfb := &ScoreOrderFragmentsBuilder{
		BaseFragmentsBuilder: NewBaseFragmentsBuilder(preTags, postTags, nil),
	}
	sfb.BaseFragmentsBuilder.SetGetWeightedFragInfoList(scoreOrderFragmentsBuilderGetWeightedFragInfoList)
	return sfb
}

func NewScoreOrderFragmentsBuilderWithBoundaryScanner(bs BoundaryScanner) *ScoreOrderFragmentsBuilder {
	sfb := &ScoreOrderFragmentsBuilder{
		BaseFragmentsBuilder: NewBaseFragmentsBuilder(nil, nil, bs),
	}
	sfb.BaseFragmentsBuilder.SetGetWeightedFragInfoList(scoreOrderFragmentsBuilderGetWeightedFragInfoList)
	return sfb
}

func NewScoreOrderFragmentsBuilderWithTagsAndBoundaryScanner(preTags, postTags []string, bs BoundaryScanner) *ScoreOrderFragmentsBuilder {
	sfb := &ScoreOrderFragmentsBuilder{
		BaseFragmentsBuilder: NewBaseFragmentsBuilder(preTags, postTags, bs),
	}
	sfb.BaseFragmentsBuilder.SetGetWeightedFragInfoList(scoreOrderFragmentsBuilderGetWeightedFragInfoList)
	return sfb
}

func scoreOrderFragmentsBuilderGetWeightedFragInfoList(src []*WeightedFragInfo) []*WeightedFragInfo {
	sort.Slice(src, func(i, j int) bool {
		if src[i].TotalBoost != src[j].TotalBoost {
			return src[i].TotalBoost > src[j].TotalBoost
		}
		return src[i].StartOffset < src[j].StartOffset
	})
	return src
}
