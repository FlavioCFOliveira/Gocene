// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "unicode"

// Sorani (Central Kurdish) Unicode code points used by the
// normalisation table. The names match the constants in
// org.apache.lucene.analysis.ckb.SoraniNormalizer.
const (
	soraniYEH            = 0x064A
	soraniDOTLESS_YEH    = 0x0649
	soraniFARSI_YEH      = 0x06CC
	soraniKAF            = 0x0643
	soraniKEHEH          = 0x06A9
	soraniHEH            = 0x0647
	soraniAE             = 0x06D5
	soraniZWNJ           = 0x200C
	soraniHEH_DOACHASHME = 0x06BE
	soraniTEH_MARBUTA    = 0x0629
	soraniREH            = 0x0631
	soraniRREH           = 0x0695
	soraniRREH_ABOVE     = 0x0692
	soraniTATWEEL        = 0x0640
	soraniFATHATAN       = 0x064B
	soraniDAMMATAN       = 0x064C
	soraniKASRATAN       = 0x064D
	soraniFATHA          = 0x064E
	soraniDAMMA          = 0x064F
	soraniKASRA          = 0x0650
	soraniSHADDA         = 0x0651
	soraniSUKUN          = 0x0652
)

// SoraniNormalizer normalises the Unicode representation of Sorani
// (Central Kurdish) text. It standardises character variants (yeh,
// kaf, heh) and removes diacritical marks and zero-width
// non-joiners.
//
// This is the Go port of
// org.apache.lucene.analysis.ckb.SoraniNormalizer from Apache Lucene
// 10.4.0.
type SoraniNormalizer struct{}

// NewSoraniNormalizer returns a fresh normaliser. The receiver carries
// no state, so a single instance may be safely shared across
// goroutines.
func NewSoraniNormalizer() *SoraniNormalizer {
	return &SoraniNormalizer{}
}

// Normalize rewrites s (interpreted as a sequence of Unicode code
// points) into a new []rune that applies the Sorani normalisation
// rules.
//
// Deviation from Lucene: the reference implementation mutates a
// char[] in place via index manipulation; this port returns a fresh
// rune slice to avoid the read/write overlap that would otherwise
// require an explicit cursor for in-place compaction. Higher-level
// filters that operate on []byte CharTermAttribute buffers should
// call NormalizeBytes.
func (n *SoraniNormalizer) Normalize(s []rune) []rune {
	out := make([]rune, 0, len(s))
	for i := 0; i < len(s); i++ {
		r := s[i]
		switch r {
		case soraniYEH, soraniDOTLESS_YEH:
			out = append(out, soraniFARSI_YEH)
		case soraniKAF:
			out = append(out, soraniKEHEH)
		case soraniZWNJ:
			// Strip the ZWNJ; if the preceding rune is HEH, promote it
			// to AE in the output.
			if len(out) > 0 && out[len(out)-1] == soraniHEH {
				out[len(out)-1] = soraniAE
			}
			// (do not append the ZWNJ itself)
		case soraniHEH:
			// HEH at end-of-word becomes AE; mid-word HEH is preserved.
			if i == len(s)-1 {
				out = append(out, soraniAE)
			} else {
				out = append(out, soraniHEH)
			}
		case soraniTEH_MARBUTA:
			out = append(out, soraniAE)
		case soraniHEH_DOACHASHME:
			out = append(out, soraniHEH)
		case soraniREH:
			// Leading REH becomes RREH.
			if len(out) == 0 {
				out = append(out, soraniRREH)
			} else {
				out = append(out, soraniREH)
			}
		case soraniRREH_ABOVE:
			out = append(out, soraniRREH)
		case soraniTATWEEL, soraniFATHATAN, soraniDAMMATAN, soraniKASRATAN,
			soraniFATHA, soraniDAMMA, soraniKASRA, soraniSHADDA, soraniSUKUN:
			// Drop diacritics and tatweel.
		default:
			if unicode.Is(unicode.Cf, r) {
				// Drop Unicode format characters to mirror Java's
				// Character.getType(ch) == Character.FORMAT check.
				continue
			}
			out = append(out, r)
		}
	}
	return out
}

// NormalizeString applies Normalize to s and returns the result as a
// string.
func (n *SoraniNormalizer) NormalizeString(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	return string(n.Normalize(runes))
}

// NormalizeBytes is the UTF-8 entry point used by token filters that
// keep their term text as a byte slice.
func (n *SoraniNormalizer) NormalizeBytes(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	return []byte(n.NormalizeString(string(b)))
}
