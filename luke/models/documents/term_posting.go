package documents

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermPosting is a holder for a term's position information, and optionally, offsets and payloads.
type TermPosting struct {
	position    int
	startOffset int
	endOffset   int
	payload     *util.BytesRef
}

func NewTermPosting(position int, penum index.PostingsEnum) (*TermPosting, error) {
	posting := &TermPosting{
		position:    position,
		startOffset: -1,
		endOffset:   -1,
	}

	sOffset := penum.StartOffset()
	eOffset := penum.EndOffset()
	if sOffset >= 0 && eOffset >= 0 {
		posting.startOffset = sOffset
		posting.endOffset = eOffset
	}

	if payload := penum.GetPayload(); payload != nil {
		posting.payload = util.BytesRefDeepCopyOf(payload)
	}

	return posting, nil
}

func (tp *TermPosting) Position() int {
	return tp.position
}

func (tp *TermPosting) StartOffset() int {
	return tp.startOffset
}

func (tp *TermPosting) EndOffset() int {
	return tp.endOffset
}

func (tp *TermPosting) Payload() *util.BytesRef {
	return tp.payload
}

func (tp *TermPosting) String() string {
	return fmt.Sprintf("TermPosting{position=%d, startOffset=%d, endOffset=%d, payload=%v}",
		tp.position, tp.startOffset, tp.endOffset, tp.payload)
}
