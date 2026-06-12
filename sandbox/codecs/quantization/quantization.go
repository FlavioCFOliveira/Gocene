// Package quantization implements org.apache.lucene.sandbox.codecs.quantization.
package quantization

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// FloatVectorValues is the minimal ordinal-keyed float-vector surface
// consumed by KMeans and SampleReader. It mirrors the subset of
// org.apache.lucene.index.FloatVectorValues actually used by the sandbox
// quantization package.
type FloatVectorValues interface {
	Size() int
	Dimension() int
	VectorValue(ord int) ([]float32, error)
}

// KmeansInitializationMethod selects the centroid seeding strategy.
type KmeansInitializationMethod int

const (
	Forgy KmeansInitializationMethod = iota
	ReservoirSampling
	PlusPlus
)

const (
	// MaxNumCentroids is the maximum number of clusters KMeans supports.
	// Matches Java Short.MAX_VALUE.
	MaxNumCentroids = 32767
	// DefaultRestarts is the default number of KMeans restarts.
	DefaultRestarts = 5
	// DefaultIters is the default number of iterations per restart.
	DefaultIters = 10
	// DefaultSampleSize is the default number of vectors to sample.
	DefaultSampleSize = 100_000
)

// Results holds the output of KMeans clustering.
type Results struct {
	// Centroids is the produced centroids, one per cluster.
	Centroids [][]float32
	// VectorCentroids maps each vector ordinal to its nearest centroid.
	// It is nil when assignCentroidsToVectors was false.
	VectorCentroids []int16
}

// Cluster is the simplified entry point. It clusters vectors into
// numClusters clusters using the default hyper-parameters and always
// assigns centroids to vectors.
func Cluster(vectors FloatVectorValues, numClusters int) (*Results, error) {
	return ClusterExpert(
		vectors,
		numClusters,
		true,           // assignCentroidsToVectors
		42,             // seed
		PlusPlus,       // initializationMethod
		false,          // normalizeCenters
		DefaultRestarts,
		DefaultIters,
		DefaultSampleSize,
	)
}

// ClusterExpert is the full KMeans entry point.
func ClusterExpert(
	vectors FloatVectorValues,
	numClusters int,
	assignCentroidsToVectors bool,
	seed int64,
	initializationMethod KmeansInitializationMethod,
	normalizeCenters bool,
	restarts int,
	iters int,
	sampleSize int,
) (*Results, error) {
	if vectors.Size() == 0 {
		return nil, nil
	}
	if numClusters < 1 || numClusters > MaxNumCentroids {
		return nil, fmt.Errorf("[numClusters] must be between [1] and [%d]", MaxNumCentroids)
	}

	// Adjust sampleSize and numClusters as Java does.
	if sampleSize < 100*numClusters {
		sampleSize = 100 * numClusters
	}
	if sampleSize > vectors.Size() {
		sampleSize = vectors.Size()
		maxNumClusters := max(1, sampleSize/100)
		if numClusters > maxNumClusters {
			numClusters = maxNumClusters
		}
	}

	rnd := rand.New(rand.NewSource(seed))
	var centroids [][]float32
	if numClusters == 1 {
		centroids = make([][]float32, 1)
		centroids[0] = make([]float32, vectors.Dimension())
	} else {
		var sampleVectors FloatVectorValues = vectors
		if vectors.Size() > sampleSize {
			sampleVectors = CreateSampleReader(vectors, sampleSize, seed)
		}
		kmeans := &KMeans{
			vectors:              sampleVectors,
			numVectors:           sampleVectors.Size(),
			numCentroids:         numClusters,
			random:               rnd,
			initializationMethod: initializationMethod,
			restarts:             restarts,
			iters:                iters,
		}
		var err error
		centroids, err = kmeans.computeCentroids(normalizeCenters)
		if err != nil {
			return nil, err
		}
	}

	var vectorCentroids []int16
	if assignCentroidsToVectors {
		vectorCentroids = make([]int16, vectors.Size())
		_, err := runKMeansStep(vectors, centroids, vectorCentroids, true, normalizeCenters)
		if err != nil {
			return nil, err
		}
	}
	return &Results{Centroids: centroids, VectorCentroids: vectorCentroids}, nil
}

// KMeans holds the state for a single clustering run.
type KMeans struct {
	vectors              FloatVectorValues
	numVectors           int
	numCentroids         int
	random               *rand.Rand
	initializationMethod KmeansInitializationMethod
	restarts             int
	iters                int
}

