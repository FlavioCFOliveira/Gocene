package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// mockDirectory is a minimal mock implementation of store.Directory for testing
type mockDirectory struct{}

func (m *mockDirectory) ListAll() ([]string, error)            { return nil, nil }
func (m *mockDirectory) FileExists(name string) bool           { return false }
func (m *mockDirectory) FileLength(name string) (int64, error) { return 0, nil }
func (m *mockDirectory) CreateOutput(name string, ctx store.IOContext) (store.IndexOutput, error) {
	return nil, nil
}
func (m *mockDirectory) OpenInput(name string, ctx store.IOContext) (store.IndexInput, error) {
	return nil, nil
}
func (m *mockDirectory) DeleteFile(name string) error               { return nil }
func (m *mockDirectory) ObtainLock(name string) (store.Lock, error) { return nil, nil }
func (m *mockDirectory) Close() error                               { return nil }
func (m *mockDirectory) GetDirectory() store.Directory              { return m }

func TestNewNRTFileDeleter(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}
	defer deleter.Close()

	if deleter == nil {
		t.Fatal("expected NRTFileDeleter to not be nil")
	}

	if !deleter.IsOpen() {
		t.Error("expected deleter to be open")
	}
}

func TestNewNRTFileDeleter_Nil(t *testing.T) {
	_, err := NewNRTFileDeleter(nil)
	if err == nil {
		t.Error("expected error for nil directory")
	}
}

func TestNRTFileDeleter_Delete(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}
	defer deleter.Close()

	// Delete a file
	err = deleter.Delete("test.txt")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// Should have deleted the file
	if deleter.GetDeleteCount() != 1 {
		t.Errorf("expected 1 deletion, got %d", deleter.GetDeleteCount())
	}
}

func TestNRTFileDeleter_Delete_Protected(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}
	defer deleter.Close()

	// Protect the file
	err = deleter.Protect("test.txt")
	if err != nil {
		t.Fatalf("failed to protect: %v", err)
	}

	// Delete should add to pending, not actually delete
	err = deleter.Delete("test.txt")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// Should not have deleted yet
	if deleter.GetDeleteCount() != 0 {
		t.Errorf("expected 0 deletions, got %d", deleter.GetDeleteCount())
	}

	// Should be pending
	if !deleter.IsPendingDeletion("test.txt") {
		t.Error("expected file to be pending deletion")
	}

	// Unprotect should trigger deletion
	err = deleter.Unprotect("test.txt")
	if err != nil {
		t.Fatalf("failed to unprotect: %v", err)
	}

	// Should have deleted now
	if deleter.GetDeleteCount() != 1 {
		t.Errorf("expected 1 deletion, got %d", deleter.GetDeleteCount())
	}

	// Should not be pending anymore
	if deleter.IsPendingDeletion("test.txt") {
		t.Error("expected file to not be pending deletion")
	}
}

func TestNRTFileDeleter_Protect(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}
	defer deleter.Close()

	// Protect a file
	err = deleter.Protect("test.txt")
	if err != nil {
		t.Fatalf("failed to protect: %v", err)
	}

	if !deleter.IsProtected("test.txt") {
		t.Error("expected file to be protected")
	}

	if deleter.GetProtectedCount() != 1 {
		t.Errorf("expected 1 protected file, got %d", deleter.GetProtectedCount())
	}
}

func TestNRTFileDeleter_Protect_Multiple(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}
	defer deleter.Close()

	// Protect same file multiple times
	for i := 0; i < 3; i++ {
		err = deleter.Protect("test.txt")
		if err != nil {
			t.Fatalf("failed to protect (iteration %d): %v", i, err)
		}
	}

	if deleter.GetProtectedCount() != 3 {
		t.Errorf("expected 3 protections, got %d", deleter.GetProtectedCount())
	}

	// Need to unprotect 3 times
	for i := 0; i < 3; i++ {
		err = deleter.Unprotect("test.txt")
		if err != nil {
			t.Fatalf("failed to unprotect (iteration %d): %v", i, err)
		}
	}

	if deleter.GetProtectedCount() != 0 {
		t.Errorf("expected 0 protections, got %d", deleter.GetProtectedCount())
	}

	// Fourth unprotect should error
	err = deleter.Unprotect("test.txt")
	if err == nil {
		t.Error("expected error when unprotecting non-protected file")
	}
}

func TestNRTFileDeleter_Unprotect_NotProtected(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}
	defer deleter.Close()

	// Unprotect a file that was never protected
	err = deleter.Unprotect("test.txt")
	if err == nil {
		t.Error("expected error when unprotecting non-protected file")
	}
}

func TestNRTFileDeleter_IsProtected(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}
	defer deleter.Close()

	// Initially not protected
	if deleter.IsProtected("test.txt") {
		t.Error("expected file to not be protected initially")
	}

	// Protect
	deleter.Protect("test.txt")

	if !deleter.IsProtected("test.txt") {
		t.Error("expected file to be protected")
	}

	// Unprotect
	deleter.Unprotect("test.txt")

	if deleter.IsProtected("test.txt") {
		t.Error("expected file to not be protected after unprotect")
	}
}

func TestNRTFileDeleter_IsPendingDeletion(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}
	defer deleter.Close()

	// Initially not pending
	if deleter.IsPendingDeletion("test.txt") {
		t.Error("expected file to not be pending initially")
	}

	// Protect and delete
	deleter.Protect("test.txt")
	deleter.Delete("test.txt")

	if !deleter.IsPendingDeletion("test.txt") {
		t.Error("expected file to be pending deletion")
	}
}

