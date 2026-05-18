package xml

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// CoreParser is the central QueryBuilderFactory shipped with Lucene's XML
// query parser. It wires the standard set of builders (registered by the
// builders package) and parses an XML document into a search.Query. Mirrors
// org.apache.lucene.queryparser.xml.CoreParser.
type CoreParser struct {
	*QueryBuilderFactory
	DefaultField string
}

// NewCoreParser builds an empty CoreParser; callers (or the builders package)
// register builders on the embedded QueryBuilderFactory before invoking Parse.
func NewCoreParser(defaultField string) *CoreParser {
	return &CoreParser{
		QueryBuilderFactory: NewQueryBuilderFactory(),
		DefaultField:        defaultField,
	}
}

// Parse reads the XML document from r and dispatches the root element to the
// registered QueryBuilders.
func (p *CoreParser) Parse(r io.Reader) (search.Query, error) {
	root, err := ParseDocument(r)
	if err != nil {
		return nil, err
	}
	return p.GetQuery(root)
}

// CorePlusQueriesParser is a CoreParser preconfigured with the optional
// queries module builders (LikeThis, FuzzyLikeThis, BoostingTerm). Mirrors
// org.apache.lucene.queryparser.xml.CorePlusQueriesParser.
type CorePlusQueriesParser struct{ *CoreParser }

// NewCorePlusQueriesParser builds a CorePlusQueriesParser. Wiring of the
// queries-module builders is performed by the builders package via
// RegisterCorePlusQueriesBuilders to avoid an import cycle.
func NewCorePlusQueriesParser(defaultField string) *CorePlusQueriesParser {
	return &CorePlusQueriesParser{CoreParser: NewCoreParser(defaultField)}
}

// CorePlusExtensionsParser extends CorePlusQueriesParser with builders for
// extension queries. Mirrors org.apache.lucene.queryparser.xml.CorePlusExtensionsParser.
type CorePlusExtensionsParser struct{ *CorePlusQueriesParser }

// NewCorePlusExtensionsParser builds a CorePlusExtensionsParser.
func NewCorePlusExtensionsParser(defaultField string) *CorePlusExtensionsParser {
	return &CorePlusExtensionsParser{CorePlusQueriesParser: NewCorePlusQueriesParser(defaultField)}
}
