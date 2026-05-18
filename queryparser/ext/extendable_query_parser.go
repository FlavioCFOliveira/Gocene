package ext

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/queryparser"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ExtendableQueryParser is the classic QueryParser augmented with the
// ExtensionQuery dispatch mechanism. When it encounters a top-level
// "<extKey>:<value>" clause whose extKey is a registered extension, it
// delegates parsing to that extension; otherwise it falls back to the
// underlying classic parser.
type ExtendableQueryParser struct {
	*queryparser.QueryParser
	extensions *Extensions
}

// NewExtendableQueryParser builds an ExtendableQueryParser with the supplied
// extensions registry. If extensions is nil a fresh empty registry is used.
func NewExtendableQueryParser(defaultField string, analyzer *analysis.StandardAnalyzer, extensions *Extensions) *ExtendableQueryParser {
	if extensions == nil {
		extensions = NewExtensions()
	}
	return &ExtendableQueryParser{
		QueryParser: queryparser.NewQueryParser(defaultField, analyzer),
		extensions:  extensions,
	}
}

// GetExtensions exposes the underlying Extensions registry.
func (p *ExtendableQueryParser) GetExtensions() *Extensions { return p.extensions }

// GetExtensionFieldDelimiter returns the delimiter used by the registry.
func (p *ExtendableQueryParser) GetExtensionFieldDelimiter() rune {
	return p.extensions.GetExtensionFieldDelimiter()
}

// Parse parses the supplied query string. When the query is a single
// "<extKey>:<value>" clause and extKey is registered as an extension, the
// extension handles the clause. Otherwise the classic parser is used.
func (p *ExtendableQueryParser) Parse(query string) (search.Query, error) {
	if extKey, raw, ok := p.tryExtractTopLevelExtension(query); ok {
		ext := p.extensions.GetExtension(extKey)
		if ext != nil {
			return ext.Parse(NewExtensionQuery(p.QueryParser, extKey, raw))
		}
	}
	return p.QueryParser.Parse(query)
}

// tryExtractTopLevelExtension inspects the query for the simple shape
// "extKey:value" or "extKey:\"quoted value\"" and returns the parts when the
// extKey is non-empty and contains only legal characters. It returns false
// for any compound or nested query, leaving such cases to the classic parser.
func (p *ExtendableQueryParser) tryExtractTopLevelExtension(query string) (string, string, bool) {
	delim := string(p.extensions.GetExtensionFieldDelimiter())
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return "", "", false
	}
	idx := strings.Index(trimmed, delim)
	if idx <= 0 {
		return "", "", false
	}
	key := trimmed[:idx]
	value := trimmed[idx+len(delim):]
	for _, r := range key {
		if !isExtKeyChar(r) {
			return "", "", false
		}
	}
	if strings.ContainsAny(value, " \t") && !(strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) {
		return "", "", false
	}
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") && len(value) >= 2 {
		value = value[1 : len(value)-1]
	}
	return key, value, true
}

func isExtKeyChar(r rune) bool {
	if r == '_' || r == '-' || r == '.' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	if r >= 'a' && r <= 'z' {
		return true
	}
	return false
}
