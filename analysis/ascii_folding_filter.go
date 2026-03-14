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
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

	if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
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

// foldLatinExtendedA folds Latin Extended-A characters.
func (f *ASCIIFoldingFilter) foldLatinExtendedA(r rune) []rune {
	// This is a simplified implementation
	// Full implementation would cover all characters in U+0100 to U+017F

	switch r {
	// Polish characters
	case 0x0141: // Ł -> L
		return []rune{'L'}
	case 0x0142: // ł -> l
		return []rune{'l'}
	case 0x0143: // Ń -> N
		return []rune{'N'}
	case 0x0144: // ń -> n
		return []rune{'n'}
	case 0x015A: // Ś -> S
		return []rune{'S'}
	case 0x015B: // ś -> s
		return []rune{'s'}
	case 0x0179: // Ź -> Z
		return []rune{'Z'}
	case 0x017A: // ź -> z
		return []rune{'z'}
	case 0x017B: // Ż -> Z
		return []rune{'Z'}
	case 0x017C: // ż -> z
		return []rune{'z'}

	// Other common characters
	case 0x0152: // Œ -> OE
		return []rune{'O', 'E'}
	case 0x0153: // œ -> oe
		return []rune{'o', 'e'}

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
