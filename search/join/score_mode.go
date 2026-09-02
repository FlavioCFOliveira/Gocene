package join

// ScoreMode defines how to aggregate multiple child hit scores into a single parent score.
type ScoreMode int

const (
	// None: Do no scoring.
	None ScoreMode = iota
	// Avg: Parent hit's score is the average of all child scores.
	Avg
	// Max: Parent hit's score is the max of all child scores.
	Max
	// Total: Parent hit's score is the sum of all child scores.
	Total
	// Min: Parent hit's score is the min of all child scores.
	Min
)
