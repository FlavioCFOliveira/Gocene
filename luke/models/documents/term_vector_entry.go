package documents

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/index"
	coreutil "github.com/FlavioCFOliveira/Gocene/util"
)

// TermVectorEntry is a holder for term vector entry representing the term and their number of occurrences, and optionally, positions in the document field.
type TermVectorEntry struct {
	termText  string
	freq      int64
	positions []TermVectorPosition
}

func NewTermVectorEntry(te index.TermsEnum) (*TermVectorEntry, error) {
	termText := te.Term().Text()

	var tvPositions []TermVectorPosition
	pe, err := te.Postings(index.PostingsFlagOffsets)
	if err != nil {
		return nil, err
	}

	doc, err := pe.NextDoc()
	if err != nil {
		return nil, err
	}
	if doc != coreutil.NO_MORE_DOCS {
		freq, err := pe.Freq()
		if err != nil {
			return nil, err
		}
		for i := 0; i < freq; i++ {
			pos, err := pe.NextPosition()
			if err != nil {
				return nil, err
			}
			if pos < 0 {
				continue
			}
			tvPos, err := NewTermVectorPosition(pos, pe)
			if err != nil {
				return nil, err
			}
			tvPositions = append(tvPositions, *tvPos)
		}
	}

	totalTermFreq, err := te.TotalTermFreq()
	if err != nil {
		return nil, err
	}
	return &TermVectorEntry{
		termText:  termText,
		freq:      totalTermFreq,
		positions: tvPositions,
	}, nil
}

func (tve *TermVectorEntry) TermText() string {
	return tve.termText
}

func (tve *TermVectorEntry) Freq() int64 {
	return tve.freq
}

func (tve *TermVectorEntry) Positions() []TermVectorPosition {
	return tve.positions
}

func (tve *TermVectorEntry) String() string {
	var posStr string
	for i, p := range tve.positions {
		if i > 0 {
			posStr += ","
		}
		posStr += p.String()
	}
	return fmt.Sprintf("TermVectorEntry{termText='%s', freq=%d, positions=[%s]}",
		tve.termText, tve.freq, posStr)
}

// TermVectorPosition is a holder for position information for a term vector entry.
type TermVectorPosition struct {
	position    int
	startOffset int
	endOffset   int
}

func NewTermVectorPosition(pos int, pe index.PostingsEnum) (*TermVectorPosition, error) {
	sOffset, err := pe.StartOffset()
	if err != nil {
		return nil, err
	}
	eOffset, err := pe.EndOffset()
	if err != nil {
		return nil, err
	}
	if sOffset >= 0 && eOffset >= 0 {
		return &TermVectorPosition{
			position:    pos,
			startOffset: sOffset,
			endOffset:   eOffset,
		}, nil
	}
	return &TermVectorPosition{
		position:    pos,
		startOffset: -1,
		endOffset:   -1,
	}, nil
}

func (tvp *TermVectorPosition) Position() int {
	return tvp.position
}

func (tvp *TermVectorPosition) StartOffset() (int, bool) {
	if tvp.startOffset >= 0 {
		return tvp.startOffset, true
	}
	return -1, false
}

func (tvp *TermVectorPosition) EndOffset() (int, bool) {
	if tvp.endOffset >= 0 {
		return tvp.endOffset, true
	}
	return -1, false
}

func (tvp *TermVectorPosition) String() string {
	return fmt.Sprintf("TermVectorPosition{position=%d, startOffset=%d, endOffset=%d}",
		tvp.position, tvp.startOffset, tvp.endOffset)
}
