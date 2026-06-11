package search

import (
	"testing"
)

func TestIndexOrDocValuesQuery_New(t *testing.T) {
	// IndexOrDocValuesQuery wraps an index query with a random-access (DV) fallback
	// For testing, use a TermQuery as both — real usage would use PointQuery + DV query
	indexQ := NewTermQuery(nil)
	randomAccessQ := NewTermQuery(nil)
	q := NewIndexOrDocValuesQuery(indexQ, randomAccessQ)
	if q == nil {
		t.Fatal("NewIndexOrDocValuesQuery returned nil")
	}
}

func TestIndexOrDocValuesQuery_GetIndexQuery(t *testing.T) {
	indexQ := NewTermQuery(nil)
	randomAccessQ := NewTermQuery(nil)
	q := NewIndexOrDocValuesQuery(indexQ, randomAccessQ)
	if q.GetIndexQuery() != indexQ {
		t.Error("GetIndexQuery should return the index query")
	}
}

func TestIndexOrDocValuesQuery_GetRandomAccessQuery(t *testing.T) {
	indexQ := NewTermQuery(nil)
	randomAccessQ := NewTermQuery(nil)
	q := NewIndexOrDocValuesQuery(indexQ, randomAccessQ)
	if q.GetRandomAccessQuery() != randomAccessQ {
		t.Error("GetRandomAccessQuery should return the random access query")
	}
}

func TestIndexOrDocValuesQuery_String(t *testing.T) {
	q := NewIndexOrDocValuesQuery(NewTermQuery(nil), NewTermQuery(nil))
	s := q.String()
	if s == "" {
		t.Error("String() returned empty")
	}
}
