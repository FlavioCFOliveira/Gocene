package store

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// SegmentWriteState contains the state for writing a segment.
type SegmentWriteState struct {
	Directory    Directory
	Context      IOContext
	SegmentInfo  SegmentInfo
	FieldInfos   *index.FieldInfos
}

// SegmentInfo holds information about a segment.
type SegmentInfo struct {
	Name       string
	ID         []byte
	MaxDoc     int
	SegmentSuffix string
}

func (s *SegmentInfo) MaxDoc() int {
	return s.MaxDoc
}
