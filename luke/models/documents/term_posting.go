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

	sOffset, err := penum.StartOffset()
	if err != nil {
		return nil, err
	}
	eOffset, err := penum.EndOffset()
	if err != nil {
		return nil, err
	}
	if sOffset >= 0 && eOffset >= 0 {
		posting.startOffset = sOffset
		posting.endOffset = eOffset
	}

	payload, err := penum.GetPayload()
	if err != nil {
		return nil, err
	}
	if payload != nil {
		posting.payload = util.BytesRefDeepCopyOf(util.NewBytesRef(payload))
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
