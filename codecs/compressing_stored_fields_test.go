package codecs

import "testing"

func TestCompressingStoredFieldsFormat_Defaults(t *testing.T) {
	fmt := NewCompressingStoredFieldsFormat(CompressionModeLZ4Fast, 16*1024, 128)
	if fmt == nil {
		t.Fatal("NewCompressingStoredFieldsFormat returned nil")
	}
	if fmt.Name() == "" {
		t.Error("format name should not be empty")
	}
}

func TestCompressingTermVectorsFormat_Defaults(t *testing.T) {
	fmt := DefaultCompressingTermVectorsFormat()
	if fmt == nil {
		t.Fatal("DefaultCompressingTermVectorsFormat returned nil")
	}
	if fmt.Name() == "" {
		t.Error("format name should not be empty")
	}
}

func TestCompressionMode_Values(t *testing.T) {
	if CompressionModeLZ4Fast.String() == "" {
		t.Error("CompressionModeLZ4Fast should have a name")
	}
	if CompressionModeLZ4High.String() == "" {
		t.Error("CompressionModeLZ4High should have a name")
	}
	if CompressionModeDeflate.String() == "" {
		t.Error("CompressionModeDeflate should have a name")
	}
}
