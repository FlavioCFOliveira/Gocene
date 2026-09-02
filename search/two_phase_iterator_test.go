package search
import (
	"errors"
	"testing"
)
type mockDocIdSetIterator struct {
	docs   []int
	cursor int
}
func (m *mockDocIdSetIterator) DocID() int {
	if m.cursor < 0 {
		return -1
	}
	if m.cursor >= len(m.docs) {
		return NO_MORE_DOCS
	}
	return m.docs[m.cursor]
}
func (m *mockDocIdSetIterator) NextDoc() (int, error) {
	m.cursor++
	if m.cursor >= len(m.docs) {
		return NO_MORE_DOCS, nil
	}
	return m.docs[m.cursor], nil
}
func (m *mockDocIdSetIterator) Advance(target int) (int, error) {
	if m.cursor < 0 {
		m.cursor = 0
	}
	for m.cursor < len(m.docs) && m.docs[m.cursor] < target {
		m.cursor++
	}
	if m.cursor >= len(m.docs) {
		return NO_MORE_DOCS, nil
	}
	return m.docs[m.cursor], nil
}
func (m *mockDocIdSetIterator) DocIDRunEnd() int {
	return m.DocID() + 1
}
func (m *mockDocIdSetIterator) Cost() int64 {
	return 1
}
type mockTwoPhaseIterator struct {
	approx   DocIdSetIterator
	matches  map[int]bool
	matchCost float32
}
func (m *mockTwoPhaseIterator) Approximation() DocIdSetIterator {
	return m.approx
}
func (m *mockTwoPhaseIterator) Matches() (bool, error) {
	doc := m.approx.DocID()
	if doc == NO_MORE_DOCS || doc == -1 {
		return false, errors.New("iterator not positioned")
	}
	return m.matches[doc], nil
}
func (m *mockTwoPhaseIterator) MatchCost() float32 {
	return m.matchCost
}
func (m *mockTwoPhaseIterator) DocIDRunEnd() (int, error) {
	return DefaultDocIDRunEnd(m)
}
func (m *mockTwoPhaseIterator) IntoBitSet(upTo int, bitSet *FixedBitSet, offset int) error {
	return DefaultIntoBitSet(m, upTo, bitSet, offset)
}
func TestTwoPhaseIterator_AsDocIdSetIterator(t *testing.T) {
	docs := []int{10, 11, 12, 13, 14}
	matches := map[int]bool{
		10: true,
		11: false,
		12: true,
		13: false,
		14: true,
	}
	approx := &mockDocIdSetIterator{docs: docs, cursor: -1}
	tpi := &mockTwoPhaseIterator{
		approx:   approx,
		matches:  matches,
		matchCost: 1.0,
	}
	iter := AsDocIdSetIterator(tpi)
	if iter.DocID() != -1 {
		t.Errorf("expected DocID -1, got %d", iter.DocID())
	}
	doc, err := iter.NextDoc()
	if err != nil {
		t.Fatal(err)
	}
	if doc != 10 {
		t.Errorf("expected 10, got %d", doc)
	}
	doc, err = iter.NextDoc()
	if err != nil {
		t.Fatal(err)
	}
	if doc != 12 {
		t.Errorf("expected 12, got %d", doc)
	}
	doc, err = iter.NextDoc()
	if err != nil {
		t.Fatal(err)
	}
	if doc != 14 {
		t.Errorf("expected 14, got %d", doc)
	}
	doc, err = iter.NextDoc()
	if err != nil {
		t.Fatal(err)
	}
	if doc != NO_MORE_DOCS {
		t.Errorf("expected NO_MORE_DOCS, got %d", doc)
	}
}
func TestTwoPhaseIterator_Advance(t *testing.T) {
	docs := []int{10, 11, 12, 13, 14}
	matches := map[int]bool{
		10: true,
		11: false,
		12: true,
		13: false,
		14: true,
	}
	approx := &mockDocIdSetIterator{docs: docs, cursor: -1}
	tpi := &mockTwoPhaseIterator{
		approx:   approx,
		matches:  matches,
		matchCost: 1.0,
	}
	iter := AsDocIdSetIterator(tpi)
	doc, err := iter.Advance(11)
	if err != nil {
		t.Fatal(err)
	}
	if doc != 12 {
		t.Errorf("expected 12, got %d", doc)
	}
	doc, err = iter.Advance(13)
	if err != nil {
		t.Fatal(err)
	}
	if doc != 14 {
		t.Errorf("expected 14, got %d", doc)
	}
	doc, err = iter.Advance(15)
	if err != nil {
		t.Fatal(err)
	}
	if doc != NO_MORE_DOCS {
		t.Errorf("expected NO_MORE_DOCS, got %d", doc)
	}
}
func TestTwoPhaseIterator_Unwrap(t *testing.T) {
	docs := []int{10}
	matches := map[int]bool{10: true}
	approx := &mockDocIdSetIterator{docs: docs, cursor: -1}
	tpi := &mockTwoPhaseIterator{
		approx:   approx,
		matches:  matches,
		matchCost: 1.0,
	}
	iter := AsDocIdSetIterator(tpi)
	unwrapped := UnwrapTwoPhaseIterator(iter)
	if unwrapped != tpi {
		t.Errorf("expected unwrapped to be tpi, got %v", unwrapped)
	}
	unwrappedOther := UnwrapTwoPhaseIterator(approx)
	if unwrappedOther != nil {
		t.Errorf("expected nil for non-wrapped iterator, got %v", unwrappedOther)
	}
}
func TestTwoPhaseIterator_IntoBitSet(t *testing.T) {
	docs := []int{10, 11, 12, 13, 14}
	matches := map[int]bool{
		10: true,
		11: false,
		12: true,
		13: false,
		14: true,
	}
	approx := &mockDocIdSetIterator{docs: docs, cursor: -1}
	tpi := &mockTwoPhaseIterator{
		approx:   approx,
		matches:  matches,
		matchCost: 1.0,
	}
	bs, _ := NewFixedBitSet(20)
	approx.NextDoc() // now on 10
	err := tpi.IntoBitSet(15, bs, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !bs.Get(10) {
		t.Error("expected bit 10 to be set")
	}
	if bs.Get(11) {
		t.Error("expected bit 11 NOT to be set")
	}
	if !bs.Get(12) {
		t.Error("expected bit 12 to be set")
	}
	if bs.Get(13) {
		t.Error("expected bit 13 NOT to be set")
	}
	if !bs.Get(14) {
		t.Error("expected bit 14 to be set")
	}
}
