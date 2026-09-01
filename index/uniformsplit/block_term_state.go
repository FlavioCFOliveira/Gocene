package uniformsplit

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// BlockTermState holds all state required for PostingsReaderBase to produce a
// PostingsEnum without re-seeking the terms dict. Mirrors
// org.apache.lucene.codecs.BlockTermState (Lucene 10.5.0).
type BlockTermState struct {
	index.OrdTermState
	// how many docs have this term
	docFreq int
	// total number of occurrences of this term
	totalTermFreq int64
	// the term's ord in the current block
	termBlockOrd int
	// fp into the terms dict primary file (_X.tim) that holds this term
	blockFilePointer int64
}

// NewBlockTermState constructs a BlockTermState.
func NewBlockTermState() *BlockTermState {
	return &BlockTermState{}
}

// CopyFrom resets this state from other.
func (s *BlockTermState) CopyFrom(other index.TermState) error {
	o, ok := other.(*BlockTermState)
	if !ok {
		return fmt.Errorf("BlockTermState.CopyFrom: incompatible source type %T", other)
	}
	if err := s.OrdTermState.CopyFrom(other); err != nil {
		return err
	}
	s.docFreq = o.docFreq
	s.totalTermFreq = o.totalTermFreq
	s.termBlockOrd = o.termBlockOrd
	s.blockFilePointer = o.blockFilePointer
	return nil
}

func (s *BlockTermState) String() string {
	return fmt.Sprintf("docFreq=%d totalTermFreq=%d termBlockOrd=%d blockFP=%d",
		s.docFreq, s.totalTermFreq, s.termBlockOrd, s.blockFilePointer)
}
