package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

type ConstantScoreQuery struct {
	query Query
	score float32
}

func NewConstantScoreQuery(query Query, score float32) *ConstantScoreQuery {
	return &ConstantScoreQuery{
		query: query,
		score: score,
	}
}

func (q *ConstantScoreQuery) Rewrite(reader IndexReader) (Query, error) {
	rewritten, err := q.query.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	return &ConstantScoreQuery{query: rewritten, score: q.score}, nil
}

func (q *ConstantScoreQuery) Clone() Query {
	return &ConstantScoreQuery{query: q.query.Clone(), score: q.score}
}

func (q *ConstantScoreQuery) Equals(other Query) bool {
	if otherQuery, ok := other.(*ConstantScoreQuery); ok {
		return q.query.Equals(otherQuery.query) && q.score == otherQuery.score
	}
	return false
}

func (q *ConstantScoreQuery) HashCode() int {
	return q.query.HashCode() ^ int(q.score)
}

func (q *ConstantScoreQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return NewConstantScoreWeight(q, searcher, scoreMode, boost), nil
}

func (q *ConstantScoreQuery) ToString(field string) string {
	return "ConstantScoreQuery(" + q.query.ToString(field) + ", " + fmt.Sprintf("%f", q.score) + ")"
}
