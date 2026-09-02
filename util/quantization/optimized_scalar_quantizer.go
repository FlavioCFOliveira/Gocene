package quantization

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

var minimumMSEGrid = [][]float32{
	{-0.798, 0.798},
	{-1.493, 1.493},
	{-2.051, 2.051},
	{-2.514, 2.514},
	{-2.916, 2.916},
	{-3.278, 3.278},
	{-3.611, 3.611},
	{-3.922, 3.922},
}

const defaultLambda = 0.1
const defaultIters = 5

// QuantizationResult contains the lower and upper interval bounds, the additional correction and the quantized component sum.
type QuantizationResult struct {
	LowerInterval        float32
	UpperInterval        float32
	AdditionalCorrection float32
	QuantizedComponentSum int
}

// OptimizedScalarQuantizer is a scalar quantizer that optimizes the quantization intervals for a given vector.
type OptimizedScalarQuantizer struct {
	similarityFunction util.VectorSimilarityFunction
	lambda             float32
	iters              int
}

func NewOptimizedScalarQuantizer(similarityFunction util.VectorSimilarityFunction, lambda float32, iters int) *OptimizedScalarQuantizer {
	return &OptimizedScalarQuantizer{
		similarityFunction: similarityFunction,
		lambda:             lambda,
		iters:              iters,
	}
}

func NewDefaultOptimizedScalarQuantizer(similarityFunction util.VectorSimilarityFunction) *OptimizedScalarQuantizer {
	return NewOptimizedScalarQuantizer(similarityFunction, defaultLambda, defaultIters)
}

func (osq *OptimizedScalarQuantizer) MultiScalarQuantize(vector []float32, destinations [][]byte, bits []byte, centroid []float32) []QuantizationResult {
	// Cosine check (using VectorUtil.IsUnitVector)
	if osq.similarityFunction == util.CosineSim {
		if !util.IsUnitVector(vector) || !util.IsUnitVector(centroid) {
			panic("vectors must be unit vectors for cosine similarity")
		}
	}

	intervalScratch := make([]float32, 2)
	var vecMean, vecVar float64
	var norm2, centroidDot float32
	min, max := float32(math.MaxFloat32), float32(-math.MaxFloat32)

	for i := 0; i < len(vector); i++ {
		if osq.similarityFunction != util.EuclideanSim {
			centroidDot += vector[i] * centroid[i]
		}
		vector[i] = vector[i] - centroid[i]
		if vector[i] < min {
			min = vector[i]
		}
		if vector[i] > max {
			max = vector[i]
		}
		norm2 += vector[i] * vector[i]
		delta := float64(vector[i]) - vecMean
		vecMean += delta / float64(i+1)
		vecVar += delta * (float64(vector[i]) - vecMean)
	}
	vecVar /= float64(len(vector))
	vecStd := math.Sqrt(vecVar)

	results := make([]QuantizationResult, len(bits))
	for i := 0; i < len(bits); i++ {
		b := bits[i]
		points := 1 << b

		intervalScratch[0] = clamp(float32(minimumMSEGrid[b-1][0]*float32(vecStd)+float32(vecMean)), min, max)
		intervalScratch[1] = clamp(float32(minimumMSEGrid[b-1][1]*float32(vecStd)+float32(vecMean)), min, max)

		osq.optimizeIntervals(intervalScratch, vector, norm2, points)

		nSteps := float32((1 << b) - 1)
		a, bBound := intervalScratch[0], intervalScratch[1]
		step := (bBound - a) / nSteps

		sumQuery := 0
		for h := 0; h < len(vector); h++ {
			xi := clamp(vector[h], a, bBound)
			assignment := int(math.Round(float64((xi - a) / step)))
			sumQuery += assignment
			destinations[i][h] = byte(assignment)
		}

		var addCorr float32
		if osq.similarityFunction == util.Euclidean {
			addCorr = norm2
		} else {
			addCorr = centroidDot
		}

		results[i] = QuantizationResult{
			LowerInterval:         intervalScratch[0],
			UpperInterval:         intervalScratch[1],
			AdditionalCorrection:  addCorr,
			QuantizedComponentSum: sumQuery,
		}
	}
	return results
}

