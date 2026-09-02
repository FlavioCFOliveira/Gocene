package util

// DocIndexIterator is a DocIdSetIterator that also provides an Index() method
// tracking a distinct ordinal for a vector associated with each doc.
type DocIndexIterator interface {
	DocIdSetIterator
	// Index returns the value index (aka "ordinal" or "ord") corresponding to the current doc.
	Index() int
}
