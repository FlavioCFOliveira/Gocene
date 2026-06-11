package memory

import "testing"

func TestMemoryIndex_New(t *testing.T) {
	mi := NewMemoryIndex()
	if mi == nil {
		t.Fatal("NewMemoryIndex returned nil")
	}
}

func TestMemoryIndex_NewWithMaxReusedBytes(t *testing.T) {
	mi := NewMemoryIndexWithMaxReusedBytes(1024)
	if mi == nil {
		t.Fatal("NewMemoryIndexWithMaxReusedBytes returned nil")
	}
}

func TestMemoryIndex_AddField(t *testing.T) {
	mi := NewMemoryIndex()
	if err := mi.AddField("test", "value"); err != nil {
		t.Fatalf("AddField: %v", err)
	}
}

func TestMemoryIndex_GetFields(t *testing.T) {
	mi := NewMemoryIndex()
	mi.AddField("f1", "v1")
	mi.AddField("f2", "v2")
	fields := mi.GetFields()
	if len(fields) != 2 {
		t.Fatalf("GetFields len=%d, want 2", len(fields))
	}
}

func TestMemoryIndex_GetTermFrequency(t *testing.T) {
	mi := NewMemoryIndex()
	mi.AddField("f", "hello world hello")
	freq := mi.GetTermFrequency("f", "hello")
	if freq != 2 {
		t.Fatalf("GetTermFrequency=%d, want 2", freq)
	}
}

func TestMemoryIndex_Reset(t *testing.T) {
	mi := NewMemoryIndex()
	mi.AddField("f", "v")
	mi.Reset()
	fields := mi.GetFields()
	if len(fields) != 0 {
		t.Fatalf("GetFields after reset len=%d, want 0", len(fields))
	}
}
