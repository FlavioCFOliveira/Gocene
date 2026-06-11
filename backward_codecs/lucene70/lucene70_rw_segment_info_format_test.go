package lucene70

import "testing"

func TestLucene70SegmentInfoFormat_New(t *testing.T) {
	f := NewLucene70SegmentInfoFormat()
	if f == nil {
		t.Fatal("NewLucene70SegmentInfoFormat returned nil")
	}
}
