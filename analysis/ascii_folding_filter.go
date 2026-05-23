// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"unicode"
)

// ASCIIFoldingFilter converts non-ASCII characters to their ASCII equivalents.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.ASCIIFoldingFilter.
//
// ASCIIFoldingFilter folds Unicode characters in the Basic Latin range
// (first 127 characters) to their ASCII equivalents. This is useful for
// normalizing text for search and indexing.
type ASCIIFoldingFilter struct {
	*BaseTokenFilter

	// preserveOriginal if true, keeps the original token and adds a folded version
	preserveOriginal bool

	// pendingOutput stores the folded version when preserveOriginal is true
	pendingOutput string
}

// NewASCIIFoldingFilter creates a new ASCIIFoldingFilter wrapping the given input.
func NewASCIIFoldingFilter(input TokenStream) *ASCIIFoldingFilter {
	return &ASCIIFoldingFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// NewASCIIFoldingFilterWithOptions creates a new ASCIIFoldingFilter with options.
func NewASCIIFoldingFilterWithOptions(input TokenStream, preserveOriginal bool) *ASCIIFoldingFilter {
	return &ASCIIFoldingFilter{
		BaseTokenFilter:  NewBaseTokenFilter(input),
		preserveOriginal: preserveOriginal,
	}
}

// IncrementToken processes the next token and applies ASCII folding.
func (f *ASCIIFoldingFilter) IncrementToken() (bool, error) {
	// If we have pending output from preserveOriginal, return it
	if f.pendingOutput != "" {
		if attr := f.GetAttributeSource().GetAttribute(CharTermAttributeType); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				termAttr.SetEmpty()
				termAttr.AppendString(f.pendingOutput)
			}
		}
		f.pendingOutput = ""
		return true, nil
	}

	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if !hasToken {
		return false, nil
	}

	if attr := f.GetAttributeSource().GetAttribute(CharTermAttributeType); attr != nil {
		if termAttr, ok := attr.(CharTermAttribute); ok {
			term := termAttr.String()
			folded := f.foldToASCII(term)

			if folded != term {
				if f.preserveOriginal {
					f.pendingOutput = folded
					// Return original first
					return true, nil
				}
				termAttr.SetEmpty()
				termAttr.AppendString(folded)
			}
		}
	}

	return true, nil
}

// foldToASCII folds the given string to ASCII.
func (f *ASCIIFoldingFilter) foldToASCII(s string) string {
	var result []rune
	for _, r := range s {
		folded := f.foldRune(r)
		result = append(result, folded...)
	}
	return string(result)
}

// foldRune folds a single rune to its ASCII equivalent(s).
func (f *ASCIIFoldingFilter) foldRune(r rune) []rune {
	// Basic Latin (ASCII) - no folding needed
	if r < 128 {
		return []rune{r}
	}

	// Latin-1 Supplement (U+0080 to U+00FF)
	if r >= 0x00C0 && r <= 0x00FF {
		return f.foldLatin1(r)
	}

	// Latin Extended-A (U+0100 to U+017F)
	if r >= 0x0100 && r <= 0x017F {
		return f.foldLatinExtendedA(r)
	}

	// Latin Extended-B (U+0180 to U+024F)
	if r >= 0x0180 && r <= 0x024F {
		return f.foldLatinExtendedB(r)
	}

	// General folding using Unicode decomposition
	// For characters not in the tables, try to decompose
	if unicode.Is(unicode.Mn, r) {
		// Skip combining marks
		return nil
	}

	// Default: return the original character
	return []rune{r}
}

