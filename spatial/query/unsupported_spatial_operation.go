// Package query implements org.apache.lucene.spatial.query.
package query

import "fmt"

// UnsupportedSpatialOperation is the error returned when a spatial strategy
// is asked to perform an operation it cannot honour. Mirrors
// org.apache.lucene.spatial.query.UnsupportedSpatialOperation.
type UnsupportedSpatialOperation struct {
	Op string
}

func (e *UnsupportedSpatialOperation) Error() string {
	return fmt.Sprintf("spatial: unsupported operation %q", e.Op)
}

// NewUnsupportedSpatialOperation builds the error.
func NewUnsupportedSpatialOperation(op string) *UnsupportedSpatialOperation {
	return &UnsupportedSpatialOperation{Op: op}
}
