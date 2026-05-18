package ext

import "github.com/FlavioCFOliveira/Gocene/search"

// ParserExtension is the contract every query parser extension must satisfy.
// It mirrors the abstract org.apache.lucene.queryparser.ext.ParserExtension
// class: the framework hands the extension an ExtensionQuery, and the
// extension is responsible for producing a search.Query (or an error).
type ParserExtension interface {
	Parse(query *ExtensionQuery) (search.Query, error)
}

// ParserExtensionFunc adapts a plain function into a ParserExtension so callers
// can register lightweight extensions without defining a new type.
type ParserExtensionFunc func(query *ExtensionQuery) (search.Query, error)

// Parse satisfies the ParserExtension interface for ParserExtensionFunc.
func (f ParserExtensionFunc) Parse(query *ExtensionQuery) (search.Query, error) {
	return f(query)
}
