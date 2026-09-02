package uhighlight

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// LimitedStoredFieldVisitor efficiently retrieves stored field values.
type LimitedStoredFieldVisitor struct {
	fields       []string
	valueSeparator rune
	maxLength    int
	values       []string
	currentField int
}

func NewLimitedStoredFieldVisitor(fields []string, valueSeparator rune, maxLength int) *LimitedStoredFieldVisitor {
	return &LimitedStoredFieldVisitor{
		fields:       fields,
		valueSeparator: valueSeparator,
		maxLength:    maxLength,
	}
}

func (v *LimitedStoredFieldVisitor) Init() {
	v.values = make([]string, len(v.fields))
	v.currentField = -1
}

func (v *LimitedStoredFieldVisitor) StringField(fieldInfo index.FieldInfo, value string) {
	// Binary search for field
	idx := -1
	for i, f := range v.fields {
		if f == fieldInfo.Name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	v.currentField = idx

	curVal := v.values[v.currentField]
	if curVal == "" {
		if len(value) > v.maxLength {
			v.values[v.currentField] = value[:v.maxLength]
		} else {
			v.values[v.currentField] = value
		}
		return
	}

	lengthBudget := v.maxLength - len(curVal)
	if lengthBudget <= 0 {
		return
	}

	var sb strings.Builder
	sb.WriteString(curVal)
	sb.WriteRune(v.valueSeparator)
	if len(value) > lengthBudget-1 {
		sb.WriteString(value[:lengthBudget-1])
	} else {
		sb.WriteString(value)
	}
	v.values[v.currentField] = sb.String()
}

func (v *LimitedStoredFieldVisitor) NeedsField(fieldInfo index.FieldInfo) bool {
	idx := -1
	for i, f := range v.fields {
		if f == fieldInfo.Name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	v.currentField = idx
	if len(v.values[v.currentField]) >= v.maxLength {
		return false
	}
	return true
}

func (v *LimitedStoredFieldVisitor) GetValuesByField() []string {
	return v.values
}

// TermVectorReusingLeafReader caches the last call to termVectors.Get(docID).
type TermVectorReusingLeafReader struct {
	reader     index.LeafReader
	lastDocID  int
	lastFields index.Fields
}

func NewTermVectorReusingLeafReader(reader index.LeafReader) *TermVectorReusingLeafReader {
	return &TermVectorReusingLeafReader{
		reader:    reader,
		lastDocID: -1,
	}
}

func (r *TermVectorReusingLeafReader) TermVectors() index.TermVectors {
	return &tvWrapper{
		r: r,
	}
}

type tvWrapper struct {
	r *TermVectorReusingLeafReader
}

func (w *tvWrapper) Get(docID int) (index.Fields, error) {
	if docID != w.r.lastDocID {
		w.r.lastDocID = docID
		fields, err := w.r.reader.TermVectors().Get(docID)
		if err != nil {
			return nil, err
		}
		w.r.lastFields = fields
	}
	return w.r.lastFields, nil
}
