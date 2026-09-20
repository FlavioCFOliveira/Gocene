package spi

// KnnSearchStrategy is the search strategy used by KnnCollector.
type KnnSearchStrategy interface {
	// NextVectorsBlock signals that the searcher has moved to the next block of vectors.
	NextVectorsBlock()
}

// KnnCollector is a knn collector used for gathering kNN results.
type KnnCollector interface {
	// EarlyTerminated returns true if search visits too many documents and terminates early.
	EarlyTerminated() bool

	// IncVisitedCount increments the visited vector count.
	IncVisitedCount(count int)

	// VisitedCount returns the current visited vector count.
	VisitedCount() int64

	// VisitLimit returns the visited vector limit.
	VisitLimit() int64

	// K returns the expected number of collected results.
	K() int

	// Collect collects the provided docId and include in the result set.
	Collect(docID int, similarity float32) bool

	// MinCompetitiveSimilarity returns the current minimum competitive similarity.
	MinCompetitiveSimilarity() float32

	// TopDocs drains the collected nearest kNN results and returns them.
	TopDocs() *TopDocs

	// GetSearchStrategy returns the search strategy used by this collector.
	GetSearchStrategy() KnnSearchStrategy
}

// KnnCollectorDecorator is the base class for decorators of KnnCollector objects.
type KnnCollectorDecorator struct {
	collector KnnCollector
}

func (d *KnnCollectorDecorator) EarlyTerminated() bool {
	return d.collector.EarlyTerminated()
}

func (d *KnnCollectorDecorator) IncVisitedCount(count int) {
	d.collector.IncVisitedCount(count)
}

func (d *KnnCollectorDecorator) VisitedCount() int64 {
	return d.collector.VisitedCount()
}

func (d *KnnCollectorDecorator) VisitLimit() int64 {
	return d.collector.VisitLimit()
}

func (d *KnnCollectorDecorator) K() int {
	return d.collector.K()
}

func (d *KnnCollectorDecorator) Collect(docID int, similarity float32) bool {
	return d.collector.Collect(docID, similarity)
}

func (d *KnnCollectorDecorator) MinCompetitiveSimilarity() float32 {
	return d.collector.MinCompetitiveSimilarity()
}

func (d *KnnCollectorDecorator) TopDocs() *TopDocs {
	return d.collector.TopDocs()
}

func (d *KnnCollectorDecorator) GetSearchStrategy() KnnSearchStrategy {
	return d.collector.GetSearchStrategy()
}
