package search

// Occur defines the occurrence of a clause in a BooleanQuery.
type Occur int

const (
	MUST Occur = iota
	SHOULD
	MUST_NOT
	FILTER
)

func (o Occur) String() string {
	switch o {
	case MUST:
		return "+"
	case SHOULD:
		return ""
	case MUST_NOT:
		return "-"
	case FILTER:
		return "#"
	default:
		return "UNKNOWN"
	}
}

// BooleanClause is a query combined with an occurrence.
type BooleanClause struct {
	query Query
	occur Occur
}

func NewBooleanClause(query Query, occur Occur) *BooleanClause {
	if query == nil {
		panic("Query must not be null")
	}
	return &BooleanClause{
		query: query,
		occur: occur,
	}
}

func (c *BooleanClause) Query() Query {
	return c.query
}

func (c *BooleanClause) Occur() Occur {
	return c.occur
}

func (c *BooleanClause) IsRequired() bool {
	return c.occur == MUST || c.occur == FILTER
}

func (c *BooleanClause) IsProhibited() bool {
	return c.occur == MUST_NOT
}

func (c *BooleanClause) IsScoring() bool {
	return c.occur == MUST || c.occur == SHOULD
}
