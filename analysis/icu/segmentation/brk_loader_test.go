// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadBRKDictionary_NilReader verifies the nil-reader branch returns the
// documented ErrNoDictionary sentinel.
func TestLoadBRKDictionary_NilReader(t *testing.T) {
	t.Parallel()

	dict, err := LoadBRKDictionary(nil)
	if !errors.Is(err, ErrNoDictionary) {
		t.Fatalf("nil reader: got err=%v, want ErrNoDictionary", err)
	}
	if dict != nil {
		t.Fatalf("nil reader: got dict=%v, want nil", dict)
	}
}

// TestParseBRKDictionary_Empty verifies the empty-slice branch.
func TestParseBRKDictionary_Empty(t *testing.T) {
	t.Parallel()

	dict, err := ParseBRKDictionary(nil)
	if !errors.Is(err, ErrNoDictionary) {
		t.Fatalf("nil slice: got err=%v, want ErrNoDictionary", err)
	}
	if dict != nil {
		t.Fatalf("nil slice: got dict=%v, want nil", dict)
	}

	dict, err = ParseBRKDictionary([]byte{})
	if !errors.Is(err, ErrNoDictionary) {
		t.Fatalf("empty slice: got err=%v, want ErrNoDictionary", err)
	}
	if dict != nil {
		t.Fatalf("empty slice: got dict=%v, want nil", dict)
	}
}

// TestParseBRKDictionary_Default validates that the real Lucene-shipped
// Default.brk blob parses cleanly and produces consistent metadata.
func TestParseBRKDictionary_Default(t *testing.T) {
	t.Parallel()

	data := mustReadFixture(t, "Default.brk")

	dict, err := ParseBRKDictionary(data)
	if err != nil {
		t.Fatalf("ParseBRKDictionary(Default.brk): %v", err)
	}
	if dict == nil {
		t.Fatal("ParseBRKDictionary(Default.brk): nil dict, no error")
	}

	// UDataInfo: header is 32 bytes (0x20), big-endian (ICU4J ships BE
	// blobs), ASCII charset, dataFormat "Brk ", formatVersion starts with
	// 0x06 (ICU RBBI format v6).
	if got, want := dict.HeaderSize, uint16(0x20); got != want {
		t.Errorf("HeaderSize: got %d, want %d", got, want)
	}
	if !dict.IsBigEndian {
		t.Errorf("IsBigEndian: got false, want true (Lucene ships BE blobs)")
	}
	if dict.CharsetFamily != 0 {
		t.Errorf("CharsetFamily: got %d, want 0 (ASCII)", dict.CharsetFamily)
	}
	if dict.FormatVersion[0] != 0x06 {
		t.Errorf("FormatVersion[0]: got 0x%02x, want 0x06", dict.FormatVersion[0])
	}
	if dict.Length == 0 {
		t.Error("Length: got 0, want non-zero")
	}
	if int(dict.Length) > len(data)-int(dict.HeaderSize) {
		t.Errorf("Length: %d exceeds blob payload (%d)", dict.Length, len(data)-int(dict.HeaderSize))
	}
	if dict.CatCount == 0 {
		t.Error("CatCount: got 0, want non-zero")
	}
	if !bytes.Equal(dict.Rules, data) {
		t.Error("Rules: not preserved verbatim")
	}
}

// TestParseBRKDictionary_Myanmar validates the MyanmarSyllable.brk fixture.
func TestParseBRKDictionary_Myanmar(t *testing.T) {
	t.Parallel()

	data := mustReadFixture(t, "MyanmarSyllable.brk")

	dict, err := ParseBRKDictionary(data)
	if err != nil {
		t.Fatalf("ParseBRKDictionary(MyanmarSyllable.brk): %v", err)
	}
	if dict.HeaderSize != 0x20 {
		t.Errorf("HeaderSize: got %d, want 32", dict.HeaderSize)
	}
	if dict.FormatVersion[0] != 0x06 {
		t.Errorf("FormatVersion[0]: got 0x%02x, want 0x06", dict.FormatVersion[0])
	}
}

// TestLoadBRKDictionary_FromReader exercises the LoadBRKDictionary streaming
// entry-point against a real Lucene fixture.
func TestLoadBRKDictionary_FromReader(t *testing.T) {
	t.Parallel()

	data := mustReadFixture(t, "Default.brk")

	dict, err := LoadBRKDictionary(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("LoadBRKDictionary: %v", err)
	}
	if dict == nil {
		t.Fatal("LoadBRKDictionary: nil dict, no error")
	}
	if !bytes.Equal(dict.Rules, data) {
		t.Error("Rules: not preserved verbatim from reader")
	}
}

