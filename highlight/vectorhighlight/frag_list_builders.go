package vectorhighlight

// FieldFragList is the per-document container of fragments produced by a
// FragListBuilder. Mirrors
// org.apache.lucene.search.vectorhighlight.FieldFragList (the interface).
type FieldFragList interface {
	Add(fragInfo *WeightedFragInfo)
	GetFragInfos() []*WeightedFragInfo
}

// WeightedFragInfo is a single fragment with its boost.
type WeightedFragInfo struct {
	StartOffset int
	EndOffset   int
	TotalBoost  float32
	SubInfos    []WeightedPhraseInfo
}

// SimpleFieldFragList stores fragments in insertion order. Mirrors
// org.apache.lucene.search.vectorhighlight.SimpleFieldFragList.
type SimpleFieldFragList struct {
	FragmentCharSize int
	infos            []*WeightedFragInfo
}

// NewSimpleFieldFragList builds an empty list with the supplied char-size.
func NewSimpleFieldFragList(fragmentCharSize int) *SimpleFieldFragList {
	return &SimpleFieldFragList{FragmentCharSize: fragmentCharSize}
}

// Add records a fragment.
func (l *SimpleFieldFragList) Add(fi *WeightedFragInfo) { l.infos = append(l.infos, fi) }

// GetFragInfos returns the recorded fragments.
func (l *SimpleFieldFragList) GetFragInfos() []*WeightedFragInfo { return l.infos }

var _ FieldFragList = (*SimpleFieldFragList)(nil)

// WeightedFieldFragList stores fragments and keeps them sorted by descending
// TotalBoost. Mirrors
// org.apache.lucene.search.vectorhighlight.WeightedFieldFragList.
type WeightedFieldFragList struct {
	FragmentCharSize int
	infos            []*WeightedFragInfo
}

// NewWeightedFieldFragList builds an empty list.
func NewWeightedFieldFragList(fragmentCharSize int) *WeightedFieldFragList {
	return &WeightedFieldFragList{FragmentCharSize: fragmentCharSize}
}

// Add records a fragment and keeps the list ordered by descending boost.
func (l *WeightedFieldFragList) Add(fi *WeightedFragInfo) {
	// linear insertion preserves stable order for equal boosts
	for i, e := range l.infos {
		if e.TotalBoost < fi.TotalBoost {
			l.infos = append(l.infos[:i], append([]*WeightedFragInfo{fi}, l.infos[i:]...)...)
			return
		}
	}
	l.infos = append(l.infos, fi)
}

// GetFragInfos returns the recorded fragments.
func (l *WeightedFieldFragList) GetFragInfos() []*WeightedFragInfo { return l.infos }

var _ FieldFragList = (*WeightedFieldFragList)(nil)

// FragListBuilder is the contract every frag-list builder implements.
type FragListBuilder interface {
	CreateFieldFragList(list *FieldPhraseList, fragmentCharSize int) FieldFragList
}

// BaseFragListBuilder is the shared scaffolding: it groups phrases that fall
// within fragmentCharSize chars of each other into a single fragment.
// Mirrors org.apache.lucene.search.vectorhighlight.BaseFragListBuilder.
type BaseFragListBuilder struct {
	MarginCharSize int
}

// NewBaseFragListBuilder builds the helper.
func NewBaseFragListBuilder(margin int) *BaseFragListBuilder {
	return &BaseFragListBuilder{MarginCharSize: margin}
}

// CreateFieldFragList implements the shared algorithm and stores the result in
// out — concrete subtypes pre-allocate the underlying list type.
func (b *BaseFragListBuilder) CreateFieldFragList(out FieldFragList, list *FieldPhraseList, fragmentCharSize int) {
	if list == nil || len(list.Phrases) == 0 {
		return
	}
	current := &WeightedFragInfo{
		StartOffset: list.Phrases[0].StartOffset,
		EndOffset:   list.Phrases[0].EndOffset,
		TotalBoost:  list.Phrases[0].TotalBoost,
		SubInfos:    []WeightedPhraseInfo{list.Phrases[0]},
	}
	for _, p := range list.Phrases[1:] {
		if p.EndOffset-current.StartOffset <= fragmentCharSize {
			if p.EndOffset > current.EndOffset {
				current.EndOffset = p.EndOffset
			}
			current.TotalBoost += p.TotalBoost
			current.SubInfos = append(current.SubInfos, p)
			continue
		}
		out.Add(current)
		current = &WeightedFragInfo{
			StartOffset: p.StartOffset,
			EndOffset:   p.EndOffset,
			TotalBoost:  p.TotalBoost,
			SubInfos:    []WeightedPhraseInfo{p},
		}
	}
	out.Add(current)
}

// SimpleFragListBuilder builds a SimpleFieldFragList. Mirrors
// org.apache.lucene.search.vectorhighlight.SimpleFragListBuilder.
type SimpleFragListBuilder struct{ *BaseFragListBuilder }

// NewSimpleFragListBuilder builds the builder.
func NewSimpleFragListBuilder() *SimpleFragListBuilder {
	return &SimpleFragListBuilder{BaseFragListBuilder: NewBaseFragListBuilder(6)}
}

// CreateFieldFragList builds a SimpleFieldFragList.
func (b *SimpleFragListBuilder) CreateFieldFragList(list *FieldPhraseList, fragmentCharSize int) FieldFragList {
	out := NewSimpleFieldFragList(fragmentCharSize)
	b.BaseFragListBuilder.CreateFieldFragList(out, list, fragmentCharSize)
	return out
}

var _ FragListBuilder = (*SimpleFragListBuilder)(nil)

// SingleFragListBuilder collapses every phrase into a single fragment.
// Mirrors org.apache.lucene.search.vectorhighlight.SingleFragListBuilder.
type SingleFragListBuilder struct{}

// CreateFieldFragList collapses every phrase into one fragment.
func (b *SingleFragListBuilder) CreateFieldFragList(list *FieldPhraseList, fragmentCharSize int) FieldFragList {
	out := NewSimpleFieldFragList(fragmentCharSize)
	if list == nil || len(list.Phrases) == 0 {
		return out
	}
	fi := &WeightedFragInfo{StartOffset: list.Phrases[0].StartOffset, EndOffset: list.Phrases[0].EndOffset}
	for _, p := range list.Phrases {
		fi.TotalBoost += p.TotalBoost
		if p.EndOffset > fi.EndOffset {
			fi.EndOffset = p.EndOffset
		}
		fi.SubInfos = append(fi.SubInfos, p)
	}
	out.Add(fi)
	return out
}

var _ FragListBuilder = (*SingleFragListBuilder)(nil)

// WeightedFragListBuilder builds a WeightedFieldFragList. Mirrors
// org.apache.lucene.search.vectorhighlight.WeightedFragListBuilder.
type WeightedFragListBuilder struct{ *BaseFragListBuilder }

// NewWeightedFragListBuilder builds the builder.
func NewWeightedFragListBuilder() *WeightedFragListBuilder {
	return &WeightedFragListBuilder{BaseFragListBuilder: NewBaseFragListBuilder(6)}
}

// CreateFieldFragList builds a WeightedFieldFragList.
func (b *WeightedFragListBuilder) CreateFieldFragList(list *FieldPhraseList, fragmentCharSize int) FieldFragList {
	out := NewWeightedFieldFragList(fragmentCharSize)
	b.BaseFragListBuilder.CreateFieldFragList(out, list, fragmentCharSize)
	return out
}

var _ FragListBuilder = (*WeightedFragListBuilder)(nil)