func (osq *OptimizedScalarQuantizer) ScalarQuantize(vector []float32, destination []byte, bits byte, centroid []float32) QuantizationResult {
	if osq.similarityFunction == util.CosineSim {
		if !util.IsUnitVector(vector) || !util.IsUnitVector(centroid) {
			panic("vectors must be unit vectors for cosine similarity")
		}
	}

	intervalScratch := make([]float32, 2)
	points := 1 << bits
	var vecMean, vecVar float64
	var norm2, centroidDot float32
	min, max := float32(math.MaxFloat32), float32(-math.MaxFloat32)

	for i := 0; i < len(vector); i++ {
		if osq.similarityFunction != util.EuclideanSim {
			centroidDot += vector[i] * centroid[i]
		}
		vector[i] = vector[i] - centroid[i]
		if vector[i] < min {
			min = vector[i]
		}
		if vector[i] > max {
			max = vector[i]
		}
		norm2 += vector[i] * vector[i]
		delta := float64(vector[i]) - vecMean
		vecMean += delta / float64(i+1)
		vecVar += delta * (float64(vector[i]) - vecMean)
	}
	vecVar /= float64(len(vector))
	vecStd := math.Sqrt(vecVar)

	intervalScratch[0] = clamp(float32(minimumMSEGrid[bits-1][0]*float32(vecStd)+float32(vecMean)), min, max)
	intervalScratch[1] = clamp(float32(minimumMSEGrid[bits-1][1]*float32(vecStd)+float32(vecMean)), min, max)

	osq.optimizeIntervals(intervalScratch, vector, norm2, points)

	nSteps := float32((1 << bits) - 1)
	a, bBound := intervalScratch[0], intervalScratch[1]
	step := (bBound - a) / nSteps

	sumQuery := 0
	for h := 0; h < len(vector); h++ {
		xi := clamp(vector[h], a, bBound)
		assignment := int(math.Round(float64((xi - a) / step)))
		sumQuery += assignment
		destination[h] = byte(assignment)
	}

	var addCorr float32
	if osq.similarityFunction == util.Euclidean {
		addCorr = norm2
	} else {
		addCorr = centroidDot
	}

	return QuantizationResult{
		LowerInterval:         intervalScratch[0],
		UpperInterval:         intervalScratch[1],
		AdditionalCorrection:  addCorr,
		QuantizedComponentSum: sumQuery,
	}
}

func DeQuantize(quantized []byte, dequantized []float32, bits byte, lowerInterval, upperInterval float32, centroid []float32) []float32 {
	nSteps := (1 << bits) - 1
	step := float64(upperInterval - lowerInterval) / float64(nSteps)
	for h := 0; h < len(quantized); h++ {
		xi := float64(quantized[h]&0xFF)*step + float64(lowerInterval)
		dequantized[h] = float32(xi + float64(centroid[h]))
	}
	return dequantized
}

func (osq *OptimizedScalarQuantizer) loss(vector []float32, interval []float32, points int, norm2 float32) float64 {
	a := float64(interval[0])
	b := float64(interval[1])
	step := (b - a) / float64(points-1)
	stepInv := 1.0 / step
	var xe, e float64
	for _, xi := range vector {
		xiVal := float64(xi)
		xiq := a + step*math.Round((clampF64(xiVal, a, b)-a)*stepInv)
		xe += xiVal * (xiVal - xiq)
		e += (xiVal - xiq) * (xiVal - xiq)
	}
	return (1.0-float64(osq.lambda))*xe*xe/float64(norm2) + float64(osq.lambda)*e
}

func (osq *OptimizedScalarQuantizer) optimizeIntervals(initInterval []float32, vector []float32, norm2 float32, points int) {
	initialLoss := osq.loss(vector, initInterval, points, norm2)
	scale := (1.0 - osq.lambda) / norm2
	if math.IsInf(float64(scale), 0) || math.IsNaN(float64(scale)) {
		return
	}
	for i := 0; i < osq.iters; i++ {
		a := initInterval[0]
		b := initInterval[1]
		stepInv := float32(points-1) / (b - a)
		var daa, dab, dbb, dax, dbx float64
		for _, xi := range vector {
			k := float32(math.Round(float64((clamp(xi, a, b)-a)*stepInv)))
			s := float64(k / float32(points-1))
			daa += (1.0 - s) * (1.0 - s)
			dab += (1.0 - s) * s
			dbb += s * s
			dax += float64(xi) * (1.0 - s)
			dbx += float64(xi) * s
		}
		m0 := float64(scale)*dax*dax + float64(osq.lambda)*daa
		m1 := float64(scale)*dax*dbx + float64(osq.lambda)*dab
		m2 := float64(scale)*dbx*dbx + float64(osq.lambda)*dbb
		det := m0*m2 - m1*m1
		if det == 0 {
			return
		}
		aOpt := float32((m2*dax - m1*dbx) / det)
		bOpt := float32((m0*dbx - m1*dax) / det)
		if math.Abs(float64(initInterval[0]-aOpt)) < 1e-8 && math.Abs(float64(initInterval[1]-bOpt)) < 1e-8 {
			return
		}
		newLoss := osq.loss(vector, []float32{aOpt, bOpt}, points, norm2)
		if newLoss > initialLoss {
			return
		}
		initInterval[0] = aOpt
		initInterval[1] = bOpt
		initialLoss = newLoss
	}
}

func Discretize(value, bucket int) int {
	return ((value + (bucket - 1)) / bucket) * bucket
}