func TestNRTFileDeleter_Close(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}

	err = deleter.Close()
	if err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if deleter.IsOpen() {
		t.Error("expected deleter to be closed")
	}

	// Close again should not error
	err = deleter.Close()
	if err != nil {
		t.Errorf("expected no error on second close: %v", err)
	}
}

func TestNRTFileDeleter_Close_Operations(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}

	deleter.Close()

	// Operations should fail when closed
	err = deleter.Delete("test.txt")
	if err == nil {
		t.Error("expected error when deleting on closed deleter")
	}

	err = deleter.Protect("test.txt")
	if err == nil {
		t.Error("expected error when protecting on closed deleter")
	}

	err = deleter.Unprotect("test.txt")
	if err == nil {
		t.Error("expected error when unprotecting on closed deleter")
	}
}

func TestNRTFileDeleter_GetPendingDeletions(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}
	defer deleter.Close()

	// Initially empty
	pending := deleter.GetPendingDeletions()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending deletions, got %d", len(pending))
	}

	// Protect and delete
	deleter.Protect("test.txt")
	deleter.Delete("test.txt")

	pending = deleter.GetPendingDeletions()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending deletion, got %d", len(pending))
	}

	if count, ok := pending["test.txt"]; !ok || count != 1 {
		t.Error("expected test.txt to be pending with count 1")
	}
}

func TestNRTFileDeleter_GetProtectedFiles(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}
	defer deleter.Close()

	// Initially empty
	protected := deleter.GetProtectedFiles()
	if len(protected) != 0 {
		t.Errorf("expected 0 protected files, got %d", len(protected))
	}

	// Protect
	deleter.Protect("test.txt")

	protected = deleter.GetProtectedFiles()
	if len(protected) != 1 {
		t.Errorf("expected 1 protected file, got %d", len(protected))
	}

	if count, ok := protected["test.txt"]; !ok || count != 1 {
		t.Error("expected test.txt to be protected with count 1")
	}
}

func TestNRTFileDeleter_ProcessPendingDeletions(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}
	defer deleter.Close()

	// Protect and delete multiple files
	deleter.Protect("file1.txt")
	deleter.Protect("file2.txt")
	deleter.Delete("file1.txt")
	deleter.Delete("file2.txt")

	// Should be pending
	if len(deleter.GetPendingDeletions()) != 2 {
		t.Error("expected 2 pending deletions")
	}

	// Unprotect file1
	deleter.Unprotect("file1.txt")

	// Should have deleted file1
	if deleter.GetDeleteCount() != 1 {
		t.Errorf("expected 1 deletion, got %d", deleter.GetDeleteCount())
	}

	// file2 should still be pending
	if !deleter.IsPendingDeletion("file2.txt") {
		t.Error("expected file2.txt to still be pending")
	}

	// Process pending deletions
	err = deleter.ProcessPendingDeletions()
	if err != nil {
		t.Fatalf("failed to process pending deletions: %v", err)
	}

	// file2 should still be pending because it's still protected
	if !deleter.IsPendingDeletion("file2.txt") {
		t.Error("expected file2.txt to still be pending (still protected)")
	}

	// Unprotect file2
	deleter.Unprotect("file2.txt")

	// Process again
	err = deleter.ProcessPendingDeletions()
	if err != nil {
		t.Fatalf("failed to process pending deletions: %v", err)
	}

	// Now file2 should be deleted
	if deleter.IsPendingDeletion("file2.txt") {
		t.Error("expected file2.txt to not be pending")
	}

	if deleter.GetDeleteCount() != 2 {
		t.Errorf("expected 2 deletions, got %d", deleter.GetDeleteCount())
	}
}

func TestNRTFileDeleter_ProcessPendingDeletions_Closed(t *testing.T) {
	dir := &mockDirectory{}

	deleter, _ := NewNRTFileDeleter(dir)
	deleter.Close()

	err := deleter.ProcessPendingDeletions()
	if err == nil {
		t.Error("expected error when processing on closed deleter")
	}
}

func TestNRTFileDeleter_Clear(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}
	defer deleter.Close()

	// Protect and delete
	deleter.Protect("test.txt")
	deleter.Delete("test.txt")

	if len(deleter.GetPendingDeletions()) != 1 {
		t.Error("expected 1 pending deletion")
	}

	// Clear
	deleter.Clear()

	if len(deleter.GetPendingDeletions()) != 0 {
		t.Error("expected 0 pending deletions after clear")
	}
}

func TestNRTFileDeleter_String(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}
	defer deleter.Close()

	str := deleter.String()
	if str == "" {
		t.Error("expected non-empty string")
	}

	// Should contain expected fields
	expected := "NRTFileDeleter{pending=0, protected=0, deleted=0}"
	if str != expected {
		t.Errorf("expected %q, got %q", expected, str)
	}
}

func TestNRTFileDeleter_MultiplePending(t *testing.T) {
	dir := &mockDirectory{}

	deleter, err := NewNRTFileDeleter(dir)
	if err != nil {
		t.Fatalf("failed to create NRTFileDeleter: %v", err)
	}
	defer deleter.Close()

	// Protect and delete same file multiple times
	deleter.Protect("test.txt")
	deleter.Delete("test.txt")
	deleter.Delete("test.txt")
	deleter.Delete("test.txt")

	// Should have 3 pending
	pending := deleter.GetPendingDeletions()
	if count, ok := pending["test.txt"]; !ok || count != 3 {
		t.Errorf("expected 3 pending, got %d", count)
	}

	// Unprotect should delete all 3
	deleter.Unprotect("test.txt")

	if deleter.GetDeleteCount() != 3 {
		t.Errorf("expected 3 deletions, got %d", deleter.GetDeleteCount())
	}
}
