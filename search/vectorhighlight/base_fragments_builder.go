package vectorhighlight

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/highlight"
)

var ColoredPreTags = []string{
	"<b style=\"background:yellow\">", "<b style=\"background:lawngreen\">",
	"<b style=\"background:aquamarine\">",
	"<b style=\"background:magenta\">", "<b style=\"background:palegreen\">",
	"<b style=\"background:coral\">",
	"<b style=\"background:wheat\">", "<b style=\"background:khaki\">",
	"<b style=\"background:lime\">",
	"<b style=\"background:deepskyblue\">", "<b style=\"background:deeppink\">",
	"<b style=\"background:salmon\">",
	"<b style=\"background:peachpuff\">", "<b style=\"background:violet\">",
	"<b style=\"background:mediumpurple\">",
	"<b style=\"background:palegoldenrod\">", "<b style=\"background:darkkhaki\">",
	"<b style=\"background:springgreen\">",
	"<b style=\"background:turquoise\">", "<b style=\"background:powderblue\">",
}

var ColoredPostTags = []string{"</b>"}

type BaseFragmentsBuilder struct {
	PreTags                       []string
	PostTags                      []string
	MultiValuedSeparator          rune
	BoundaryScanner               BoundaryScanner
	DiscreteMultiValueHighlighting bool
	getWeightedFragInfoList       func(src []*WeightedFragInfo) []*WeightedFragInfo
}

func NewBaseFragmentsBuilder(preTags, postTags []string, bs BoundaryScanner) *BaseFragmentsBuilder {
	if preTags == nil {
		preTags = []string{"<b>"}
	}
	if postTags == nil {
		postTags = []string{"</b>"}
	}
	if bs == nil {
		bs = NewSimpleBoundaryScanner()
	}
	return &BaseFragmentsBuilder{
		PreTags:              preTags,
		PostTags:             postTags,
		MultiValuedSeparator: ' ',
		BoundaryScanner:      bs,
	}
}

func (b *BaseFragmentsBuilder) SetGetWeightedFragInfoList(f func(src []*WeightedFragInfo) []*WeightedFragInfo) {
	b.getWeightedFragInfoList = f
}

func (b *BaseFragmentsBuilder) CreateFragment(reader index.IndexReader, docID int, fieldName string, fieldFragList *FieldFragList) (string, error) {
	frags, err := b.CreateFragments(reader, docID, fieldName, fieldFragList, 1)
	if err != nil {
		return "", err
	}
	if len(frags) == 0 {
		return "", nil
	}
	return frags[0], nil
}

func (b *BaseFragmentsBuilder) CreateFragments(reader index.IndexReader, docID int, fieldName string, fieldFragList *FieldFragList, maxNumFragments int) ([]string, error) {
	return b.CreateFragmentsWithTags(reader, docID, fieldName, fieldFragList, maxNumFragments, b.PreTags, b.PostTags, highlight.NewDefaultEncoder())
}

func (b *BaseFragmentsBuilder) CreateFragmentWithTags(reader index.IndexReader, docID int, fieldName string, fieldFragList *FieldFragList, preTags, postTags []string, encoder highlight.Encoder) (string, error) {
	frags, err := b.CreateFragmentsWithTags(reader, docID, fieldName, fieldFragList, 1, preTags, postTags, encoder)
	if err != nil {
		return "", err
	}
	if len(frags) == 0 {
		return "", nil
	}
	return frags[0], nil
}

func (b *BaseFragmentsBuilder) CreateFragmentsWithTags(reader index.IndexReader, docID int, fieldName string, fieldFragList *FieldFragList, maxNumFragments int, preTags, postTags []string, encoder highlight.Encoder) ([]string, error) {
	if maxNumFragments < 0 {
		return nil, fmt.Errorf("maxNumFragments(%d) must be positive number", maxNumFragments)
	}

	fragInfos := fieldFragList.GetFragInfos()
	values, err := b.getFields(reader, docID, fieldName)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, nil
	}

	if b.DiscreteMultiValueHighlighting && len(values) > 1 {
		fragInfos = b.discreteMultiValueHighlighting(fragInfos, values)
	}

	if b.getWeightedFragInfoList != nil {
		fragInfos = b.getWeightedFragInfoList(fragInfos)
	}

	limitFragments := maxNumFragments
	if len(fragInfos) < limitFragments {
		limitFragments = len(fragInfos)
	}

	fragments := make([]string, 0, limitFragments)
	var buffer strings.Builder
	nextValueIndex := 0
	for n := 0; n < limitFragments; n++ {
		fragInfo := fragInfos[n]
		fragments = append(fragments, b.makeFragment(&buffer, &nextValueIndex, values, fragInfo, preTags, postTags, encoder))
	}
	return fragments, nil
}

func (b *BaseFragmentsBuilder) getFields(reader index.IndexReader, docID int, fieldName string) ([]string, error) {
	fields := make([]string, 0)

	// We need to get stored fields.
	// In Gocene, we can use reader.StoredFields().Document(docID, visitor)
	// But we only want the field with fieldName.

	err := reader.StoredFields().Document(docID, func(fieldInfo index.FieldInfo, value string) {
		if fieldInfo.Name == fieldName {
			fields = append(fields, value)
		}
	})
	if err != nil {
		return nil, err
	}

	return fields, nil
}

