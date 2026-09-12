package vectorhighlight

import (
	"fmt"
	"math"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TermInfo represents a single term with its position/offsets in the document and IDF weight.
type TermInfo struct {
	Text        string
	StartOffset int
	EndOffset   int
	Position    int
	Weight      float32
	Next        *TermInfo
}

func (ti *TermInfo) String() string {
	return fmt.Sprintf("%s(%d,%d,%d)", ti.Text, ti.StartOffset, ti.EndOffset, ti.Position)
}

// FieldTermStack is a stack that keeps query terms in the specified field of the
// document to be highlighted.
type FieldTermStack struct {
	fieldName string
	termList  []*TermInfo
}

func NewFieldTermStack(reader index.IndexReader, docID int, fieldName string, fieldQuery *FieldQuery) error {
	fts := &FieldTermStack{
		fieldName: fieldName,
		termList:  make([]*TermInfo, 0),
	}

	termSet := fieldQuery.GetTermSet(fieldName)
	if termSet == nil {
		return nil
	}

	vectors := reader.TermVectors(docID)
	if vectors == nil {
		return nil
	}

	vector := vectors.Terms(fieldName)
	if vector == nil || !vector.HasPositions() {
		return nil
	}

	termsEnum := vector.Iterator()
	var dpEnum index.PostingsEnum
	var text string

	numDocs := reader.MaxDoc()

	for {
		text = termsEnum.Next()
		if text == "" {
			break
		}

		if !termSet.Contains(text) {
			continue
		}

		dpEnum = termsEnum.Postings(dpEnum, index.PostingsEnumPos)
		dpEnum.NextDoc()

		// IDF weight calculation
		docFreq := reader.DocFreq(fieldName, text)
		weight := float32(math.Log(float64(numDocs)/float64(docFreq+1)) + 1.0)

		freq := dpEnum.Freq()
		for i := 0; i < freq; i++ {
			pos := dpEnum.NextPosition()
			if dpEnum.StartOffset() < 0 {
				return nil // no offsets, null snippet
			}
			fts.termList = append(fts.termList, &TermInfo{
				Text:        text,
				StartOffset: dpEnum.StartOffset(),
				EndOffset:   dpEnum.EndOffset(),
				Position:    pos,
				Weight:      weight,
			})
		}
	}

	// sort by position
	sort.Slice(fts.termList, func(i, j int) bool {
		return fts.termList[i].Position < fts.termList[j].Position
	})

	// now look for dups at the same position, linking them together
	currentPos := -1
	var previous *TermInfo
	var first *TermInfo

	var processed []*TermInfo
	for _, current := range fts.termList {
		if current.Position == currentPos {
			if previous != nil {
				previous.Next = current
				previous = current
			}
		} else {
			if previous != nil {
				previous.Next = first
			}
			first = current
			previous = current
			currentPos = current.Position
		}
		processed = append(processed, current)
	}
	if previous != nil {
		previous.Next = first
	}

	// The original Java code uses a LinkedList and removes duplicates.
	// We need to filter the termList to keep only the 'first' of each position
	// because the others are linked via .Next

	var filtered []*TermInfo
	for i := 0; i < len(fts.termList); i++ {
		if i == 0 || fts.termList[i].Position != fts.termList[i-1].Position {
			filtered = append(filtered, fts.termList[i])
		}
	}
	fts.termList = filtered

	return nil
}

func (fts *FieldTermStack) GetFieldName() string {
	return fts.fieldName
}

func (fts *FieldTermStack) Pop() *TermInfo {
	if fts.IsEmpty() {
		return nil
	}
	ti := fts.termList[0]
	fts.termList = fts.termList[1:]
	return ti
}

func (fts *FieldTermStack) Push(ti *TermInfo) {
	fts.termList = append([]*TermInfo{ti}, fts.termList...)
}

func (fts *FieldTermStack) IsEmpty() bool {
	return len(fts.termList) == 0
}
