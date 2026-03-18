// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// FinnishStopWords contains common Finnish stop words.
var FinnishStopWords = []string{
	"aiemmin", "aihe", "aiheesta", "aika", "aikaa", "aikaan", "aikaisemmin", "aikana",
	"aikoina", "aikuiset", "aina", "ainakaan", "ainakin", "ainoa", "ainoat", "aiomme",
	"aion", "aionsa", "aiot", "aioitte", "aioit", "aist", "aivan", "ajan", "alas",
	"alemmas", "alkuisin", "alkuun", "alla", "alle", "aloitamme", "aloitan", "aloitat",
	"aloitatte", "aloitettava", "aloitettevaksi", "aloitettu", "aloittaa", "aloittamatta",
	"aloitti", "aloittivat", "alta", "aluksi", "alussa", "alusta", "annettava",
	"annettevaksi", "annettu", "antaa", "antamatta", "antoi", "apu", "asia", "asiaa",
	"asian", "asiasta", "asiat", "asioiden", "asioihin", "asioita", "asti", "avuksi",
	"edelle", "edelleen", "edellä", "edeltä", "edemmäs", "edes", "edessä", "edestä",
	"ehkä", "ei", "eikä", "eilen", "eivät", "eli", "ellei", "elleivät", "ellemme",
	"ellen", "ellet", "ellette", "emme", "en", "enemmän", "ennen", "ensi", "ensimmäinen",
	"ensimmäiseksi", "ensimmäisen", "ensimmäisenä", "ensimmäiset", "ensimmäisiksi",
	"ensimmäisinä", "ensimmäisiä", "ensin", "entinen", "entisen", "entisiä", "enää",
	"eri", "erittäin", "erityisesti", "eräät", "eräs", "et", "ette", "ettei", "että",
	"haikki", "halua", "haluaa", "haluamatta", "haluamme", "haluan", "haluat",
	"haluatte", "haluavat", "halunnut", "halusi", "halusivat", "halutessa", "haluton",
	"he", "hei", "heidän", "heidät", "heihin", "heille", "heillä", "heiltä", "heissä",
	"heistä", "heitä", "helposti", "heti", "hetkellä", "hieman", "hitaasti", "hoikka",
	"huolimatta", "huomenna", "hyvin", "hyvä", "hyvät", "hyviä", "hän", "häneen",
	"hänelle", "hänellä", "häneltä", "hänen", "hänessä", "hänestä", "hänet", "häneltä",
	"ihan", "ilman", "ilmeisesti", "itse", "itsensä", "itseään", "ja", "jo", "johon",
	"joiden", "joihin", "joiksi", "joilla", "joille", "joilta", "joina", "joissa",
	"joista", "joita", "joka", "jokainen", "jokin", "joko", "joku", "jolla", "jolle",
	"jolloin", "jolta", "jompikumpi", "jonka", "jona", "jonkin", "jonne", "joo",
	"jopa", "jos", "joskus", "jossa", "josta", "jota", "jotain", "joten", "jotenkin",
	"jotenkuten", "jotka", "jotta", "joukkoon", "joukossa", "joukosta", "juh", "jälkeen",
	"jälleen", "kaikki", "kaikkia", "kaikkiaan", "kaikkiin", "kaikille", "kaikilta",
	"kaikkena", "kannalta", "kannattaa", "kanssa", "kanssaan", "kanssamme", "kappale",
	"kautta", "kenen", "keneltä", "kenties", "kerran", "kerta", "kertaa", "kesken",
	"keskenään", "keskimäärin", "ketkä", "kiitos", "kohti", "koko", "kokonaan",
	"kolmas", "kolme", "kolmen", "kolmesti", "koska", "koskaan", "kovin", "kuitenkaan",
	"kuitenkin", "kuka", "kukaan", "kukin", "kumpainen", "kumpi", "kumpikaan",
	"kumpikin", "kun", "kuten", "kuuden", "kuusi", "kuutta", "kyllä", "kymmenen",
	"kyse", "liian", "liki", "lisäksi", "lisää", "lla", "luo", "mahdollisimman",
	"mahdollista", "me", "meidän", "meidät", "meille", "meillä", "meiltä", "meissä",
	"meistä", "meitä", "melkein", "melko", "menee", "meneet", "menemme", "menen",
	"menet", "menette", "menevät", "meni", "menimme", "menin", "menit", "menivät",
	"mennessä", "mennessään", "mennyt", "menossa", "mihin", "mikä", "mikään",
	"mikäli", "mille", "milloin", "milloinkaan", "millä", "miltä", "minkä", "minne",
	"minua", "minulla", "minulle", "minulta", "minun", "minussa", "minusta", "minut",
	"minä", "missä", "mistä", "miten", "mitä", "moi", "molemmat", "mones", "monesti",
	"monet", "moni", "monia", "moniaan", "monien", "muiden", "muita", "mukaan",
	"mukaansa", "mukana", "mutta", "muu", "muualla", "muuallaan", "muualle", "muualta",
	"muuanne", "muulloin", "muun", "muut", "muuta", "muutama", "muutaman", "muuten",
	"myöhemmin", "myös", "myöskin", "myötä", "ne", "neljä", "neljän", "neljää",
	"niiden", "niin", "niinä", "niistä", "niitä", "noin", "nopeammin", "nopeasti",
	"nopeiten", "nro", "nuo", "nyt", "näiden", "näin", "näissä", "näissähin",
	"näissältä", "näitä", "nämä", "ohi", "oikea", "oikealla", "oikein", "ole", "olemme",
	"olen", "olet", "olette", "oleva", "olevan", "olevat", "oli", "olimme", "olin",
	"olisi", "olisimme", "olisin", "olisit", "olisitte", "olisivat", "olit", "olitte",
	"olivat", "olla", "olleet", "olli", "ollut", "oma", "omaa", "omaan", "omaksi",
	"omalle", "omalta", "oman", "omassa", "omat", "omia", "omien", "omiin", "omille",
	"omilta", "omissa", "omista", "on", "onkin", "onko", "ovat", "paikalla", "paitsi",
	"pakosti", "paljon", "paremmin", "parempi", "parhaillaan", "parhaiten", "perusteella",
	"peräti", "pian", "pieneen", "pieneksi", "pienelle", "pieneltä", "pienempi",
	"pienempiä", "pienet", "pieni", "pienin", "pienineen", "pieninään", "pienissä",
	"pienistä", "pienitä", "pieneksi", "päälle", "runsaasti", "saakka", "sadam",
	"sama", "samaa", "samaan", "samalla", "samallalta", "samalle", "saman", "samassa",
	"samat", "sami", "samoin", "sata", "sataa", "satojen", "se", "seitsemän", "sekä",
	"sen", "seuraavat", "siellä", "sieltä", "siihen", "siinä", "siis", "siitä",
	"sijaan", "siksi", "sille", "silloin", "sillä", "siltä", "sinä", "sinne",
	"sinua", "sinulla", "sinulle", "sinulta", "sinun", "sinussa", "sinusta", "sinut",
	"sisällä", "sitä", "sitten", "siten", "sittenkin", "sittemmin", "sivu", "sivulta",
	"sivulle", "sivulla", "sivun", "sivut", "sopi", "suoraan", "suureksi", "suuren",
	"suuret", "suuri", "suuria", "suurin", "suurineen", "suurinään", "suurissa",
	"suurista", "suuritä", "suurten", "taa", "taas", "taemmas", "tahansa", "tai",
	"takaa", "takaisin", "takana", "takia", "talle", "tapa", "tavalla", "tavoitteena",
	"te", "teidän", "teidät", "teille", "teillä", "teiltä", "teissä", "teistä",
	"teitä", "tietysti", "todella", "toinen", "toisen", "toisiin", "toisia",
	"toisiaan", "toisilla", "toisille", "toisilta", "toisina", "toisineen",
	"toisissa", "toisista", "toisitta", "toisittain", "toivottavasti", "toki",
	"tosin", "totta", "tuhat", "tule", "tulee", "tuleeko", "tulemaan", "tulemme",
	"tulen", "tulet", "tulette", "tulevat", "tuli", "tulimme", "tulin", "tulisi",
	"tulisimme", "tulisin", "tulisit", "tulisitte", "tulisivat", "tulit", "tulitte",
	"tulivat", "tulla", "tulleet", "tullut", "tuntuu", "tuskin", "tykö", "tähän",
	"tällä", "tällöin", "tämä", "tämän", "tänne", "tänä", "tänään", "tässä",
	"tästä", "täten", "tätä", "täysin", "täytyy", "täällä", "täältä", "ulkopuolella",
	"usea", "useasti", "useimmiten", "usein", "uudeksi", "uuden", "uudet", "uusi",
	"uusia", "uusien", "uusin", "uusineen", "uusissa", "uusista", "vaan", "vahemmän",
	"vai", "vaihe", "vaikea", "vaikean", "vaikeat", "vaikeilta", "vaikeissa",
	"vaikeista", "vaikka", "vain", "varmasti", "varsin", "varsinkin", "varten",
	"vasen", "vasemmalla", "vasemmalta", "vasemmalle", "vasemman", "vasemmassa",
	"vasemmat", "vasempaan", "vasempana", "vasempi", "vasempia", "vasempien",
	"vasta", "vastaan", "vastakkain", "vastan", "verran", "vielä", "vierekkäin",
	"vieressä", "viiden", "viime", "viimeinen", "viimeisen", "viimeiset", "viimeksi",
	"viisi", "voi", "voidaan", "voimme", "voin", "voisi", "voisimme", "voisin",
	"voisit", "voisitte", "voisivat", "voit", "voitte", "voivat", "vuoden", "vuoksi",
	"vuosi", "vuosien", "vuosina", "vuotta", "vähemmän", "vähintään", "vähitellen",
	"vai", "yhä", "yhdeksän", "yhden", "yhdessä", "yhtä", "yhtäällä", "yhtäälle",
	"yhtäältä", "yhtään", "yhteen", "yhteensä", "yhteinen", "yhteydessä", "yhteyteen",
	"yksi", "yksin", "yksittäin", "yleensä", "ylemmäs", "yli", "ylös", "ympäri",
	"äskettäin", "äär",
}

