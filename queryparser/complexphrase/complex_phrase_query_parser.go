// Package complexphrase implements Lucene's ComplexPhraseQueryParser, a
// classic-style query parser that additionally understands wildcards inside
// quoted phrases (e.g. `"jack jo*"`) and rewrites those phrases into the
// equivalent SpanNearQuery of SpanTermQuery / SpanMultiTermQueryWrapper
// clauses. Mirrors org.apache.lucene.queryparser.complexPhrase.ComplexPhraseQueryParser.
package complexphrase

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queryparser"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ComplexPhraseQueryParser extends the classic QueryParser with phrase-level
// wildcard support. When the parser detects a phrase containing one or more
// wildcard characters it produces a SpanNearQuery whose clauses are
// SpanTermQuery / SpanMultiTermQueryWrapper instances; ordinary phrases fall
// back to the classic parser's PhraseQuery output.
type ComplexPhraseQueryParser struct {
	*queryparser.QueryParser
	DefaultField string
	InOrder      bool
}

// NewComplexPhraseQueryParser builds the parser using the default field and
// analyzer. InOrder defaults to true (clauses must appear in phrase order).
func NewComplexPhraseQueryParser(defaultField string, analyzer *analysis.StandardAnalyzer) *ComplexPhraseQueryParser {
	return &ComplexPhraseQueryParser{
		QueryParser:  queryparser.NewQueryParser(defaultField, analyzer),
		DefaultField: defaultField,
		InOrder:      true,
	}
}

// Parse rewrites phrases containing wildcards into SpanNearQuery clauses and
// delegates everything else to the classic parser.
func (p *ComplexPhraseQueryParser) Parse(query string) (search.Query, error) {
	preprocessed, complexPhrases := p.extractComplexPhrases(query)
	classic, err := p.QueryParser.Parse(preprocessed)
	if err != nil {
		return nil, err
	}
	if len(complexPhrases) == 0 {
		return classic, nil
	}
	return p.substituteComplexPhrases(classic, complexPhrases), nil
}

// complexPhrase pairs the placeholder term substituted into the classic query
// with the SpanNearQuery that should ultimately replace it.
type complexPhrase struct {
	Placeholder string
	Field       string
	Span        search.Query
}

// extractComplexPhrases scans for `"..."` segments containing a wildcard, replaces
// each with a unique placeholder term that the classic parser can consume,
// and returns the modified query plus the replacement metadata.
func (p *ComplexPhraseQueryParser) extractComplexPhrases(query string) (string, []*complexPhrase) {
	var b strings.Builder
	b.Grow(len(query))
	var phrases []*complexPhrase
	inPhrase := false
	phraseStart := -1
	currentField := p.DefaultField
	i := 0
	for i < len(query) {
		c := query[i]
		if !inPhrase {
			if c == ':' {
				start := strings.LastIndexAny(b.String(), " \t()") + 1
				cur := b.String()
				if start < len(cur) {
					currentField = cur[start:]
				}
				b.WriteByte(c)
				i++
				continue
			}
			if c == '"' {
				inPhrase = true
				phraseStart = i
				i++
				continue
			}
			b.WriteByte(c)
			i++
			continue
		}
		if c == '"' {
			phrase := query[phraseStart+1 : i]
			i++
			slop := 0
			if i < len(query) && query[i] == '~' {
				i++
				for i < len(query) && query[i] >= '0' && query[i] <= '9' {
					slop = slop*10 + int(query[i]-'0')
					i++
				}
			}
			if containsWildcard(phrase) {
				ph := &complexPhrase{
					Placeholder: "__complexphrase_" + itoa(len(phrases)) + "__",
					Field:       currentField,
					Span:        p.buildSpan(currentField, phrase, slop),
				}
				phrases = append(phrases, ph)
				b.WriteString(ph.Placeholder)
			} else {
				b.WriteByte('"')
				b.WriteString(phrase)
				b.WriteByte('"')
				if slop > 0 {
					b.WriteByte('~')
					b.WriteString(itoa(slop))
				}
			}
			inPhrase = false
			phraseStart = -1
			continue
		}
		i++
	}
	if inPhrase {
		b.WriteString(query[phraseStart:])
	}
	return b.String(), phrases
}

// substituteComplexPhrases walks the classic parser output and swaps any
// placeholder TermQuery for the matching SpanNearQuery.
func (p *ComplexPhraseQueryParser) substituteComplexPhrases(q search.Query, phrases []*complexPhrase) search.Query {
	idx := lookupByPlaceholder(phrases)
	return mapQuery(q, func(inner search.Query) search.Query {
		if tq, ok := inner.(*search.TermQuery); ok {
			text := tq.Term().Text()
			if span, ok := idx[text]; ok {
				return span
			}
		}
		return inner
	})
}

// buildSpan converts a wildcard-bearing phrase into a SpanNearQuery.
func (p *ComplexPhraseQueryParser) buildSpan(field, phrase string, slop int) search.Query {
	tokens := strings.Fields(phrase)
	clauses := make([]search.SpanQuery, 0, len(tokens))
	for _, tok := range tokens {
		clauses = append(clauses, spanClauseForToken(field, tok))
	}
	if len(clauses) == 0 {
		return search.NewMatchNoDocsQuery()
	}
	if len(clauses) == 1 {
		return clauses[0]
	}
	return search.NewSpanNearQuery(clauses, slop, p.InOrder)
}

// spanClauseForToken returns the SpanQuery clause appropriate for a single
// token. Tokens containing '*' or '?' use SpanMultiTermQueryWrapper around the
// corresponding multi-term query; plain tokens use SpanTermQuery.
func spanClauseForToken(field, tok string) search.SpanQuery {
	if strings.ContainsAny(tok, "*?") {
		mt := search.NewMultiTermQuery(field, index.NewTerm(field, tok))
		return search.NewSpanMultiTermQueryWrapper(mt)
	}
	return search.NewSpanTermQuery(index.NewTerm(field, tok))
}

func containsWildcard(s string) bool {
	return strings.ContainsAny(s, "*?")
}

func lookupByPlaceholder(phrases []*complexPhrase) map[string]search.Query {
	m := make(map[string]search.Query, len(phrases))
	for _, p := range phrases {
		m[p.Placeholder] = p.Span
	}
	return m
}

// mapQuery walks q and replaces each leaf via fn, preserving BooleanQuery /
// BoostQuery structure. Unknown query types are returned untouched.
func mapQuery(q search.Query, fn func(search.Query) search.Query) search.Query {
	switch v := q.(type) {
	case *search.BooleanQuery:
		out := search.NewBooleanQuery()
		for _, c := range v.Clauses() {
			out.Add(mapQuery(c.Query, fn), c.Occur)
		}
		return out
	case *search.BoostQuery:
		return search.NewBoostQuery(mapQuery(v.Query(), fn), v.Boost())
	default:
		return fn(q)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	b := make([]byte, 0, 4)
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
