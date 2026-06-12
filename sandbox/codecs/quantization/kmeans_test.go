package quantization

import (
	"math/rand"
	"testing"
)

// floatVectorValues is a test-only implementation of FloatVectorValues.
type floatVectorValues struct {
	vectors [][]float32
	dim     int
}

func (f *floatVectorValues) Size() int           { return len(f.vectors) }
func (f *floatVectorValues) Dimension() int       { return f.dim }
func (f *floatVectorValues) VectorValue(ord int) ([]float32, error) {
	return f.vectors[ord], nil
}

// generateData creates deterministic random float vectors clustered around
// nClusters random centroids, matching the pattern used by the Java
// TestKMeans.generateData helper.
func generateData(nSamples, nDims, nClusters int, rnd *rand.Rand) *floatVectorValues {
	vectors := make([][]float32, nSamples)
	centroids := make([][]float32, nClusters)
	for i := 0; i < nClusters; i++ {
		centroids[i] = make([]float32, nDims)
		for j := 0; j < nDims; j++ {
			centroids[i][j] = rnd.Float32() * 100
		}
	}
	for i := 0; i < nSamples; i++ {
		cluster := rnd.Intn(nClusters)
		vector := make([]float32, nDims)
		for j := 0; j < nDims; j++ {
			vector[j] = centroids[cluster][j] + rnd.Float32()*10 - 5
		}
		vectors[i] = vector
	}
	return &floatVectorValues{vectors: vectors, dim: nDims}
}

func TestKMeansAPI(t *testing.T) {
	rnd := rand.New(rand.NewSource(12345))

	nClusters := rnd.Intn(10) + 1 // 1..10
	nVectors := nClusters*100 + rnd.Intn(nClusters*100+1)
	dims := rnd.Intn(19) + 2 // 2..20

	vectors := generateData(nVectors, dims, nClusters, rnd)

	// Default case.
	{
		results, err := Cluster(vectors, nClusters)
		if err != nil {
			t.Fatalf("Cluster default error: %v", err)
		}
		if results == nil {
			t.Fatal("expected non-nil Results")
		}
		if len(results.Centroids) != nClusters {
			t.Fatalf("expected %d centroids, got %d", nClusters, len(results.Centroids))
		}
		if len(results.VectorCentroids) != nVectors {
			t.Fatalf("expected %d vectorCentroids, got %d", nVectors, len(results.VectorCentroids))
		}
	}

	// Expert case.
	{
		assignCentroidsToVectors := rnd.Intn(2) == 0
		initializationMethod := KmeansInitializationMethod(rnd.Intn(3))
		restarts := rnd.Intn(5) + 1
		iters := rnd.Intn(10) + 1
		sampleSize := rnd.Intn(nVectors*2-10) + 10

		results, err := ClusterExpert(
			vectors,
			nClusters,
			assignCentroidsToVectors,
			rnd.Int63(),
			initializationMethod,
			false, // normalizeCenters
			restarts,
			iters,
			sampleSize,
		)
		if err != nil {
			t.Fatalf("ClusterExpert error: %v", err)
		}
		if results == nil {
			t.Fatal("expected non-nil Results")
		}
		if len(results.Centroids) != nClusters {
			t.Fatalf("expected %d centroids, got %d", nClusters, len(results.Centroids))
		}
		if assignCentroidsToVectors {
			if len(results.VectorCentroids) != nVectors {
				t.Fatalf("expected %d vectorCentroids, got %d", nVectors, len(results.VectorCentroids))
			}
		} else {
			if results.VectorCentroids != nil {
				t.Fatalf("expected nil vectorCentroids, got %v", results.VectorCentroids)
			}
		}
	}
}