func TransposeHalfByte(q []byte, quantQueryByte []byte) {
	for i := 0; i < len(q); {
		lowerByte := 0
		lowerMiddleByte := 0
		upperMiddleByte := 0
		upperByte := 0
		for j := 7; j >= 0 && i < len(q); j-- {
			lowerByte |= (int(q[i]) & 1) << j
			lowerMiddleByte |= ((int(q[i]) >> 1) & 1) << j
			upperMiddleByte |= ((int(q[i]) >> 2) & 1) << j
			upperByte |= ((int(q[i]) >> 3) & 1) << j
			i++
		}
		index := ((i + 7) / 8) - 1
		quantQueryByte[index] = byte(lowerByte)
		quantQueryByte[index+len(quantQueryByte)/4] = byte(lowerMiddleByte)
		quantQueryByte[index+len(quantQueryByte)/2] = byte(upperMiddleByte)
		quantQueryByte[index+3*len(quantQueryByte)/4] = byte(upperByte)
	}
}

func PackAsBinary(vector []byte, packed []byte) {
	for i := 0; i < len(vector); {
		var result byte = 0
		for j := 7; j >= 0 && i < len(vector); j-- {
			result |= (vector[i] & 1) << j
			i++
		}
		index := ((i + 7) / 8) - 1
		packed[index] = result
	}
}

func UnpackBinary(packed []byte, vector []byte) {
	vectorIndex := 0
	for packedIndex := 0; packedIndex < len(packed) && vectorIndex < len(vector); packedIndex++ {
		packedByte := packed[packedIndex]
		for j := 7; j >= 0 && vectorIndex < len(vector); j-- {
			vector[vectorIndex] = (packedByte >> j) & 1
			vectorIndex++
		}
	}
}

func TransposeDibit(vector []byte, packed []byte) {
	limit := len(vector) - 7
	i := 0
	index := 0
	for ; i < limit; i += 8 {
		index++
		lowerByte := (vector[i] & 1) << 7 |
			(vector[i+1] & 1) << 6 |
			(vector[i+2] & 1) << 5 |
			(vector[i+3] & 1) << 4 |
			(vector[i+4] & 1) << 3 |
			(vector[i+5] & 1) << 2 |
			(vector[i+6] & 1) << 1 |
			(vector[i+7] & 1)
		upperByte := ((vector[i] >> 1) & 1) << 7 |
			((vector[i+1] >> 1) & 1) << 6 |
			((vector[i+2] >> 1) & 1) << 5 |
			((vector[i+3] >> 1) & 1) << 4 |
			((vector[i+4] >> 1) & 1) << 3 |
			((vector[i+5] >> 1) & 1) << 2 |
			((vector[i+6] >> 1) & 1) << 1 |
			((vector[i+7] >> 1) & 1)
		packed[index] = byte(lowerByte)
		packed[index+len(packed)/2] = byte(upperByte)
	}
	if i == len(vector) {
		return
	}
	var lowerByte, upperByte int
	for j := 7; i < len(vector); j-- {
		lowerByte |= int(vector[i]&1) << j
		upperByte |= int((vector[i]>>1)&1) << j
	}
	packed[index] = byte(lowerByte)
	packed[index+len(packed)/2] = byte(upperByte)
}

func UntransposeDibit(packed []byte, vector []byte) {
	stripeSize := len(packed) / 2
	limit := len(vector) - 7
	i := 0
	index := 0
	for ; i < limit; i += 8 {
		index++
		lowerByte := packed[index]
		upperByte := packed[index+stripeSize]
		vector[i] = byte(((lowerByte >> 7) & 1) | ((upperByte >> 7) & 1) << 1)
		vector[i+1] = byte(((lowerByte >> 6) & 1) | ((upperByte >> 6) & 1) << 1)
		vector[i+2] = byte(((lowerByte >> 5) & 1) | ((upperByte >> 5) & 1) << 1)
		vector[i+3] = byte(((lowerByte >> 4) & 1) | ((upperByte >> 4) & 1) << 1)
		vector[i+4] = byte(((lowerByte >> 3) & 1) | ((upperByte >> 3) & 1) << 1)
		vector[i+5] = byte(((lowerByte >> 2) & 1) | ((upperByte >> 2) & 1) << 1)
		vector[i+6] = byte(((lowerByte >> 1) & 1) | ((upperByte >> 1) & 1) << 1)
		vector[i+7] = byte((lowerByte & 1) | ((upperByte & 1) << 1))
	}
	if i < len(vector) {
		lowerByte := packed[index]
		upperByte := packed[index+stripeSize]
		for j := 7; i < len(vector); j-- {
			vector[i] = byte(((lowerByte >> j) & 1) | ((upperByte >> j) & 1) << 1)
		}
	}
}

func clamp(x, a, b float32) float32 {
	if x < a {
		return a
	}
	if x > b {
		return b
	}
	return x
}

func clampF64(x, a, b float64) float64 {
	if x < a {
		return a
	}
	if x > b {
		return b
	}
	return x
}
