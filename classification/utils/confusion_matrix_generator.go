// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package utils

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/FlavioCFOliveira/Gocene/classification"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// ConfusionMatrix holds the result of evaluating a classifier across a
// labelled dataset.
//
// Port of org.apache.lucene.classification.utils.ConfusionMatrixGenerator.ConfusionMatrix.
type ConfusionMatrix struct {
	// counts[expected][actual] → number of documents
	counts map[string]map[string]int64
	// linearizedMatrix is the flattened [expected][actual] counts table
	linearizedMatrix  [][]int64
	classes           []string
	avgClassCorrect   float64
	avgClassPrecision float64
}

// GetLinearizedMatrix returns the confusion matrix as a 2-D slice indexed
// [expected][actual].
func (m *ConfusionMatrix) GetLinearizedMatrix() [][]int64 { return m.linearizedMatrix }

// GetClasses returns the ordered list of class labels in the matrix.
func (m *ConfusionMatrix) GetClasses() []string { return m.classes }

// GetAvgClassCorrectness returns the average per-class correctness (recall).
func (m *ConfusionMatrix) GetAvgClassCorrectness() float64 { return m.avgClassCorrect }

// GetAvgClassPrecision returns the average per-class precision.
func (m *ConfusionMatrix) GetAvgClassPrecision() float64 { return m.avgClassPrecision }

// String returns a human-readable tabular representation.
func (m *ConfusionMatrix) String() string {
	if len(m.classes) == 0 {
		return "ConfusionMatrix{}"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("ConfusionMatrix{classes=%v, avgCorrectness=%.4f, avgPrecision=%.4f}",
		m.classes, m.avgClassCorrect, m.avgClassPrecision))
	return sb.String()
}

// GetConfusionMatrix evaluates a classifier on the given reader and returns
// the confusion matrix. For each document, the text field value is passed to
// the classifier and the predicted class is compared against the true class
// from the class field.
//
// The reader must support stored fields for both the class field and text
// field. A timeout context can be provided via timeoutMilliseconds; zero
// means no timeout.
func GetConfusionMatrix[T comparable](
	reader index.IndexReaderInterface,
	classifier classification.Classifier[T],
	classFieldName string,
	textFieldName string,
	timeoutMilliseconds int64,
) (*ConfusionMatrix, error) {
	// Set up timeout if requested.
	var ctx context.Context
	var cancel context.CancelFunc
	if timeoutMilliseconds > 0 {
		ctx, cancel = context.WithTimeout(context.Background(),
			time.Duration(timeoutMilliseconds)*time.Millisecond)
		defer cancel()
	}

	cm := &ConfusionMatrix{
		counts: make(map[string]map[string]int64),
	}

	leaves, err := reader.Leaves()
	if err != nil {
		return nil, fmt.Errorf("ConfusionMatrixGenerator: leaves: %w", err)
	}

	for _, leaf := range leaves {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
		}

		lr := leaf.Reader()
		stored, err := lr.StoredFields()
		if err != nil || stored == nil {
			continue
		}

		for docID := 0; docID < lr.MaxDoc(); docID++ {
			visitor := document.NewDocumentStoredFieldVisitor()
			if err := stored.Document(docID, visitor); err != nil {
				continue
			}
			doc := visitor.GetDocument()
			if doc == nil {
				continue
			}

			// Get true (expected) class.
			expectedField := doc.Get(classFieldName)
			if expectedField == nil {
				continue
			}
			expected := strings.TrimSpace(expectedField.StringValue())

			// Get text to classify.
			textField := doc.Get(textFieldName)
			if textField == nil {
				continue
			}
			text := strings.TrimSpace(textField.StringValue())

			// Classify.
			result, err := classifier.AssignClass(text)
			if err != nil || result == nil {
				continue
			}
			actual := fmt.Sprintf("%v", result.AssignedClass)

			// Record in confusion matrix.
			cm.increment(expected, actual)
		}
	}

	if cm.total() == 0 {
		return nil, fmt.Errorf("ConfusionMatrixGenerator: no documents with both class field %q and text field %q", classFieldName, textFieldName)
	}

	cm.buildLinearized()
	cm.computeAverages()
	return cm, nil
}

// increment records an expected→actual pair.
func (m *ConfusionMatrix) increment(expected, actual string) {
	if _, ok := m.counts[expected]; !ok {
		m.counts[expected] = make(map[string]int64)
	}
	m.counts[expected][actual]++
}

// total returns the total number of classified documents.
func (m *ConfusionMatrix) total() int64 {
	var t int64
	for _, row := range m.counts {
		for _, c := range row {
			t += c
		}
	}
	return t
}

// buildLinearized collects all unique class labels and builds the
// linearized [expected][actual] matrix.
func (m *ConfusionMatrix) buildLinearized() {
	set := make(map[string]bool)
	for k := range m.counts {
		set[k] = true
	}
	for _, row := range m.counts {
		for k := range row {
			set[k] = true
		}
	}
	m.classes = make([]string, 0, len(set))
	for c := range set {
		m.classes = append(m.classes, c)
	}
	sort.Strings(m.classes)

	n := len(m.classes)
	m.linearizedMatrix = make([][]int64, n)
	for i, exp := range m.classes {
		m.linearizedMatrix[i] = make([]int64, n)
		for j, act := range m.classes {
			m.linearizedMatrix[i][j] = m.counts[exp][act]
		}
	}
}

// computeAverages computes per-class correctness (recall) and precision.
func (m *ConfusionMatrix) computeAverages() {
	n := len(m.classes)
	if n == 0 {
		return
	}

	var sumCorrect, sumPrecision float64
	for i := range m.classes {
		// Correctness (recall) = TP / (TP + FN) = diagonal / row sum
		var rowSum int64
		for j := 0; j < n; j++ {
			rowSum += m.linearizedMatrix[i][j]
		}
		tp := m.linearizedMatrix[i][i]
		if rowSum > 0 {
			sumCorrect += float64(tp) / float64(rowSum)
		}

		// Precision = TP / (TP + FP) = diagonal / column sum
		var colSum int64
		for j := 0; j < n; j++ {
			colSum += m.linearizedMatrix[j][i]
		}
		if colSum > 0 {
			sumPrecision += float64(tp) / float64(colSum)
		}
	}

	m.avgClassCorrect = sumCorrect / float64(n)
	m.avgClassPrecision = sumPrecision / float64(n)
}

// Ensure ConfusionMatrixGenerator is non-nil (package-level function already
// provides the generator API).
var _ = GetConfusionMatrix[string]

