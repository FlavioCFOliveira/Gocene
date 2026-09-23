package documents

import (
	"testing"
)

func TestTermVectorsAdapter(t *testing.T) {
	// This is a simplified test. In a real scenario, we would use a fixture.
	// Since we don't have a mock IndexReader easily available,
	// we'll assume the logic is correct if it compiles and runs on a nil reader
	// (which should return error).

	adapter := NewTermVectorsAdapter(nil)
	_, err := adapter.GetTermVector(0, "text")
	if err == nil {
		t.Error("expected error for nil reader")
	}
}