// foldLatin1 folds Latin-1 Supplement characters.
func (f *ASCIIFoldingFilter) foldLatin1(r rune) []rune {
	switch r {
	// Uppercase vowels with grave accent
	case 0x00C0: // À -> A
		return []rune{'A'}
	case 0x00C8: // È -> E
		return []rune{'E'}
	case 0x00CC: // Ì -> I
		return []rune{'I'}
	case 0x00D2: // Ò -> O
		return []rune{'O'}
	case 0x00D9: // Ù -> U
		return []rune{'U'}

	// Uppercase vowels with acute accent
	case 0x00C1: // Á -> A
		return []rune{'A'}
	case 0x00C9: // É -> E
		return []rune{'E'}
	case 0x00CD: // Í -> I
		return []rune{'I'}
	case 0x00D3: // Ó -> O
		return []rune{'O'}
	case 0x00DA: // Ú -> U
		return []rune{'U'}
	case 0x00DD: // Ý -> Y
		return []rune{'Y'}

	// Uppercase vowels with circumflex
	case 0x00C2: // Â -> A
		return []rune{'A'}
	case 0x00CA: // Ê -> E
		return []rune{'E'}
	case 0x00CE: // Î -> I
		return []rune{'I'}
	case 0x00D4: // Ô -> O
		return []rune{'O'}
	case 0x00DB: // Û -> U
		return []rune{'U'}

	// Uppercase vowels with tilde
	case 0x00C3: // Ã -> A
		return []rune{'A'}
	case 0x00D5: // Õ -> O
		return []rune{'O'}

	// Uppercase vowels with diaeresis
	case 0x00C4: // Ä -> A
		return []rune{'A'}
	case 0x00CB: // Ë -> E
		return []rune{'E'}
	case 0x00CF: // Ï -> I
		return []rune{'I'}
	case 0x00D6: // Ö -> O
		return []rune{'O'}
	case 0x00DC: // Ü -> U
		return []rune{'U'}
	case 0x0178: // Ÿ -> Y
		return []rune{'Y'}

	// Uppercase A with ring above
	case 0x00C5: // Å -> A
		return []rune{'A'}

	// Uppercase AE
	case 0x00C6: // Æ -> AE
		return []rune{'A', 'E'}

	// Uppercase C with cedilla
	case 0x00C7: // Ç -> C
		return []rune{'C'}

	// Uppercase ETH
	case 0x00D0: // Ð -> D
		return []rune{'D'}

	// Uppercase N with tilde
	case 0x00D1: // Ñ -> N
		return []rune{'N'}

	// Uppercase O with slash
	case 0x00D8: // Ø -> O
		return []rune{'O'}

	// Uppercase Thorn
	case 0x00DE: // Þ -> TH
		return []rune{'T', 'H'}

	// Lowercase vowels with grave accent
	case 0x00E0: // à -> a
		return []rune{'a'}
	case 0x00E8: // è -> e
		return []rune{'e'}
	case 0x00EC: // ì -> i
		return []rune{'i'}
	case 0x00F2: // ò -> o
		return []rune{'o'}
	case 0x00F9: // ù -> u
		return []rune{'u'}

	// Lowercase vowels with acute accent
	case 0x00E1: // á -> a
		return []rune{'a'}
	case 0x00E9: // é -> e
		return []rune{'e'}
	case 0x00ED: // í -> i
		return []rune{'i'}
	case 0x00F3: // ó -> o
		return []rune{'o'}
	case 0x00FA: // ú -> u
		return []rune{'u'}
	case 0x00FD: // ý -> y
		return []rune{'y'}
	case 0x00FF: // ÿ -> y
		return []rune{'y'}

	// Lowercase vowels with circumflex
	case 0x00E2: // â -> a
		return []rune{'a'}
	case 0x00EA: // ê -> e
		return []rune{'e'}
	case 0x00EE: // î -> i
		return []rune{'i'}
	case 0x00F4: // ô -> o
		return []rune{'o'}
	case 0x00FB: // û -> u
		return []rune{'u'}

	// Lowercase vowels with tilde
	case 0x00E3: // ã -> a
		return []rune{'a'}
	case 0x00F5: // õ -> o
		return []rune{'o'}

	// Lowercase vowels with diaeresis
	case 0x00E4: // ä -> a
		return []rune{'a'}
	case 0x00EB: // ë -> e
		return []rune{'e'}
	case 0x00EF: // ï -> i
		return []rune{'i'}
	case 0x00F6: // ö -> o
		return []rune{'o'}
	case 0x00FC: // ü -> u
		return []rune{'u'}

	// Lowercase a with ring above
	case 0x00E5: // å -> a
		return []rune{'a'}

	// Lowercase ae
	case 0x00E6: // æ -> ae
		return []rune{'a', 'e'}

	// Lowercase c with cedilla
	case 0x00E7: // ç -> c
		return []rune{'c'}

	// Lowercase eth
	case 0x00F0: // ð -> d
		return []rune{'d'}

	// Lowercase n with tilde
	case 0x00F1: // ñ -> n
		return []rune{'n'}

	// Lowercase o with slash
	case 0x00F8: // ø -> o
		return []rune{'o'}

	// Lowercase thorn
	case 0x00FE: // þ -> th
		return []rune{'t', 'h'}

	// German sharp s
	case 0x00DF: // ß -> ss
		return []rune{'s', 's'}

	default:
		return []rune{r}
	}
}

