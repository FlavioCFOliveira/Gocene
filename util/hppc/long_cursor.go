package hppc

import "fmt"

// LongCursor holds an int index and a long value.
// It is a port of org.apache.lucene.internal.hppc.LongCursor.
type LongCursor struct {
	Index int32
	Value int64
}

// String returns a string representation of the LongCursor.
func (c LongCursor) String() string {
	return fmt.Sprintf("[cursor, index: %d, value: %d]", c.Index, c.Value)
}
