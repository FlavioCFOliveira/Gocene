// Package quantization implements
// org.apache.lucene.sandbox.codecs.quantization.
package quantization

// KMeans is the K-means clusterer used to quantise vectors. Mirrors
// org.apache.lucene.sandbox.codecs.quantization.KMeans.
type KMeans struct {
	K        int
	MaxIters int
}

// NewKMeans builds the clusterer.
func NewKMeans(k, maxIters int) *KMeans {
	if k < 1 {
		k = 1
	}
	if maxIters < 1 {
		maxIters = 25
	}
	return &KMeans{K: k, MaxIters: maxIters}
}

// Fit clusters the supplied vectors and returns K centroids. The Go port
// runs a basic Lloyd's iteration; concrete production-grade variants live
// elsewhere.
func (k *KMeans) Fit(vectors [][]float32, dim int) [][]float32 {
	if len(vectors) == 0 || dim < 1 {
		return nil
	}
	if k.K > len(vectors) {
		k.K = len(vectors)
	}
	centroids := make([][]float32, k.K)
	for i := 0; i < k.K; i++ {
		centroids[i] = append([]float32(nil), vectors[i]...)
	}
	for iter := 0; iter < k.MaxIters; iter++ {
		sums := make([][]float64, k.K)
		counts := make([]int, k.K)
		for i := range sums {
			sums[i] = make([]float64, dim)
		}
		for _, v := range vectors {
			best, _ := nearest(v, centroids)
			counts[best]++
			for d := 0; d < dim; d++ {
				sums[best][d] += float64(v[d])
			}
		}
		for i := 0; i < k.K; i++ {
			if counts[i] == 0 {
				continue
			}
			for d := 0; d < dim; d++ {
				centroids[i][d] = float32(sums[i][d] / float64(counts[i]))
			}
		}
	}
	return centroids
}

func nearest(v []float32, centroids [][]float32) (int, float64) {
	best, bestDist := -1, 0.0
	for i, c := range centroids {
		d := dist(v, c)
		if best < 0 || d < bestDist {
			best, bestDist = i, d
		}
	}
	return best, bestDist
}

func dist(a, b []float32) float64 {
	var sum float64
	for i := range a {
		diff := float64(a[i] - b[i])
		sum += diff * diff
	}
	return sum
}

// SampleReader streams a uniformly-sampled subset of the supplied vectors.
// Mirrors org.apache.lucene.sandbox.codecs.quantization.SampleReader.
type SampleReader struct {
	Vectors    [][]float32
	SampleSize int
}

// NewSampleReader builds the reader.
func NewSampleReader(vectors [][]float32, sampleSize int) *SampleReader {
	if sampleSize < 1 {
		sampleSize = 1
	}
	if sampleSize > len(vectors) {
		sampleSize = len(vectors)
	}
	return &SampleReader{Vectors: vectors, SampleSize: sampleSize}
}

// Sample returns the first SampleSize vectors (deterministic order — the
// caller can shuffle the input before construction when they want a random
// sample).
func (r *SampleReader) Sample() [][]float32 {
	out := make([][]float32, r.SampleSize)
	for i := 0; i < r.SampleSize; i++ {
		out[i] = r.Vectors[i]
	}
	return out
}
