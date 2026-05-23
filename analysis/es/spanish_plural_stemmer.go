// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package es

// SpanishPluralStemmer implements a plural-to-singular stemming algorithm for
// Spanish, following the rules described at:
// http://www.wikilengua.org/index.php/Plural_(formación)
//
// This is the Go port of
// org.apache.lucene.analysis.es.SpanishPluralStemmer from Apache Lucene
// 10.4.0.
//
// Deviation: Java uses CharArraySet (case-insensitive hash set of char[])
// for invariants/specialCases. Go uses plain map[string]struct{} with
// lower-case keys; words are pre-normalised by removeAccents before lookup.
type SpanishPluralStemmer struct{}

// NewSpanishPluralStemmer creates a new SpanishPluralStemmer.
func NewSpanishPluralStemmer() *SpanishPluralStemmer { return &SpanishPluralStemmer{} }

// Stem applies plural-to-singular stemming to s[:length] in-place and returns
// the new length.
func (st *SpanishPluralStemmer) Stem(s []rune, length int) int {
	if length < 4 {
		return length
	}
	removeAccents(s, length)
	if pluralInvariant(s, length) {
		return length
	}
	if pluralSpecial(s, length) {
		return length - 2
	}
	if s[length-1] != 's' {
		return length
	}
	// last char is 's'
	if !isVowel(s[length-2]) {
		// no vowel before 's' → singular ends in consonant
		return length - 1
	}
	if s[length-4] == 'q' ||
		(s[length-4] == 'g' && s[length-3] == 'u' &&
			(s[length-2] == 'i' || s[length-2] == 'e')) {
		// maniquis, caquis, parques
		return length - 1
	}
	if isVowel(s[length-4]) && s[length-3] == 'r' && s[length-2] == 'e' {
		// escaneres, alfileres, amores, cables
		return length - 2
	}
	if isVowel(s[length-4]) &&
		(s[length-3] == 'd' || s[length-3] == 'l' || s[length-3] == 'n' || s[length-3] == 'x') &&
		s[length-2] == 'e' {
		// abades, comerciales, faxes, relojes
		return length - 2
	}
	if (s[length-3] == 'y' || s[length-3] == 'u') && s[length-2] == 'e' {
		// bambues, leyes
		return length - 2
	}
	if (s[length-4] == 'u' || s[length-4] == 'l' || s[length-4] == 'r' ||
		s[length-4] == 't' || s[length-4] == 'n') &&
		s[length-3] == 'i' && s[length-2] == 'e' {
		// jabalies, israelies, maniquies
		return length - 2
	}
	if s[length-3] == 's' && s[length-2] == 'e' {
		// reses
		return length - 2
	}
	if isVowel(s[length-3]) && s[length-2] == 'i' {
		// jerseis → jersey
		s[length-2] = 'y'
		return length - 1
	}
	if s[length-3] == 'd' && s[length-2] == 'i' {
		// brandis → brandy
		s[length-2] = 'y'
		return length - 1
	}
	if s[length-2] == 'e' && s[length-3] == 'c' {
		// voces → voz
		s[length-3] = 'z'
		return length - 2
	}
	if isVowel(s[length-2]) {
		// remove last 's': jabalís, casas, coches, etc.
		return length - 1
	}
	return length
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func isVowel(c rune) bool {
	return c == 'a' || c == 'e' || c == 'i' || c == 'o' || c == 'u'
}

func removeAccents(s []rune, length int) {
	for i := 0; i < length; i++ {
		switch s[i] {
		case 'à', 'á', 'â', 'ä':
			s[i] = 'a'
		case 'ò', 'ó', 'ô', 'ö':
			s[i] = 'o'
		case 'è', 'é', 'ê', 'ë':
			s[i] = 'e'
		case 'ù', 'ú', 'û', 'ü':
			s[i] = 'u'
		case 'ì', 'í', 'î', 'ï':
			s[i] = 'i'
		}
	}
}

func runeKey(s []rune, length int) string { return string(s[:length]) }

func pluralInvariant(s []rune, length int) bool {
	_, ok := pluralInvariants[runeKey(s, length)]
	return ok
}

func pluralSpecial(s []rune, length int) bool {
	_, ok := pluralSpecialCases[runeKey(s, length)]
	return ok
}

// pluralInvariants contains words whose plural and singular forms are
// identical; they must not be stemmed.
var pluralInvariants = func() map[string]struct{} {
	words := []string{
		"abrebotellas", "abrecartas", "abrelatas", "afueras", "albatros",
		"albricias", "aledanos", "alexis", "alicates", "analisis",
		"andurriales", "antitesis", "anicos", "apendicitis", "apocalipsis",
		"arcoiris", "aries", "bilis", "boletus", "boris", "brindis",
		"cactus", "canutas", "caries", "cascanueces", "cascarrabias",
		"ciempies", "cifosis", "cortaplumas", "corpus", "cosmos",
		"cosquillas", "creces", "crisis", "cuatrocientas", "cuatrocientos",
		"cuelgacapas", "cuentacuentos", "cuentapasos", "cumpleanos",
		"doscientas", "doscientos", "dosis", "enseres", "entonces",
		"esponsales", "estatus", "exequias", "fauces", "forceps",
		"fotosintesis", "gafas", "gafotas", "gargaras", "gris",
		"honorarios", "ictus", "jueves", "lapsus", "lavacoches",
		"lavaplatos", "limpiabotas", "lunes", "maitines", "martes",
		"mondadientes", "novecientas", "novecientos", "nupcias",
		"ochocientas", "ochocientos", "pais", "paris", "parabrisas",
		"paracaidas", "parachoques", "paraguas", "pararrayos",
		"pisapapeles", "piscis", "portaaviones", "portamaletas",
		"portamantas", "quinientas", "quinientos", "quitamanchas",
		"recogepelotas", "rictus", "rompeolas", "sacacorchos",
		"sacapuntas", "saltamontes", "salvavidas", "seis", "seiscientas",
		"seiscientos", "setecientas", "setecientos", "sintesis", "tenis",
		"tifus", "trabalenguas", "vacaciones", "venus", "versus",
		"viacrucis", "virus", "viveres", "volandas",
	}
	m := make(map[string]struct{}, len(words))
	for _, w := range words {
		m[w] = struct{}{}
	}
	return m
}()

// pluralSpecialCases contains words ending in "es" that have specific
// singular forms (len-2 strips the "es").
var pluralSpecialCases = func() map[string]struct{} {
	words := []string{
		"yoes", "noes", "sies", "clubes", "faralaes", "albalaes",
		"itemes", "albumes", "sandwiches", "relojes", "bojes",
		"contrarreloj", "carcajes",
	}
	m := make(map[string]struct{}, len(words))
	for _, w := range words {
		m[w] = struct{}{}
	}
	return m
}()