func (k *KMeans) computeCentroids(normalizeCenters bool) ([][]float32, error) {
	vectorCentroids := make([]int16, k.numVectors)
	minSquaredDist := math.MaxFloat64
	var bestCentroids [][]float32

	for restart := 0; restart < k.restarts; restart++ {
		var centroids [][]float32
		switch k.initializationMethod {
		case Forgy:
			centroids = k.initializeForgy()
		case ReservoirSampling:
			centroids = k.initializeReservoirSampling()
		case PlusPlus:
			var err error
			centroids, err = k.initializePlusPlus()
			if err != nil {
				return nil, err
			}
		}

		prevSquaredDist := math.MaxFloat64
		var squaredDist float64
		for iter := 0; iter < k.iters; iter++ {
			var err error
			squaredDist, err = runKMeansStep(k.vectors, centroids, vectorCentroids, false, normalizeCenters)
			if err != nil {
				return nil, err
			}
			if prevSquaredDist <= squaredDist+1e-6 {
				break
			}
			prevSquaredDist = squaredDist
		}
		if squaredDist < minSquaredDist {
			minSquaredDist = squaredDist
			bestCentroids = centroids
		}
	}
	return bestCentroids, nil
}

// initializeForgy randomly selects numCentroids distinct vectors as initial
// centroids.
func (k *KMeans) initializeForgy() [][]float32 {
	selection := make(map[int]struct{}, k.numCentroids)
	for len(selection) < k.numCentroids {
		selection[k.random.Intn(k.numVectors)] = struct{}{}
	}
	initialCentroids := make([][]float32, k.numCentroids)
	i := 0
	for selectedIdx := range selection {
		vec, _ := k.vectors.VectorValue(selectedIdx)
		initialCentroids[i] = append([]float32(nil), vec...)
		i++
	}
	return initialCentroids
}

// initializeReservoirSampling selects initial centroids using reservoir
// sampling over the vectors.
func (k *KMeans) initializeReservoirSampling() [][]float32 {
	initialCentroids := make([][]float32, k.numCentroids)
	for index := 0; index < k.numVectors; index++ {
		vec, _ := k.vectors.VectorValue(index)
		if index < k.numCentroids {
			initialCentroids[index] = append([]float32(nil), vec...)
		} else if k.random.Float64() < float64(k.numCentroids)*(1.0/float64(index)) {
			c := k.random.Intn(k.numCentroids)
			initialCentroids[c] = append([]float32(nil), vec...)
		}
	}
	return initialCentroids
}

// initializePlusPlus selects initial centroids using the k-means++ method.
func (k *KMeans) initializePlusPlus() ([][]float32, error) {
	initialCentroids := make([][]float32, k.numCentroids)
	firstIndex := k.random.Intn(k.numVectors)
	value, err := k.vectors.VectorValue(firstIndex)
	if err != nil {
		return nil, err
	}
	initialCentroids[0] = append([]float32(nil), value...)

	minDistances := make([]float32, k.numVectors)
	for i := range minDistances {
		minDistances[i] = math.MaxFloat32
	}

	for i := 1; i < k.numCentroids; i++ {
		var totalSum float64
		for j := 0; j < k.numVectors; j++ {
			vec, err := k.vectors.VectorValue(j)
			if err != nil {
				return nil, err
			}
			dist := util.SquareDistance(vec, initialCentroids[i-1])
			if dist < minDistances[j] {
				minDistances[j] = dist
			}
			totalSum += float64(minDistances[j])
		}

		r := totalSum * k.random.Float64()
		var cumulativeSum float64
		nextCentroidIndex := -1
		for j := 0; j < k.numVectors; j++ {
			cumulativeSum += float64(minDistances[j])
			if cumulativeSum >= r && minDistances[j] > 0 {
				nextCentroidIndex = j
				break
			}
		}
		value, err = k.vectors.VectorValue(nextCentroidIndex)
		if err != nil {
			return nil, err
		}
		initialCentroids[i] = append([]float32(nil), value...)
	}
	return initialCentroids, nil
}