// FinnishAnalyzer is an analyzer for Finnish language text.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.fi.FinnishAnalyzer.
//
// FinnishAnalyzer uses the StandardTokenizer with Finnish stop words removal
// and light stemming.
type FinnishAnalyzer struct {
	*BaseAnalyzer

	// stopWords is the set of stop words to filter
	stopWords *CharArraySet
}

// NewFinnishAnalyzer creates a new FinnishAnalyzer with default Finnish stop words.
func NewFinnishAnalyzer() *FinnishAnalyzer {
	stopSet := GetWordSetFromStrings(FinnishStopWords, true)
	return NewFinnishAnalyzerWithWords(stopSet)
}

// NewFinnishAnalyzerWithWords creates a FinnishAnalyzer with custom stop words.
func NewFinnishAnalyzerWithWords(stopWords *CharArraySet) *FinnishAnalyzer {
	a := &FinnishAnalyzer{
		BaseAnalyzer: NewAnalyzer(),
		stopWords:    stopWords,
	}

	// Set up the analysis chain
	a.TokenizerFactory = NewStandardTokenizerFactory()
	a.AddTokenFilter(NewLowerCaseFilterFactory())
	a.AddTokenFilter(NewStopFilterFactoryWithWords(stopWords))
	a.AddTokenFilter(NewFinnishLightStemFilterFactory())

	return a
}

