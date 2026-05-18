// Package ext provides an extensible classic query parser that lets callers
// register custom extension handlers keyed by an extension token.
//
// This is the Go port of org.apache.lucene.queryparser.ext.
package ext

import (
	"github.com/FlavioCFOliveira/Gocene/queryparser"
)

// ExtensionQuery is the data carrier passed to a ParserExtension when it is
// invoked. It mirrors org.apache.lucene.queryparser.ext.ExtensionQuery and
// captures the originating parser, the resolved field, and the raw query
// payload that should be parsed by the extension.
type ExtensionQuery struct {
	topLevelParser *queryparser.QueryParser
	field          string
	rawQueryString string
}

// NewExtensionQuery constructs an ExtensionQuery for the given parser, field,
// and raw query string.
func NewExtensionQuery(topLevelParser *queryparser.QueryParser, field, rawQueryString string) *ExtensionQuery {
	return &ExtensionQuery{
		topLevelParser: topLevelParser,
		field:          field,
		rawQueryString: rawQueryString,
	}
}

// GetField returns the field name associated with this extension query.
func (e *ExtensionQuery) GetField() string { return e.field }

// GetRawQueryString returns the raw query payload to be handled by the extension.
func (e *ExtensionQuery) GetRawQueryString() string { return e.rawQueryString }

// GetTopLevelParser returns the parser that triggered this extension call.
func (e *ExtensionQuery) GetTopLevelParser() *queryparser.QueryParser { return e.topLevelParser }