// runKMeansStep performs a single Lloyd iteration: assign each vector to
// its nearest centroid, recompute centroids as the mean of assigned vectors,
// and optionally normalise them. It returns the sum of squared distances.
func runKMeansStep(
	vectors FloatVectorValues,
	centroids [][]float32,
	docCentroids []int16,
	useKahanSummation bool,
	normalizeCentroids bool,
) (float64, error) {
	numCentroids := int16(len(centroids))
	dim := len(centroids[0])

	newCentroids := make([][]float32, numCentroids)
	for c := range newCentroids {
		newCentroids[c] = make([]float32, dim)
	}
	newCentroidSize := make([]int, numCentroids)

	var compensations [][]float32
	if useKahanSummation {
		compensations = make([][]float32, numCentroids)
		for c := range compensations {
			compensations[c] = make([]float32, dim)
		}
	}

	var sumSquaredDist float64
	numVectors := vectors.Size()
	for docID := 0; docID < numVectors; docID++ {
		vector, err := vectors.VectorValue(docID)
		if err != nil {
			return 0, err
		}
		bestCentroid := int16(0)
		if numCentroids > 1 {
			minSquaredDist := float32(math.MaxFloat32)
			for c := int16(0); c < numCentroids; c++ {
				squareDist := util.SquareDistance(centroids[c], vector)
				if squareDist < minSquaredDist {
					bestCentroid = c
					minSquaredDist = squareDist
				}
			}
			sumSquaredDist += float64(minSquaredDist)
		}

		newCentroidSize[bestCentroid]++
		for d := 0; d < dim; d++ {
			if useKahanSummation {
				y := vector[d] - compensations[bestCentroid][d]
				t := newCentroids[bestCentroid][d] + y
				compensations[bestCentroid][d] = (t - newCentroids[bestCentroid][d]) - y
				newCentroids[bestCentroid][d] = t
			} else {
				newCentroids[bestCentroid][d] += vector[d]
			}
		}
		docCentroids[docID] = bestCentroid
	}

	unassignedCentroids := make([]int, 0)
	for c := 0; c < int(numCentroids); c++ {
		if newCentroidSize[c] > 0 {
			for d := 0; d < dim; d++ {
				centroids[c][d] = newCentroids[c][d] / float32(newCentroidSize[c])
			}
		} else {
			unassignedCentroids = append(unassignedCentroids, c)
		}
	}
	if len(unassignedCentroids) > 0 {
		AssignCentroids(vectors, centroids, unassignedCentroids)
	}
	if normalizeCentroids {
		for c := range centroids {
			util.L2Normalize(centroids[c])
		}
	}
	return sumSquaredDist, nil
}

// AssignCentroids handles centroids that received no assigned vectors by
// copying outlying vectors to them. The outlying vectors are chosen by
// descending distance to the current centroid set.
func AssignCentroids(vectors FloatVectorValues, centroids [][]float32, unassignedCentroidsIdxs []int) {
	assignedCentroidsIdxs := make([]int, 0, len(centroids)-len(unassignedCentroidsIdxs))
	for i := range centroids {
		found := false
		for _, u := range unassignedCentroidsIdxs {
			if u == i {
				found = true
				break
			}
		}
		if !found {
			assignedCentroidsIdxs = append(assignedCentroidsIdxs, i)
		}
	}

	queue := hnsw.NewNeighborQueue(len(unassignedCentroidsIdxs), false)
	numVectors := vectors.Size()
	for i := 0; i < numVectors; i++ {
		vector, _ := vectors.VectorValue(i)
		for _, idx := range assignedCentroidsIdxs {
			squareDist := util.SquareDistance(centroids[idx], vector)
			queue.InsertWithOverflow(int32(i), squareDist)
		}
	}

	for i := 0; i < len(unassignedCentroidsIdxs); i++ {
		vector, _ := vectors.VectorValue(int(queue.TopNode()))
		unassignedCentroidIdx := unassignedCentroidsIdxs[i]
		centroids[unassignedCentroidIdx] = append([]float32(nil), vector...)
		queue.Pop()
	}
}

// SampleReader exposes a uniformly-sampled subset of an underlying
// FloatVectorValues. It mirrors org.apache.lucene.sandbox.codecs.quantization.SampleReader.
type SampleReader struct {
	origin      FloatVectorValues
	sampleSize  int
	sampleFunc  func(int) int
}

// CreateSampleReader builds a SampleReader that returns a random sample of
// at most sampleSize vectors from origin, selected by reservoir sampling.
func CreateSampleReader(origin FloatVectorValues, sampleSize int, seed int64) *SampleReader {
	samples := ReservoirSample(origin.Size(), sampleSize, seed)
	return &SampleReader{
		origin:     origin,
		sampleSize: len(samples),
		sampleFunc: func(i int) int { return samples[i] },
	}
}

// ReservoirSample selects k elements from n elements using the reservoir
// sampling algorithm. When n <= k it returns a slice of length n containing
// indices 0..n-1.
func ReservoirSample(n, k int, seed int64) []int {
	if n <= 0 {
		return nil
	}
	if k <= 0 {
		return nil
	}
	if n <= k {
		reservoir := make([]int, n)
		for i := 0; i < n; i++ {
			reservoir[i] = i
		}
		return reservoir
	}

	rnd := rand.New(rand.NewSource(seed))
	reservoir := make([]int, k)
	for i := 0; i < k; i++ {
		reservoir[i] = i
	}
	for i := k; i < n; i++ {
		j := rnd.Intn(i + 1)
		if j < k {
			reservoir[j] = i
		}
	}
	return reservoir
}

// Size returns the sample size.
func (r *SampleReader) Size() int { return r.sampleSize }

// Dimension returns the dimension of the underlying vectors.
func (r *SampleReader) Dimension() int { return r.origin.Dimension() }

// VectorValue returns the float vector for the given sampled ordinal.
func (r *SampleReader) VectorValue(ord int) ([]float32, error) {
	return r.origin.VectorValue(r.sampleFunc(ord))
}