func (b *BaseFragmentsBuilder) makeFragment(buffer *strings.Builder, index *int, values []string, fragInfo *WeightedFragInfo, preTags, postTags []string, encoder highlight.Encoder) string {
	var fragment strings.Builder
	s := fragInfo.StartOffset
	modifiedStartOffset := s

	src := b.getFragmentSourceMSO(buffer, index, values, s, fragInfo.EndOffset, &modifiedStartOffset)
	srcIndex := 0
	for _, subInfo := range fragInfo.SubInfos {
		for _, to := range subInfo.TermsOffsets {
			fragment.WriteString(encoder.EncodeText(src[srcIndex : to.StartOffset-modifiedStartOffset]))
			fragment.WriteString(b.getPreTag(preTags, subInfo.Seqnum))
			fragment.WriteString(encoder.EncodeText(src[to.StartOffset-modifiedStartOffset : to.EndOffset-modifiedStartOffset]))
			fragment.WriteString(b.getPostTag(postTags, subInfo.Seqnum))
			srcIndex = to.EndOffset - modifiedStartOffset
		}
	}
	fragment.WriteString(encoder.EncodeText(src[srcIndex:]))
	return fragment.String()
}

func (b *BaseFragmentsBuilder) getFragmentSourceMSO(buffer *strings.Builder, index *int, values []string, startOffset, endOffset int, modifiedStartOffset *int) string {
	for buffer.Len() < endOffset && *index < len(values) {
		buffer.WriteString(values[*index])
		buffer.WriteRune(b.MultiValuedSeparator)
		*index++
	}
	bufferLength := buffer.Len()

	// In Java: if (values[index[0] - 1].fieldType().tokenized()) { bufferLength--; }
	// We don't have fieldType here, but typically stored fields for highlighting are tokenized.
	// Let's assume it is tokenized and remove the trailing separator.
	if len(values) > 0 {
		bufferLength--
	}

	eo := bufferLength
	if bufferLength < endOffset {
		eo = bufferLength
	} else {
		eo = b.BoundaryScanner.FindEndOffset(buffer.String(), endOffset)
	}

	*modifiedStartOffset = b.BoundaryScanner.FindStartOffset(buffer.String(), startOffset)
	return buffer.String()[*modifiedStartOffset : eo]
}

func (b *BaseFragmentsBuilder) discreteMultiValueHighlighting(fragInfos []*WeightedFragInfo, fields []string) []*WeightedFragInfo {
	fieldNameToFragInfos := make(map[string][]*WeightedFragInfo)
	for _, field := range fields {
		fieldNameToFragInfos[field] = make([]*WeightedFragInfo, 0)
	}

	for _, fragInfo := range fragInfos {
		fieldStart := 0
		fieldEnd := 0
		for _, field := range fields {
			if field == "" {
				fieldEnd++
				continue
			}
			fieldStart = fieldEnd
			fieldEnd += len(field) + 1

			if fragInfo.StartOffset >= fieldStart &&
				fragInfo.EndOffset >= fieldStart &&
				fragInfo.StartOffset <= fieldEnd &&
				fragInfo.EndOffset <= fieldEnd {
				fieldNameToFragInfos[field] = append(fieldNameToFragInfos[field], fragInfo)
				goto nextFragInfo
			}

			if len(fragInfo.SubInfos) == 0 {
				continue
			}

			firstToffs := fragInfo.SubInfos[0].TermsOffsets[0]
			if fragInfo.StartOffset >= fieldEnd || firstToffs.StartOffset >= fieldEnd {
				continue
			}

			fragStart := fieldStart
			if fragInfo.StartOffset > fieldStart && fragInfo.StartOffset < fieldEnd {
				fragStart = fragInfo.StartOffset
			}

			fragEnd := fieldEnd
			if fragInfo.EndOffset > fieldStart && fragInfo.EndOffset < fieldEnd {
				fragEnd = fragInfo.EndOffset
			}

			subInfos := make([]SubInfo, 0)
			boost := float32(0)
			for _, subInfo := range fragInfo.SubInfos {
				toffsList := make([]Toffs, 0)
				for _, toffs := range subInfo.TermsOffsets {
					if toffs.StartOffset >= fieldEnd {
						break
					}
					startsAfterField := toffs.StartOffset >= fieldStart
					endsBeforeField := toffs.EndOffset < fieldEnd
					if startsAfterField && endsBeforeField {
						toffsList = append(toffsList, toffs)
					} else if startsAfterField {
						toffsList = append(toffsList, Toffs{toffs.StartOffset, fieldEnd - 1})
					} else if endsBeforeField {
						toffsList = append(toffsList, Toffs{fieldStart, toffs.EndOffset})
					} else {
						toffsList = append(toffsList, Toffs{fieldStart, fieldEnd - 1})
					}
				}
				if len(toffsList) > 0 {
					subInfos = append(subInfos, SubInfo{
						Text:         subInfo.Text,
						TermsOffsets: toffsList,
						Seqnum:       subInfo.Seqnum,
						Boost:        subInfo.Boost,
					})
					boost += subInfo.Boost
				}
			}
			weightedFragInfo := &WeightedFragInfo{
				StartOffset: fragStart,
				EndOffset:   fragEnd,
				SubInfos:    subInfos,
				TotalBoost:  boost,
			}
			fieldNameToFragInfos[field] = append(fieldNameToFragInfos[field], weightedFragInfo)
		}
	nextFragInfo:
	}

	result := make([]*WeightedFragInfo, 0)
	for _, weightedFragInfos := range fieldNameToFragInfos {
		result = append(result, weightedFragInfos...)
	}

	// Sort by start offset
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].StartOffset > result[j].StartOffset {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

func (b *BaseFragmentsBuilder) getPreTag(preTags []string, num int) string {
	if len(preTags) == 0 {
		return ""
	}
	return preTags[num%len(preTags)]
}

func (b *BaseFragmentsBuilder) getPostTag(postTags []string, num int) string {
	if len(postTags) == 0 {
		return ""
	}
	return postTags[num%len(postTags)]
}
