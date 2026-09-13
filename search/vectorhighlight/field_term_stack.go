package vectorhighlight

import (
	"fmt"
	"math"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// TermInfo is a single term with its position/offsets in the document and IDF
// weight. It is comparable but considers only position.
//
// Mirrors the nested class FieldTermStack.TermInfo of Apache Lucene 10.5.0.
type TermInfo struct {
	Text        string
	StartOffset int
	EndOffset   int
	Position    int

	// Weight is the IDF-weight of this term.
	Weight float32

	// Next points to other TermInfo's at the same position. This is a circular
	// list, so with no syns it just points to itself.
	Next *TermInfo
}

// NewTermInfo creates a TermInfo whose circular next pointer points at itself.
func NewTermInfo(text string, startOffset, endOffset, position int, weight float32) *TermInfo {
	ti := &TermInfo{
		Text:        text,
		StartOffset: startOffset,
		EndOffset:   endOffset,
		Position:    position,
		Weight:      weight,
	}
	ti.Next = ti
	return ti
}

// SetNext sets the next TermInfo at this same position.
func (ti *TermInfo) SetNext(next *TermInfo) { ti.Next = next }

// GetNext returns the next TermInfo at this same position. This is a circular
// list!
func (ti *TermInfo) GetNext() *TermInfo { return ti.Next }

// GetText returns the term text.
func (ti *TermInfo) GetText() string { return ti.Text }

// GetStartOffset returns the start offset of this term occurrence.
func (ti *TermInfo) GetStartOffset() int { return ti.StartOffset }

// GetEndOffset returns the end offset of this term occurrence.
func (ti *TermInfo) GetEndOffset() int { return ti.EndOffset }

// GetPosition returns the position of this term occurrence.
func (ti *TermInfo) GetPosition() int { return ti.Position }

// GetWeight returns the IDF weight of this term.
func (ti *TermInfo) GetWeight() float32 { return ti.Weight }

func (ti *TermInfo) String() string {
	return fmt.Sprintf("%s(%d,%d,%d)", ti.Text, ti.StartOffset, ti.EndOffset, ti.Position)
}

// CompareTo orders TermInfo by position alone.
func (ti *TermInfo) CompareTo(o *TermInfo) int { return ti.Position - o.Position }

// HashCode mirrors TermInfo.hashCode(), which hashes the position alone.
func (ti *TermInfo) HashCode() int {
	const prime = 31
	result := 1
	result = prime*result + ti.Position
	return result
}

// Equals mirrors TermInfo.equals(Object), which compares the position alone.
func (ti *TermInfo) Equals(other *TermInfo) bool {
	if ti == other {
		return true
	}
	if other == nil {
		return false
	}
	return ti.Position == other.Position
}

// FieldTermStack is a stack that keeps query terms in the specified field of
// the document to be highlighted.
//
// Mirrors org.apache.lucene.search.vectorhighlight.FieldTermStack of Apache
// Lucene 10.5.0.
type FieldTermStack struct {
	fieldName string
	termList  []*TermInfo
}

// NewFieldTermStack builds the stack of query terms found in field fieldName of
// document docID.
//
// Mirrors FieldTermStack(IndexReader, int, String, FieldQuery) of Apache Lucene
// 10.5.0.
func NewFieldTermStack(reader index.IndexReader, docID int, fieldName string, fieldQuery *FieldQuery) (*FieldTermStack, error) {
	fts := &FieldTermStack{fieldName: fieldName}

	termSet := fieldQuery.GetTermSet(fieldName)
	// just return to make null snippet if un-matched fieldName specified when
	// fieldMatch == true
	if termSet == nil {
		return fts, nil
	}

	termVectors, err := reader.TermVectors()
	if err != nil {
		return nil, err
	}
	vectors, err := termVectors.Get(docID)
	if err != nil {
		return nil, err
	}
	if vectors == nil {
		// null snippet
		return fts, nil
	}

	vector, err := vectors.Terms(fieldName)
	if err != nil {
		return nil, err
	}
	if vector == nil || !vector.HasPositions() {
		// null snippet
		return fts, nil
	}

	termsEnum, err := vector.GetIterator()
	if err != nil {
		return nil, err
	}

	numDocs := reader.MaxDoc()

	for {
		text, err := termsEnum.Next()
		if err != nil {
			return nil, err
		}
		if text == nil {
			break
		}
		term := text.Text()
		if _, ok := termSet[term]; !ok {
			continue
		}
		dpEnum, err := termsEnum.Postings(spi.PostingsFlagPositions)
		if err != nil {
			return nil, err
		}
		if _, err := dpEnum.NextDoc(); err != nil {
			return nil, err
		}

		// For weight look here:
		// http://lucene.apache.org/core/3_6_0/api/core/org/apache/lucene/search/DefaultSimilarity.html
		docFreq, err := index.DocFreq(reader, index.NewTerm(fieldName, term))
		if err != nil {
			return nil, err
		}
		weight := float32(math.Log(float64(numDocs)/float64(docFreq+1)) + 1.0)

		freq, err := dpEnum.Freq()
		if err != nil {
			return nil, err
		}

		for i := 0; i < freq; i++ {
			pos, err := dpEnum.NextPosition()
			if err != nil {
				return nil, err
			}
			startOffset, err := dpEnum.StartOffset()
			if err != nil {
				return nil, err
			}
			if startOffset < 0 {
				return fts, nil // no offsets, null snippet
			}
			endOffset, err := dpEnum.EndOffset()
			if err != nil {
				return nil, err
			}
			fts.termList = append(fts.termList, NewTermInfo(term, startOffset, endOffset, pos, weight))
		}
	}

	// sort by position
	sort.SliceStable(fts.termList, func(i, j int) bool {
		return fts.termList[i].CompareTo(fts.termList[j]) < 0
	})

	// now look for dups at the same position, linking them together
	currentPos := -1
	var previous *TermInfo
	var first *TermInfo
	kept := fts.termList[:0]
	for _, current := range fts.termList {
		if current.Position == currentPos {
			previous.SetNext(current)
			previous = current
			// iterator.remove(): the duplicate stays reachable only through the
			// circular next chain.
			continue
		}
		if previous != nil {
			previous.SetNext(first)
		}
		previous = current
		first = current
		currentPos = current.Position
		kept = append(kept, current)
	}
	if previous != nil {
		previous.SetNext(first)
	}
	fts.termList = kept

	return fts, nil
}

// GetFieldName returns the field name.
func (fts *FieldTermStack) GetFieldName() string {
	return fts.fieldName
}

// Pop returns the top TermInfo object of the stack, or nil when it is empty.
func (fts *FieldTermStack) Pop() *TermInfo {
	if len(fts.termList) == 0 {
		return nil
	}
	ti := fts.termList[0]
	fts.termList = fts.termList[1:]
	return ti
}

// Push puts termInfo on the top of the stack.
func (fts *FieldTermStack) Push(ti *TermInfo) {
	fts.termList = append([]*TermInfo{ti}, fts.termList...)
}

// IsEmpty reports whether the stack is empty.
func (fts *FieldTermStack) IsEmpty() bool {
	return len(fts.termList) == 0
}