// foldLatinExtendedA folds Latin Extended-A characters (U+0100–U+017F).
// Entries follow the Lucene 10.4.0 ASCIIFoldingFilter mapping table.
func (f *ASCIIFoldingFilter) foldLatinExtendedA(r rune) []rune {
	switch r {
	// A variants
	case 0x0100, 0x0102, 0x0104: // Ā Ă Ą -> A
		return []rune{'A'}
	case 0x0101, 0x0103, 0x0105: // ā ă ą -> a
		return []rune{'a'}

	// C variants
	case 0x0106, 0x0108, 0x010A, 0x010C: // Ć Ĉ Ċ Č -> C
		return []rune{'C'}
	case 0x0107, 0x0109, 0x010B, 0x010D: // ć ĉ ċ č -> c
		return []rune{'c'}

	// D variants
	case 0x010E: // Ď -> D
		return []rune{'D'}
	case 0x010F: // ď -> d
		return []rune{'d'}
	case 0x0110: // Đ -> D
		return []rune{'D'}
	case 0x0111: // đ -> d
		return []rune{'d'}

	// E variants
	case 0x0112, 0x0114, 0x0116, 0x0118, 0x011A: // Ē Ĕ Ė Ę Ě -> E
		return []rune{'E'}
	case 0x0113, 0x0115, 0x0117, 0x0119, 0x011B: // ē ĕ ė ę ě -> e
		return []rune{'e'}

	// G variants
	case 0x011C, 0x011E, 0x0120, 0x0122: // Ĝ Ğ Ġ Ģ -> G
		return []rune{'G'}
	case 0x011D, 0x011F, 0x0121, 0x0123: // ĝ ğ ġ ģ -> g
		return []rune{'g'}

	// H variants
	case 0x0124: // Ĥ -> H
		return []rune{'H'}
	case 0x0125: // ĥ -> h
		return []rune{'h'}
	case 0x0126: // Ħ -> H
		return []rune{'H'}
	case 0x0127: // ħ -> h
		return []rune{'h'}

	// I variants
	case 0x0128, 0x012A, 0x012C, 0x012E, 0x0130: // Ĩ Ī Ĭ Į İ -> I
		return []rune{'I'}
	case 0x0129, 0x012B, 0x012D, 0x012F, 0x0131: // ĩ ī ĭ į ı -> i
		return []rune{'i'}

	// IJ ligature
	case 0x0132: // Ĳ -> IJ
		return []rune{'I', 'J'}
	case 0x0133: // ĳ -> ij
		return []rune{'i', 'j'}

	// J variants
	case 0x0134: // Ĵ -> J
		return []rune{'J'}
	case 0x0135: // ĵ -> j
		return []rune{'j'}

	// K variants
	case 0x0136: // Ķ -> K
		return []rune{'K'}
	case 0x0137: // ķ -> k
		return []rune{'k'}
	case 0x0138: // ĸ -> k
		return []rune{'k'}

	// L variants
	case 0x0139, 0x013B, 0x013D, 0x013F, 0x0141: // Ĺ Ļ Ľ Ŀ Ł -> L
		return []rune{'L'}
	case 0x013A, 0x013C, 0x013E, 0x0140, 0x0142: // ĺ ļ ľ ŀ ł -> l
		return []rune{'l'}

	// N variants
	case 0x0143, 0x0145, 0x0147: // Ń Ņ Ň -> N
		return []rune{'N'}
	case 0x0144, 0x0146, 0x0148, 0x0149: // ń ņ ň ŉ -> n
		return []rune{'n'}
	case 0x014A: // Ŋ -> N
		return []rune{'N'}
	case 0x014B: // ŋ -> n
		return []rune{'n'}

	// O variants
	case 0x014C, 0x014E, 0x0150: // Ō Ŏ Ő -> O
		return []rune{'O'}
	case 0x014D, 0x014F, 0x0151: // ō ŏ ő -> o
		return []rune{'o'}

	// OE ligature
	case 0x0152: // Œ -> OE
		return []rune{'O', 'E'}
	case 0x0153: // œ -> oe
		return []rune{'o', 'e'}

	// R variants
	case 0x0154, 0x0156, 0x0158: // Ŕ Ŗ Ř -> R
		return []rune{'R'}
	case 0x0155, 0x0157, 0x0159: // ŕ ŗ ř -> r
		return []rune{'r'}

	// S variants
	case 0x015A, 0x015C, 0x015E, 0x0160: // Ś Ŝ Ş Š -> S
		return []rune{'S'}
	case 0x015B, 0x015D, 0x015F, 0x0161: // ś ŝ ş š -> s
		return []rune{'s'}
	case 0x017F: // ſ -> s
		return []rune{'s'}

	// T variants
	case 0x0162, 0x0164, 0x0166: // Ţ Ť Ŧ -> T
		return []rune{'T'}
	case 0x0163, 0x0165, 0x0167: // ţ ť ŧ -> t
		return []rune{'t'}

	// U variants
	case 0x0168, 0x016A, 0x016C, 0x016E, 0x0170, 0x0172: // Ũ Ū Ŭ Ů Ű Ų -> U
		return []rune{'U'}
	case 0x0169, 0x016B, 0x016D, 0x016F, 0x0171, 0x0173: // ũ ū ŭ ů ű ų -> u
		return []rune{'u'}

	// W variants
	case 0x0174: // Ŵ -> W
		return []rune{'W'}
	case 0x0175: // ŵ -> w
		return []rune{'w'}

	// Y variants
	case 0x0176: // Ŷ -> Y
		return []rune{'Y'}
	case 0x0177: // ŷ -> y
		return []rune{'y'}
	case 0x0178: // Ÿ -> Y (also in Latin-1 range)
		return []rune{'Y'}

	// Z variants
	case 0x0179, 0x017B, 0x017D: // Ź Ż Ž -> Z
		return []rune{'Z'}
	case 0x017A, 0x017C, 0x017E: // ź ż ž -> z
		return []rune{'z'}

	default:
		return []rune{r}
	}
}

// foldLatinExtendedB folds Latin Extended-B characters.
func (f *ASCIIFoldingFilter) foldLatinExtendedB(r rune) []rune {
	// Simplified implementation
	// Full implementation would cover all characters in U+0180 to U+024F
	return []rune{r}
}
