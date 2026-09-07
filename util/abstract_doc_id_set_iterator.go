package util

import (
	"fmt"
	"unsafe"
)

// AbstractDocIdSetIterator is an abstract implementation of a DocIdSetIterator that tracks the current doc ID.
// Implementing DocIdSetIterator by embedding this struct is recommended to reduce polymorphism of call sites to DocID().
type AbstractDocIdSetIterator struct {
	// Doc is the current doc ID, initialized at -1.
	Doc int
}

// DocID returns the current doc ID.
func (it *AbstractDocIdSetIterator) DocID() int {
	return it.Doc
}

// Equals returns true if it and other are equal.
// Following the default Java Object implementation as it is not overridden in Lucene's AbstractDocIdSetIterator.
func (it *AbstractDocIdSetIterator) Equals(other interface{}) bool {
	return it == other
}

// HashCode returns a hash code for the iterator.
// Following the default Java Object implementation.
func (it *AbstractDocIdSetIterator) HashCode() int {
	return int(uintptr(unsafe.Pointer(it)))
}

// String returns a string representation of the iterator.
func (it *AbstractDocIdSetIterator) String() string {
	return fmt.Sprintf("AbstractDocIdSetIterator{doc=%d}", it.Doc)
}
