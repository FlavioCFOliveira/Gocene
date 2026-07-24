// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0 test
// org.apache.lucene.index.TestIndexWriterUnicode
// (core/src/test/org/apache/lucene/index/TestIndexWriterUnicode.java).
//
// GOC-4184. The Java original has five @Test methods. The two that exercise
// only UTF-8/UTF-16 transcoding (testRandomUnicodeStrings, testAllUnicodeChars)
// are ported in full. The three that require an IndexWriter / DirectoryReader
// round-trip (testEmbeddedFFFF, testInvalidUTF16, testTermUTF16SortOrder) are
// skipped pending that indexing infrastructure, per Sprint 55 option c.

package index_test

import (
	"math/rand"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	indexTestutil "github.com/FlavioCFOliveira/Gocene/index/testutil"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// fillUnicode populates buffer with random UTF-16 code units and records the
// expected post-transcode code units in expected. It mirrors the Java helper
// of the same name: lone or out-of-order surrogates produce U+FFFD in expected
// while buffer retains the raw (illegal) unit. Returns true when at least one
// illegal sequence was injected.
func fillUnicode(r *rand.Rand, buffer, expected []uint16, offset, count int) bool {
	end := offset + count
	hasIllegal := false

	// Don't start in the middle of a valid surrogate pair.
	if offset > 0 && buffer[offset] >= 0xdc00 && buffer[offset] < 0xe000 {
		offset--
	}

	nextInt := func(lim int) int { return r.Intn(lim) }
	nextRange := func(start, e int) int { return start + nextInt(e-start) }

	for i := offset; i < end; i++ {
		t := nextInt(6)
		switch {
		case t == 0 && i < end-1:
			// Make a surrogate pair.
			expected[i] = uint16(nextRange(0xd800, 0xdc00))
			buffer[i] = expected[i]
			i++
			expected[i] = uint16(nextRange(0xdc00, 0xe000))
			buffer[i] = expected[i]
		case t <= 1:
			expected[i] = uint16(nextInt(0x80))
			buffer[i] = expected[i]
		case t == 2:
			expected[i] = uint16(nextRange(0x80, 0x800))
			buffer[i] = expected[i]
		case t == 3:
			expected[i] = uint16(nextRange(0x800, 0xd800))
			buffer[i] = expected[i]
		case t == 4:
			expected[i] = uint16(nextRange(0xe000, 0xffff))
			buffer[i] = expected[i]
		case t == 5 && i < end-1:
			// Illegal unpaired surrogate.
			if nextInt(10) == 7 {
				if r.Intn(2) == 1 {
					buffer[i] = uint16(nextRange(0xd800, 0xdc00))
				} else {
					buffer[i] = uint16(nextRange(0xdc00, 0xe000))
				}
				expected[i] = 0xfffd
				i++
				expected[i] = uint16(nextRange(0x800, 0xd800))
				buffer[i] = expected[i]
				hasIllegal = true
			} else {
				expected[i] = uint16(nextRange(0x800, 0xd800))
				buffer[i] = expected[i]
			}
		default:
			expected[i] = ' '
			buffer[i] = ' '
		}
	}

	return hasIllegal
}

// utf16Units builds a string from raw UTF-16 code units. Unlike a Go string
// literal it accepts lone surrogates, which the Java test relies on. Code
// units outside the surrogate range are kept as-is; lone or out-of-order
// surrogates survive into the result so callers can feed illegal UTF-16 in.
func utf16Units(units ...uint16) string {
	r := make([]rune, len(units))
	for i, u := range units {
		r[i] = rune(u)
	}
	return string(r)
}

// Surrogate and replacement code units used to assemble the utf8Data fixture.
const (
	lowSur  = 0xdc17 // unpaired low surrogate
	highSur = 0xd917 // unpaired high surrogate
	fffd    = 0xfffd // U+FFFD replacement character
)

// utf8Data pairs each invalid-UTF16 input with its expected U+FFFD-substituted
// form, mirroring the Java fixture. Each entry is assembled from raw code
// units because Go string literals reject lone surrogates. It backs the
// skipped testInvalidUTF16 port.
var utf8Data = []string{
	// unpaired low surrogate
	utf16Units('a', 'b', lowSur, 'c', 'd'), utf16Units('a', 'b', fffd, 'c', 'd'),
	utf16Units(lowSur, 'a', 'b', 'c', 'd'), utf16Units(fffd, 'a', 'b', 'c', 'd'),
	utf16Units(lowSur), utf16Units(fffd),
	utf16Units('a', 'b', lowSur, lowSur, 'c', 'd'), utf16Units('a', 'b', fffd, fffd, 'c', 'd'),
	utf16Units(lowSur, lowSur, 'a', 'b', 'c', 'd'), utf16Units(fffd, fffd, 'a', 'b', 'c', 'd'),
	utf16Units(lowSur, lowSur), utf16Units(fffd, fffd),

	// unpaired high surrogate
	utf16Units('a', 'b', highSur, 'c', 'd'), utf16Units('a', 'b', fffd, 'c', 'd'),
	utf16Units(highSur, 'a', 'b', 'c', 'd'), utf16Units(fffd, 'a', 'b', 'c', 'd'),
	utf16Units(highSur), utf16Units(fffd),
	utf16Units('a', 'b', highSur, highSur, 'c', 'd'), utf16Units('a', 'b', fffd, fffd, 'c', 'd'),
	utf16Units(highSur, highSur, 'a', 'b', 'c', 'd'), utf16Units(fffd, fffd, 'a', 'b', 'c', 'd'),
	utf16Units(highSur, highSur), utf16Units(fffd, fffd),

	// backwards surrogates
	utf16Units('a', 'b', lowSur, highSur, 'c', 'd'), utf16Units('a', 'b', fffd, fffd, 'c', 'd'),
	utf16Units(lowSur, highSur, 'a', 'b', 'c', 'd'), utf16Units(fffd, fffd, 'a', 'b', 'c', 'd'),
	utf16Units(lowSur, highSur), utf16Units(fffd, fffd),
	utf16Units('a', 'b', lowSur, highSur, lowSur, highSur, 'c', 'd'),
	utf16Units('a', 'b', fffd, highSur, lowSur, fffd, 'c', 'd'),
	utf16Units(lowSur, highSur, lowSur, highSur, 'a', 'b', 'c', 'd'),
	utf16Units(fffd, highSur, lowSur, fffd, 'a', 'b', 'c', 'd'),
	utf16Units(lowSur, highSur, lowSur, highSur), utf16Units(fffd, highSur, lowSur, fffd),
}

// TestIndexWriterUnicode_RandomUnicodeStrings ports testRandomUnicodeStrings
// (LUCENE-510): random UTF-16 buffers transcoded to UTF-8 and back must yield
// the expected code units, and for legal input the UTF-8 must match Go's
// native encoding byte for byte.
func TestIndexWriterUnicode_RandomUnicodeStrings(t *testing.T) {
	r := rand.New(rand.NewSource(0x510))
	buffer := make([]uint16, 20)
	expected := make([]uint16, 20)
	utf16Out := util.NewCharsRefBuilder()

	const num = 10000
	for iter := 0; iter < num; iter++ {
		hasIllegal := fillUnicode(r, buffer, expected, 0, 20)

		utf8Buf := make([]byte, util.MaxUTF8Length(20))
		n := util.UTF16ToUTF8Chars(buffer, 0, 20, utf8Buf)
		utf8Bytes := utf8Buf[:n]

		if !hasIllegal {
			// For legal UTF-16 the transcode must equal Go's native encoding.
			b := []byte(string(utf16.Decode(buffer[:20])))
			if len(b) != len(utf8Bytes) {
				t.Fatalf("iter %d: utf8 length = %d, want %d", iter, len(utf8Bytes), len(b))
			}
			for i := range b {
				if b[i] != utf8Bytes[i] {
					t.Fatalf("iter %d: byte %d = %#x, want %#x", iter, i, utf8Bytes[i], b[i])
				}
			}
		}

		// Gocene's CharsRefBuilder stores code points ([]rune), whereas
		// Lucene's stores UTF-16 code units (char[]). Re-encode the decoded
		// runes to UTF-16 so the code-unit count and per-unit comparison
		// match the Java assertions on utf16.length()/utf16.charAt().
		utf16Out.CopyUTF8Bytes(utf8Bytes, 0, len(utf8Bytes))
		decoded := utf16.Encode(utf16Out.Chars())
		if len(decoded) != 20 {
			t.Fatalf("iter %d: decoded length = %d, want 20", iter, len(decoded))
		}
		for i := 0; i < 20; i++ {
			if decoded[i] != expected[i] {
				t.Fatalf("iter %d: char %d = %#x, want %#x", iter, i, decoded[i], expected[i])
			}
		}
	}
}

// TestIndexWriterUnicode_AllUnicodeChars ports testAllUnicodeChars
// (LUCENE-510): every valid code point must survive a UTF-16 -> UTF-8 -> UTF-16
// round-trip and match Go's native UTF-8 encoding.
func TestIndexWriterUnicode_AllUnicodeChars(t *testing.T) {
	utf16Out := util.NewCharsRefBuilder()
	chars := make([]uint16, 2)

	for ch := 0; ch < 0x0010FFFF; ch++ {
		if ch == 0xd800 {
			// Skip invalid code points (the surrogate range).
			ch = 0xe000
		}

		var l int
		if ch <= 0xffff {
			chars[0] = uint16(ch)
			l = 1
		} else {
			chars[0] = uint16(((ch - 0x10000) >> 10) + util.UniSurHighStart)
			chars[1] = uint16(((ch - 0x10000) & 0x3FF) + util.UniSurLowStart)
			l = 2
		}

		utf8Buf := make([]byte, util.MaxUTF8Length(l))
		n := util.UTF16ToUTF8Chars(chars, 0, l, utf8Buf)
		utf8Bytes := utf8Buf[:n]

		s1 := string(utf16.Decode(chars[:l]))
		s2 := string(utf8Bytes)
		if s1 != s2 {
			t.Fatalf("codepoint %#x: utf8 decode = %q, want %q", ch, s2, s1)
		}

		utf16Out.CopyUTF8Bytes(utf8Bytes, 0, len(utf8Bytes))
		if utf16Out.String() != s1 {
			t.Fatalf("codepoint %#x: CharsRefBuilder = %q, want %q", ch, utf16Out.String(), s1)
		}

		b := []byte(s1)
		if len(utf8Bytes) != len(b) {
			t.Fatalf("codepoint %#x: utf8 length = %d, want %d", ch, len(utf8Bytes), len(b))
		}
		for j := range utf8Bytes {
			if utf8Bytes[j] != b[j] {
				t.Fatalf("codepoint %#x: byte %d = %#x, want %#x", ch, j, utf8Bytes[j], b[j])
			}
		}
	}

	// Guard against the constants drifting; utf8 import is otherwise unused.
	if utf8.RuneError != 0xFFFD {
		t.Fatalf("unexpected utf8.RuneError %#x", utf8.RuneError)
	}
}

// TestIndexWriterUnicode_EmbeddedFFFF ports testEmbeddedFFFF.
//
// Java indexes two documents, one containing the term "a￿b" (with an
// embedded U+FFFF), and asserts that DirectoryReader.docFreq returns 1 for it.
func TestIndexWriterUnicode_EmbeddedFFFF(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	tf, _ := document.NewTextField("field", "a a￿b", false)
	doc.Add(tf)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	doc = document.NewDocument()
	tf2, _ := document.NewTextField("field", "a", false)
	doc.Add(tf2)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()

	terms, err := r.Terms("field")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	if terms == nil {
		t.Fatal("Terms(field) returned nil")
	}
	te, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	found, err := te.SeekExact(schema.NewTerm("field", "a￿b"))
	if err != nil {
		t.Fatalf("SeekExact: %v", err)
	}
	if !found {
		t.Fatal("term \"a\\uffffb\" not found")
	}
	df, err := te.DocFreq()
	if err != nil {
		t.Fatalf("DocFreq: %v", err)
	}
	if df != 1 {
		t.Fatalf("DocFreq = %d, want 1", df)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIndexWriterUnicode_InvalidUTF16 ports testInvalidUTF16 (LUCENE-510).
// Skipped: requires indexing each utf8Data input and reading it back via
// stored fields and docFreq.
func TestIndexWriterUnicode_InvalidUTF16(t *testing.T) {
	t.Fatal("GOC-4184: IndexWriter/DirectoryReader indexing round-trip not yet ported")
	_ = utf8Data
}

// TestIndexWriterUnicode_TermUTF16SortOrder ports testTermUTF16SortOrder.
//
// It indexes random single-char and surrogate-pair terms, then verifies that
// the terms dictionary iterates in Unicode code-point order.
func TestIndexWriterUnicode_TermUTF16SortOrder(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	riw := indexTestutil.NewWithConfig(w, 1, indexTestutil.Config{
		CommitProbability:     0,
		ForceMergeProbability: 0,
	})

	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.Freeze()

	allTerms := make(map[string]struct{})
	const num = 200
	for i := 0; i < num; i++ {
		var s string
		if rnd.Intn(2) == 0 {
			// single char
			var r rune
			if rnd.Intn(2) == 0 {
				// above surrogates
				r = rune(rnd.Intn(0xffff-0xdfff) + 0xe000)
			} else {
				// below surrogates
				r = rune(rnd.Intn(0xd800))
			}
			s = string(r)
		} else {
			// surrogate pair
			hi := rune(rnd.Intn(0xdbff-0xd800+1) + 0xd800)
			lo := rune(rnd.Intn(0xdfff-0xdc00+1) + 0xdc00)
			s = string([]rune{hi, lo})
		}
		allTerms[s] = struct{}{}

		doc := document.NewDocument()
		f, _ := document.NewField("f", s, ft)
		doc.Add(f)
		if err := riw.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
		if (i+1)%42 == 0 {
			if err := riw.Commit(); err != nil {
				t.Fatalf("Commit %d: %v", i, err)
			}
		}
	}

	r, err := riw.GetReader()
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	defer r.Close()

	terms, err := r.Terms("f")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	if terms == nil {
		t.Fatal("Terms(f) returned nil")
	}

	it, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	var prev string
	seen := make(map[string]struct{})
	for {
		te, err := it.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if te == nil {
			break
		}
		cur := te.Text()
		seen[cur] = struct{}{}
		if prev != "" && cur < prev {
			t.Errorf("terms out of order: %q before %q", prev, cur)
		}
		prev = cur
	}

	for term := range allTerms {
		if _, ok := seen[term]; !ok {
			t.Errorf("term %q not found in dictionary", term)
		}
	}

	if err := riw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
