package vectorhighlight

// BoundaryScanner finds fragment boundaries: pluggable into BaseFragmentsBuilder.
// Mirrors org.apache.lucene.search.vectorhighlight.BoundaryScanner.
type BoundaryScanner interface {
	// FindStartOffset scans backward to find the start offset.
	// Mirrors org.apache.lucene.search.vectorhighlight.BoundaryScanner.findStartOffset.
	FindStartOffset(text string, start int) int

	// FindEndOffset scans forward to find the end offset.
	// Mirrors org.apache.lucene.search.vectorhighlight.BoundaryScanner.findEndOffset.
	FindEndOffset(text string, end int) int
}
