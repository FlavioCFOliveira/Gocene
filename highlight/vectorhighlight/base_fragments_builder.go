package vectorhighlight

import "strings"

// FragmentsBuilder is the contract every fragment renderer implements.
// Mirrors org.apache.lucene.search.vectorhighlight.FragmentsBuilder.
type FragmentsBuilder interface {
	CreateFragments(text string, list FieldFragList, maxFragments int) []string
}

// BaseFragmentsBuilder provides the default rendering: wrap each
// WeightedPhraseInfo's start/end in Pre/Post tags. Mirrors
// org.apache.lucene.search.vectorhighlight.BaseFragmentsBuilder.
type BaseFragmentsBuilder struct {
	Pre  string
	Post string
}

// NewBaseFragmentsBuilder builds the helper with the supplied tag pair.
func NewBaseFragmentsBuilder(pre, post string) *BaseFragmentsBuilder {
	if pre == "" {
		pre = "<b>"
	}
	if post == "" {
		post = "</b>"
	}
	return &BaseFragmentsBuilder{Pre: pre, Post: post}
}

// CreateFragments renders up to maxFragments per-fragment snippets.
func (b *BaseFragmentsBuilder) CreateFragments(text string, list FieldFragList, maxFragments int) []string {
	if list == nil {
		return nil
	}
	infos := list.GetFragInfos()
	if maxFragments > 0 && len(infos) > maxFragments {
		infos = infos[:maxFragments]
	}
	out := make([]string, 0, len(infos))
	for _, fi := range infos {
		out = append(out, b.renderFragment(text, fi))
	}
	return out
}

func (b *BaseFragmentsBuilder) renderFragment(text string, fi *WeightedFragInfo) string {
	var sb strings.Builder
	if fi.StartOffset < 0 {
		fi.StartOffset = 0
	}
	if fi.EndOffset > len(text) {
		fi.EndOffset = len(text)
	}
	cursor := fi.StartOffset
	for _, p := range fi.SubInfos {
		from := p.StartOffset
		to := p.EndOffset
		if from < cursor {
			from = cursor
		}
		if to > fi.EndOffset {
			to = fi.EndOffset
		}
		if from > cursor {
			sb.WriteString(text[cursor:from])
		}
		if to > from {
			sb.WriteString(b.Pre)
			sb.WriteString(text[from:to])
			sb.WriteString(b.Post)
			cursor = to
		}
	}
	if cursor < fi.EndOffset {
		sb.WriteString(text[cursor:fi.EndOffset])
	}
	return sb.String()
}

var _ FragmentsBuilder = (*BaseFragmentsBuilder)(nil)