func TestKMeansSpecialCases(t *testing.T) {
	// nClusters > nVectors.
	{
		rnd := rand.New(rand.NewSource(42))
		nClusters := 20
		nVectors := 10
		vectors := generateData(nVectors, 5, nClusters, rnd)
		results, err := Cluster(vectors, nClusters)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Java adjusts numClusters down to 1 because sampleSize is clamped
		// to vectors.size() and maxNumClusters becomes max(1, 10/100)=1.
		if len(results.Centroids) != 1 {
			t.Fatalf("expected 1 centroid, got %d", len(results.Centroids))
		}
		if len(results.VectorCentroids) != nVectors {
			t.Fatalf("expected %d vectorCentroids, got %d", nVectors, len(results.VectorCentroids))
		}
	}

	// Small sample size.
	{
		rnd := rand.New(rand.NewSource(43))
		sampleSize := 2
		nClusters := 2
		nVectors := 300
		vectors := generateData(nVectors, 5, nClusters, rnd)
		results, err := ClusterExpert(
			vectors,
			nClusters,
			true,
			rnd.Int63(),
			PlusPlus,
			false,
			1,
			2,
			sampleSize,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results.Centroids) != nClusters {
			t.Fatalf("expected %d centroids, got %d", nClusters, len(results.Centroids))
		}
		if len(results.VectorCentroids) != nVectors {
			t.Fatalf("expected %d vectorCentroids, got %d", nVectors, len(results.VectorCentroids))
		}
	}

	// Unassigned centroids.
	{
		rnd := rand.New(rand.NewSource(44))
		nClusters := 4
		nVectors := 400
		vectors := generateData(nVectors, 5, nClusters, rnd)
		results, err := Cluster(vectors, nClusters)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		centroids := results.Centroids
		if len(centroids) != nClusters {
			t.Fatalf("expected %d centroids, got %d", nClusters, len(centroids))
		}

		unassignedIdxs := []int{0, 3}
		AssignCentroids(vectors, centroids, unassignedIdxs)

		if len(centroids) != nClusters {
			t.Fatalf("expected %d centroids after assign, got %d", nClusters, len(centroids))
		}
		// Verify that the unassigned centroids now have non-nil slices.
		for _, idx := range unassignedIdxs {
			if centroids[idx] == nil {
				t.Fatalf("centroid %d is nil after AssignCentroids", idx)
			}
			if len(centroids[idx]) != 5 {
				t.Fatalf("centroid %d has wrong dimension: %d", idx, len(centroids[idx]))
			}
		}
	}
}

func TestReservoirSample(t *testing.T) {
	// n <= k should return 0..n-1.
	{
		samples := ReservoirSample(5, 10, 1)
		if len(samples) != 5 {
			t.Fatalf("expected 5 samples, got %d", len(samples))
		}
		for i, v := range samples {
			if v != i {
				t.Fatalf("expected sample[%d]=%d, got %d", i, i, v)
			}
		}
	}

	// n > k should return k distinct samples in [0, n).
	{
		samples := ReservoirSample(100, 10, 2)
		if len(samples) != 10 {
			t.Fatalf("expected 10 samples, got %d", len(samples))
		}
		seen := make(map[int]struct{}, 10)
		for _, v := range samples {
			if v < 0 || v >= 100 {
				t.Fatalf("sample out of range: %d", v)
			}
			if _, ok := seen[v]; ok {
				t.Fatalf("duplicate sample: %d", v)
			}
			seen[v] = struct{}{}
		}
	}
}

func TestSampleReader(t *testing.T) {
	rnd := rand.New(rand.NewSource(99))
	nVectors := 50
	dims := 8
	vectors := generateData(nVectors, dims, 5, rnd)

	reader := CreateSampleReader(vectors, 10, 123)
	if reader.Size() != 10 {
		t.Fatalf("expected size 10, got %d", reader.Size())
	}
	if reader.Dimension() != dims {
		t.Fatalf("expected dimension %d, got %d", dims, reader.Dimension())
	}
	for i := 0; i < reader.Size(); i++ {
		vec, err := reader.VectorValue(i)
		if err != nil {
			t.Fatalf("VectorValue error: %v", err)
		}
		if len(vec) != dims {
			t.Fatalf("expected vector dim %d, got %d", dims, len(vec))
		}
	}
}