// TokenStream creates a TokenStream for analyzing text.
func (a *FinnishAnalyzer) TokenStream(fieldName string, reader io.Reader) (TokenStream, error) {
	return a.BaseAnalyzer.TokenStream(fieldName, reader)
}

// GetStopWords returns the stop words used by this analyzer.
func (a *FinnishAnalyzer) GetStopWords() *CharArraySet {
	return a.stopWords
}

// SetStopWords sets the stop words for this analyzer.
func (a *FinnishAnalyzer) SetStopWords(stopWords *CharArraySet) {
	a.stopWords = stopWords
}

// Ensure FinnishAnalyzer implements Analyzer
var _ Analyzer = (*FinnishAnalyzer)(nil)
var _ AnalyzerInterface = (*FinnishAnalyzer)(nil)

// FinnishLightStemFilter implements light stemming for Finnish.
type FinnishLightStemFilter struct {
	*BaseTokenFilter
}

// NewFinnishLightStemFilter creates a new FinnishLightStemFilter.
func NewFinnishLightStemFilter(input TokenStream) *FinnishLightStemFilter {
	return &FinnishLightStemFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
}

// IncrementToken processes the next token and applies light stemming.
func (f *FinnishLightStemFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}

	if hasToken {
		if attr := f.GetAttributeSource().GetAttribute("CharTermAttribute"); attr != nil {
			if termAttr, ok := attr.(CharTermAttribute); ok {
				term := termAttr.String()
				stemmed := finnishLightStem(term)
				if stemmed != term {
					termAttr.SetEmpty()
					termAttr.AppendString(stemmed)
				}
			}
		}
	}

	return hasToken, nil
}

// finnishLightStem applies light Finnish stemming.
func finnishLightStem(term string) string {
	if len(term) < 4 {
		return term
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(term)
	length := len(runes)

	// Remove common Finnish suffixes
	switch {
	// -inen, -iset (adjective endings)
	case length > 4 && string(runes[length-4:]) == "inen":
		return string(runes[:length-3])
	case length > 4 && string(runes[length-4:]) == "iset":
		return string(runes[:length-3])
	// -ksi, -lle, -lta, -ssa, -sta (case endings)
	case length > 3 && (string(runes[length-3:]) == "ksi" || string(runes[length-3:]) == "lle" ||
		string(runes[length-3:]) == "lta" || string(runes[length-3:]) == "ssa" ||
		string(runes[length-3:]) == "sta"):
		return string(runes[:length-2])
	// -n (genitive)
	case length > 1 && runes[length-1] == 'n':
		return string(runes[:length-1])
	// -t (plural)
	case length > 1 && runes[length-1] == 't':
		return string(runes[:length-1])
	}

	return term
}

// FinnishLightStemFilterFactory creates FinnishLightStemFilter instances.
type FinnishLightStemFilterFactory struct{}

// NewFinnishLightStemFilterFactory creates a new FinnishLightStemFilterFactory.
func NewFinnishLightStemFilterFactory() *FinnishLightStemFilterFactory {
	return &FinnishLightStemFilterFactory{}
}

// Create creates a new FinnishLightStemFilter.
func (f *FinnishLightStemFilterFactory) Create(input TokenStream) TokenFilter {
	return NewFinnishLightStemFilter(input)
}

// Ensure FinnishLightStemFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*FinnishLightStemFilterFactory)(nil)