// TestParseBRKDictionary_BadMagic ensures a corrupted ICU magic is rejected
// with an error wrapping ErrInvalidBRK.
func TestParseBRKDictionary_BadMagic(t *testing.T) {
	t.Parallel()

	data := mustReadFixture(t, "Default.brk")
	clone := append([]byte(nil), data...)
	clone[2] = 0xff // corrupt magic1
	clone[3] = 0xff

	_, err := ParseBRKDictionary(clone)
	if !errors.Is(err, ErrInvalidBRK) {
		t.Fatalf("bad magic: got err=%v, want ErrInvalidBRK", err)
	}
}

// TestParseBRKDictionary_BadDataFormat ensures the dataFormat tag is enforced.
func TestParseBRKDictionary_BadDataFormat(t *testing.T) {
	t.Parallel()

	data := mustReadFixture(t, "Default.brk")
	clone := append([]byte(nil), data...)
	clone[12] = 'X' // mangle "Brk " → "Xrk "

	_, err := ParseBRKDictionary(clone)
	if !errors.Is(err, ErrInvalidBRK) {
		t.Fatalf("bad dataFormat: got err=%v, want ErrInvalidBRK", err)
	}
}

// TestParseBRKDictionary_BadRBBIMagic ensures the inner RBBIDataHeader magic
// is enforced.
func TestParseBRKDictionary_BadRBBIMagic(t *testing.T) {
	t.Parallel()

	data := mustReadFixture(t, "Default.brk")
	clone := append([]byte(nil), data...)
	// Lucene .brk files are big-endian; corrupt the magic in that order.
	binary.BigEndian.PutUint32(clone[0x20:0x24], 0xdeadbeef)

	_, err := ParseBRKDictionary(clone)
	if !errors.Is(err, ErrInvalidBRK) {
		t.Fatalf("bad RBBI magic: got err=%v, want ErrInvalidBRK", err)
	}
}

// TestParseBRKDictionary_TooSmall covers the truncated-blob branch.
func TestParseBRKDictionary_TooSmall(t *testing.T) {
	t.Parallel()

	_, err := ParseBRKDictionary([]byte{0x00, 0x20, 0xda, 0x27})
	if !errors.Is(err, ErrInvalidBRK) {
		t.Fatalf("too small: got err=%v, want ErrInvalidBRK", err)
	}
}

// TestBRKDictionary_AsBreakIterator verifies the placeholder execution path:
// the iterator must conform to the RuleBasedBreakIterator contract, expose the
// originating dictionary, and report HasDictionaryExecution == false until a
// real RBBI engine is wired in.
func TestBRKDictionary_AsBreakIterator(t *testing.T) {
	t.Parallel()

	data := mustReadFixture(t, "Default.brk")
	dict, err := ParseBRKDictionary(data)
	if err != nil {
		t.Fatalf("ParseBRKDictionary: %v", err)
	}
	if dict.HasDictionaryExecution() {
		t.Fatal("HasDictionaryExecution: got true, want false (no RBBI engine yet)")
	}

	bi := dict.AsBreakIterator()
	if bi == nil {
		t.Fatal("AsBreakIterator: nil")
	}

	// Exercise the iterator with a tiny CJK input; the placeholder must
	// still advance and report rule-status values without panicking.
	text := []rune("中文 abc")
	bi.SetText(text, 0, len(text))
	seen := 0
	for {
		pos := bi.Next()
		if pos == Done {
			break
		}
		seen++
		if seen > 64 {
			t.Fatal("AsBreakIterator: iterator did not terminate")
		}
	}
	if seen == 0 {
		t.Error("AsBreakIterator: no breaks emitted")
	}

	// Clone must be independent.
	clone := bi.Clone()
	if clone == nil {
		t.Fatal("Clone: nil")
	}
	if clone == bi {
		t.Error("Clone: returned the same instance")
	}

	// Dictionary handle must be preserved.
	dbi, ok := bi.(*dictionaryBackedBreakIterator)
	if !ok {
		t.Fatalf("AsBreakIterator: unexpected type %T", bi)
	}
	if dbi.Dictionary() != dict {
		t.Error("Dictionary: returned a different dictionary")
	}
}

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}
