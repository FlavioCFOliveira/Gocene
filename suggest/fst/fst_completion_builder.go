package fst

// FSTCompletionBuilder fills an FSTCompletion from a Lookup-style input.
// Mirrors org.apache.lucene.search.suggest.fst.FSTCompletionBuilder.
type FSTCompletionBuilder struct {
	NumBuckets   int
	ExactFirst   bool
	sorter       BytesRefSorter
	weights      map[string]int64
	maxWeight    int64
}

// NewFSTCompletionBuilder builds the builder.
func NewFSTCompletionBuilder(numBuckets int, sorter BytesRefSorter, exactFirst bool) *FSTCompletionBuilder {
	if numBuckets < 1 {
		numBuckets = 10
	}
	if sorter == nil {
		sorter = NewInMemoryBytesRefSorter()
	}
	return &FSTCompletionBuilder{
		NumBuckets: numBuckets,
		ExactFirst: exactFirst,
		sorter:     sorter,
		weights:    make(map[string]int64),
	}
}

// Add records the (term, weight) tuple.
func (b *FSTCompletionBuilder) Add(term []byte, weight int64) error {
	if err := b.sorter.Add(term); err != nil {
		return err
	}
	key := string(term)
	if w, ok := b.weights[key]; ok {
		if weight > w {
			b.weights[key] = weight
		}
	} else {
		b.weights[key] = weight
	}
	if weight > b.maxWeight {
		b.maxWeight = weight
	}
	return nil
}

// Build returns the populated FSTCompletion.
func (b *FSTCompletionBuilder) Build() (*FSTCompletion, error) {
	completion := NewFSTCompletion(b.ExactFirst)
	terms, err := b.sorter.Iterate()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(terms))
	for _, t := range terms {
		key := string(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		w := b.weights[key]
		bucket := 0
		if b.maxWeight > 0 {
			bucket = int((float64(w) / float64(b.maxWeight)) * float64(b.NumBuckets-1))
		}
		completion.AddEntry(key, bucket)
	}
	completion.Finalize()
	return completion, nil
}
