package vectorhighlight

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/highlight"
	"github.com/FlavioCFOliveira/Gocene/index"
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
	PreTags                        []string
	PostTags                       []string
	MultiValuedSeparator           rune
	BoundaryScanner                BoundaryScanner
	DiscreteMultiValueHighlighting bool
	getWeightedFragInfoList        func(src []*WeightedFragInfo) []*WeightedFragInfo
}

func NewBaseFragmentsBuilder(preTags, postTags []string, bs BoundaryScanner) *BaseFragmentsBuilder {
	if preTags == nil {
		preTags = []string{"<b>"}
	}
	if postTags == nil {
		postTags = []string{"</b>"}
	}
	if bs == nil {
		bs = NewSimpleBoundaryScanner(DefaultMaxScan, DefaultBoundaryChars)
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

func (b *BaseFragmentsBuilder) CreateFragment(reader index.IndexReader, docID int, fieldName string, fieldFragList FieldFragList) (string, error) {
	frags, err := b.CreateFragments(reader, docID, fieldName, fieldFragList, 1)
	if err != nil {
		return "", err
	}
	if len(frags) == 0 {
		return "", nil
	}
	return frags[0], nil
}

func (b *BaseFragmentsBuilder) CreateFragments(reader index.IndexReader, docID int, fieldName string, fieldFragList FieldFragList, maxNumFragments int) ([]string, error) {
	return b.CreateFragmentsWithTags(reader, docID, fieldName, fieldFragList, maxNumFragments, b.PreTags, b.PostTags, highlight.NewDefaultEncoder())
}

func (b *BaseFragmentsBuilder) CreateFragmentWithTags(reader index.IndexReader, docID int, fieldName string, fieldFragList FieldFragList, preTags, postTags []string, encoder highlight.Encoder) (string, error) {
	frags, err := b.CreateFragmentsWithTags(reader, docID, fieldName, fieldFragList, 1, preTags, postTags, encoder)
	if err != nil {
		return "", err
	}
	if len(frags) == 0 {
		return "", nil
	}
	return frags[0], nil
}

func (b *BaseFragmentsBuilder) CreateFragmentsWithTags(reader index.IndexReader, docID int, fieldName string, fieldFragList FieldFragList, maxNumFragments int, preTags, postTags []string, encoder highlight.Encoder) ([]string, error) {
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

// getFieldsVisitor collects the stored values of a single field.
//
// Renders the anonymous StoredFieldVisitor of
// BaseFragmentsBuilder.getFields(IndexReader, int, String).
//
// PORT NOTE: Apache Lucene 10.5.0 receives the FieldInfo in
// StoredFieldVisitor.stringField and copies fieldInfo.hasTermVectors() onto the
// FieldType it builds. spi.StoredFieldVisitor carries only the field name, so
// the term-vector bit cannot be reproduced here. Nothing in this class reads
// it: getFragmentSourceMSO consults only FieldType.tokenized(), which
// TextField.TYPE_STORED sets and setStoreTermVectors never changes.
type getFieldsVisitor struct {
	fieldName string
	fields    []*document.Field
	err       error
}

// NeedsField reports whether the visitor wishes to receive the named field.
//
// Mirrors Status needsField(FieldInfo): Status.YES for the requested field,
// Status.NO for every other.
func (v *getFieldsVisitor) NeedsField(name string) bool { return name == v.fieldName }

func (v *getFieldsVisitor) StringField(field string, value string) {
	if field != v.fieldName {
		return
	}
	ft := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	f, err := document.NewField(field, value, ft)
	if err != nil {
		if v.err == nil {
			v.err = err
		}
		return
	}
	v.fields = append(v.fields, f)
}

func (v *getFieldsVisitor) BinaryField(string, []byte)  {}
func (v *getFieldsVisitor) IntField(string, int)        {}
func (v *getFieldsVisitor) LongField(string, int64)     {}
func (v *getFieldsVisitor) FloatField(string, float32)  {}
func (v *getFieldsVisitor) DoubleField(string, float64) {}

func (b *BaseFragmentsBuilder) getFields(reader index.IndexReader, docID int, fieldName string) ([]*document.Field, error) {
	// according to javadoc, doc.getFields(fieldName) cannot be used with lazy
	// loaded field???
	storedFields, err := reader.StoredFields()
	if err != nil {
		return nil, err
	}
	visitor := &getFieldsVisitor{fieldName: fieldName}
	if err := storedFields.Document(docID, visitor); err != nil {
		return nil, err
	}
	if visitor.err != nil {
		return nil, visitor.err
	}
	return visitor.fields, nil
}

func (b *BaseFragmentsBuilder) makeFragment(buffer *strings.Builder, index *int, values []*document.Field, fragInfo *WeightedFragInfo, preTags, postTags []string, encoder highlight.Encoder) string {
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

func (b *BaseFragmentsBuilder) getFragmentSourceMSO(buffer *strings.Builder, index *int, values []*document.Field, startOffset, endOffset int, modifiedStartOffset *int) string {
	for buffer.Len() < endOffset && *index < len(values) {
		buffer.WriteString(values[*index].StringValue())
		*index++
		buffer.WriteRune(b.GetMultiValuedSeparator())
	}
	bufferLength := buffer.Len()
	// we added the multi value char to the last buffer, ignore it
	if values[*index-1].FieldType().Tokenized() {
		bufferLength--
	}
	eo := bufferLength
	if bufferLength >= endOffset {
		eo = b.BoundaryScanner.FindEndOffset(buffer.String(), endOffset)
	}
	*modifiedStartOffset = b.BoundaryScanner.FindStartOffset(buffer.String(), startOffset)
	return buffer.String()[*modifiedStartOffset:eo]
}

func (b *BaseFragmentsBuilder) getFragmentSource(buffer *strings.Builder, index *int, values []*document.Field, startOffset, endOffset int) string {
	for buffer.Len() < endOffset && *index < len(values) {
		buffer.WriteString(values[*index].StringValue())
		buffer.WriteRune(b.MultiValuedSeparator)
		*index++
	}
	eo := endOffset
	if buffer.Len() < endOffset {
		eo = buffer.Len()
	}
	return buffer.String()[startOffset:eo]
}

func (b *BaseFragmentsBuilder) discreteMultiValueHighlighting(fragInfos []*WeightedFragInfo, fields []*document.Field) []*WeightedFragInfo {
	fieldNameToFragInfos := make(map[string][]*WeightedFragInfo)
	fieldNameOrder := make([]string, 0, len(fields))
	for _, field := range fields {
		if _, ok := fieldNameToFragInfos[field.Name()]; !ok {
			fieldNameOrder = append(fieldNameOrder, field.Name())
		}
		fieldNameToFragInfos[field.Name()] = make([]*WeightedFragInfo, 0)
	}

nextFragInfo:
	for _, fragInfo := range fragInfos {
		fieldStart := 0
		fieldEnd := 0
		for _, field := range fields {
			if field.StringValue() == "" {
				fieldEnd++
				continue
			}
			fieldStart = fieldEnd
			// + 1 for going to next field with same name.
			fieldEnd += len(field.StringValue()) + 1

			if fragInfo.StartOffset >= fieldStart &&
				fragInfo.EndOffset >= fieldStart &&
				fragInfo.StartOffset <= fieldEnd &&
				fragInfo.EndOffset <= fieldEnd {
				fieldNameToFragInfos[field.Name()] = append(fieldNameToFragInfos[field.Name()], fragInfo)
				continue nextFragInfo
			}

			if len(fragInfo.SubInfos) == 0 {
				continue nextFragInfo
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
			// The boost of the new info will be the sum of the boosts of its
			// SubInfos
			boost := float32(0)
			keptSubInfos := fragInfo.SubInfos[:0]
			for si := range fragInfo.SubInfos {
				subInfo := &fragInfo.SubInfos[si]
				toffsList := make([]Toffs, 0)
				keptToffs := subInfo.TermsOffsets[:0]
				for _, toffs := range subInfo.TermsOffsets {
					if toffs.StartOffset >= fieldEnd {
						// We've gone past this value so its not worth iterating
						// any more.
						keptToffs = append(keptToffs, toffs)
						continue
					}
					startsAfterField := toffs.StartOffset >= fieldStart
					endsBeforeField := toffs.EndOffset < fieldEnd
					switch {
					case startsAfterField && endsBeforeField:
						// The Toff is entirely within this value.
						toffsList = append(toffsList, toffs)
					case startsAfterField:
						// The Toffs starts within this value but ends after
						// this value so we clamp the returned Toffs to this
						// value and leave the Toffs in the iterator for the
						// next value of this field.
						toffsList = append(toffsList, Toffs{toffs.StartOffset, fieldEnd - 1})
						keptToffs = append(keptToffs, toffs)
					case endsBeforeField:
						// The Toffs starts before this value but ends in this
						// value which means we're really continuing from where
						// we left off above. Since we use the remainder of the
						// offset we can remove it from the iterator.
						toffsList = append(toffsList, Toffs{fieldStart, toffs.EndOffset})
					default:
						// The Toffs spans the whole value so we clamp on both
						// sides. This is basically a combination of both arms
						// of the loop above.
						toffsList = append(toffsList, Toffs{fieldStart, fieldEnd - 1})
						keptToffs = append(keptToffs, toffs)
					}
				}
				subInfo.TermsOffsets = keptToffs
				if len(toffsList) > 0 {
					subInfos = append(subInfos, SubInfo{
						Text:         subInfo.Text,
						TermsOffsets: toffsList,
						Seqnum:       subInfo.Seqnum,
						Boost:        subInfo.Boost,
					})
					boost += subInfo.Boost
				}
				if len(subInfo.TermsOffsets) != 0 {
					keptSubInfos = append(keptSubInfos, *subInfo)
				}
			}
			fragInfo.SubInfos = keptSubInfos
			weightedFragInfo := &WeightedFragInfo{
				StartOffset: fragStart,
				EndOffset:   fragEnd,
				SubInfos:    subInfos,
				TotalBoost:  boost,
			}
			fieldNameToFragInfos[field.Name()] = append(fieldNameToFragInfos[field.Name()], weightedFragInfo)
		}
	}

	result := make([]*WeightedFragInfo, 0)
	for _, name := range fieldNameOrder {
		result = append(result, fieldNameToFragInfos[name]...)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].StartOffset-result[j].StartOffset < 0
	})

	return result
}

// GetMultiValuedSeparator returns the character inserted between the values of
// a multi-valued field.
func (b *BaseFragmentsBuilder) GetMultiValuedSeparator() rune { return b.MultiValuedSeparator }

// SetMultiValuedSeparator sets the character inserted between the values of a
// multi-valued field.
func (b *BaseFragmentsBuilder) SetMultiValuedSeparator(separator rune) {
	b.MultiValuedSeparator = separator
}

// IsDiscreteMultiValueHighlighting reports whether each value of a multi-valued
// field is highlighted on its own.
func (b *BaseFragmentsBuilder) IsDiscreteMultiValueHighlighting() bool {
	return b.DiscreteMultiValueHighlighting
}

// SetDiscreteMultiValueHighlighting sets whether each value of a multi-valued
// field is highlighted on its own.
func (b *BaseFragmentsBuilder) SetDiscreteMultiValueHighlighting(v bool) {
	b.DiscreteMultiValueHighlighting = v
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
