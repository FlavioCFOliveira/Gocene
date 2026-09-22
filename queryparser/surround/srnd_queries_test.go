package surround

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestSrndQuery_Boosts(t *testing.T) {
	factory := &BasicQueryFactory{}
	field := "text"

	tests := []struct {
		name     string
		node     SrndQuery
		weighted bool
		weight   float32
	}{
		{
			name:     "term unweighted",
			node:     NewSrndTermQuery("apple", false),
			weighted: false,
		},
		{
			name: "term weighted",
			node: func() SrndQuery {
				q := NewSrndTermQuery("apple", false)
				q.SetWeight(2.0)
				return q
			}(),
			weighted: true,
			weight:   2.0,
		},
		{
			name:     "prefix unweighted",
			node:     NewSrndPrefixQuery("app", false, '*'),
			weighted: false,
		},
		{
			name: "prefix weighted",
			node: func() SrndQuery {
				q := NewSrndPrefixQuery("app", false, '*')
				q.SetWeight(1.5)
				return q
			}(),
			weighted: true,
			weight:   1.5,
		},
		{
			name:     "trunc unweighted",
			node:     NewSrndTruncQuery("a?p*", '*', '?'),
			weighted: false,
		},
		{
			name: "trunc weighted",
			node: func() SrndQuery {
				q := NewSrndTruncQuery("a?p*", '*', '?')
				q.SetWeight(3.0)
				return q
			}(),
			weighted: true,
			weight:   3.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := tt.node.MakeLuceneQueryField(field, factory)
			if err != nil {
				t.Fatalf("MakeLuceneQueryField failed: %v", err)
			}

			if tt.weighted {
				bq, ok := q.(*search.BoostQuery)
				if !ok {
					t.Errorf("expected BoostQuery, got %T", q)
					return
				}
				if bq.Boost() != tt.weight {
					t.Errorf("expected boost %g, got %g", tt.weight, bq.Boost())
				}
			} else {
				if _, ok := q.(*search.BoostQuery); ok {
					t.Errorf("expected no BoostQuery, but got one")
				}
			}
		})
	}
}

func TestSrndQuery_String(t *testing.T) {
	tests := []struct {
		name     string
		node     SrndQuery
		expected string
	}{
		{
			name:     "term plain",
			node:     NewSrndTermQuery("apple", false),
			expected: "apple",
		},
		{
			name:     "term quoted",
			node:     NewSrndTermQuery("apple pie", true),
			expected: `"apple pie"`,
		},
		{
			name: "term weighted",
			node: func() SrndQuery {
				q := NewSrndTermQuery("apple", false)
				q.SetWeight(2.0)
				return q
			}(),
			expected: "apple^2",
		},
		{
			name:     "prefix plain",
			node:     NewSrndPrefixQuery("app", false, '*'),
			expected: "app*",
		},
		{
			name:     "prefix quoted",
			node:     NewSrndPrefixQuery("app", true, '*'),
			expected: `"app"*`,
		},
		{
			name: "prefix weighted",
			node: func() SrndQuery {
				q := NewSrndPrefixQuery("app", false, '*')
				q.SetWeight(1.5)
				return q
			}(),
			expected: "app*^1.5",
		},
		{
			name:     "trunc plain",
			node:     NewSrndTruncQuery("a?p*", '*', '?'),
			expected: "a?p*",
		},
		{
			name: "trunc weighted",
			node: func() SrndQuery {
				q := NewSrndTruncQuery("a?p*", '*', '?')
				q.SetWeight(3.0)
				return q
			}(),
			expected: "a?p*^3",
		},
		{
			name: "boolean",
			node: func() SrndQuery {
				q := NewSrndBooleanQuery([]SrndQuery{
					NewSrndTermQuery("apple", false),
					NewSrndTermQuery("banana", false),
				}, search.MUST)
				q.SetWeight(2.0)
				return q
			}(),
			expected: "(apple banana)^2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.node.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSrndBooleanQuery_Behavior(t *testing.T) {
	factory := &BasicQueryFactory{}
	field := "text"

	t.Run("boosted boolean", func(t *testing.T) {
		q := NewSrndBooleanQuery([]SrndQuery{
			NewSrndTermQuery("apple", false),
			NewSrndTermQuery("banana", false),
		}, search.MUST)
		q.SetWeight(2.0)

		luceneQ, err := q.MakeLuceneQueryField(field, factory)
		if err != nil {
			t.Fatalf("MakeLuceneQueryField failed: %v", err)
		}

		bq, ok := luceneQ.(*search.BoostQuery)
		if !ok {
			t.Fatalf("expected BoostQuery, got %T", luceneQ)
		}
		if bq.Boost() != 2.0 {
			t.Errorf("expected boost 2.0, got %g", bq.Boost())
		}

		inner, ok := bq.Query().(*search.BooleanQuery)
		if !ok {
			t.Fatalf("expected inner BooleanQuery, got %T", bq.Query)
		}
		if len(inner.Clauses()) != 2 {
			t.Errorf("expected 2 clauses, got %d", len(inner.Clauses()))
		}
	})

	t.Run("too few queries", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic for too few queries in MakeBooleanQuery")
			}
		}()
		MakeBooleanQuery([]search.Query{search.NewBooleanQueryBuilder().Build()}, search.MUST)
	})
}
