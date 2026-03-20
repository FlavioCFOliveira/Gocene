package search

import (
	"context"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestNewNRTSearcher(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sm, err := NewSearcherManager(dir, nil)
	if err != nil {
		t.Fatalf("failed to create SearcherManager: %v", err)
	}
	defer sm.Close()

	searcher, err := NewNRTSearcher(sm)
	if err != nil {
		t.Fatalf("failed to create NRTSearcher: %v", err)
	}
	defer searcher.Close()

	if searcher == nil {
		t.Fatal("expected NRTSearcher to not be nil")
	}

	if !searcher.IsOpen() {
		t.Error("expected searcher to be open")
	}

	if searcher.GetVersion() != 1 {
		t.Errorf("expected version 1, got %d", searcher.GetVersion())
	}

	if searcher.GetRefreshCount() != 0 {
		t.Errorf("expected 0 refreshes, got %d", searcher.GetRefreshCount())
	}
}

func TestNewNRTSearcher_NilManager(t *testing.T) {
	_, err := NewNRTSearcher(nil)
	if err == nil {
		t.Error("expected error for nil manager")
	}
}

func TestNRTSearcher_Refresh(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sm, _ := NewSearcherManager(dir, nil)
	defer sm.Close()

	searcher, _ := NewNRTSearcher(sm)
	defer searcher.Close()

	initialVersion := searcher.GetVersion()
	initialRefreshCount := searcher.GetRefreshCount()

	ctx := context.Background()
	err := searcher.Refresh(ctx)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	if searcher.GetVersion() != initialVersion+1 {
		t.Errorf("expected version %d, got %d", initialVersion+1, searcher.GetVersion())
	}

	if searcher.GetRefreshCount() != initialRefreshCount+1 {
		t.Errorf("expected refresh count %d, got %d", initialRefreshCount+1, searcher.GetRefreshCount())
	}
}

func TestNRTSearcher_Refresh_Closed(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sm, _ := NewSearcherManager(dir, nil)
	defer sm.Close()

	searcher, _ := NewNRTSearcher(sm)
	searcher.Close()

	ctx := context.Background()
	err := searcher.Refresh(ctx)
	if err == nil {
		t.Error("expected error when refreshing closed searcher")
	}
}

func TestNRTSearcher_IncrementVersion(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sm, _ := NewSearcherManager(dir, nil)
	defer sm.Close()

	searcher, _ := NewNRTSearcher(sm)
	defer searcher.Close()

	initialVersion := searcher.GetVersion()

	searcher.IncrementVersion()

	if searcher.GetVersion() != initialVersion+1 {
		t.Errorf("expected version %d, got %d", initialVersion+1, searcher.GetVersion())
	}
}

func TestNRTSearcher_GetLastRefreshTime(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sm, _ := NewSearcherManager(dir, nil)
	defer sm.Close()

	searcher, _ := NewNRTSearcher(sm)
	defer searcher.Close()

	initialTime := searcher.GetLastRefreshTime()
	if initialTime.IsZero() {
		t.Error("expected initial refresh time to be set")
	}

	time.Sleep(10 * time.Millisecond)
	ctx := context.Background()
	searcher.Refresh(ctx)

	newTime := searcher.GetLastRefreshTime()
	if !newTime.After(initialTime) {
		t.Error("expected new refresh time to be after initial time")
	}
}

func TestNRTSearcher_AutoRefresh(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sm, _ := NewSearcherManager(dir, nil)
	defer sm.Close()

	searcher, _ := NewNRTSearcher(sm)
	defer searcher.Close()

	err := searcher.StartAutoRefresh(50 * time.Millisecond)
	if err != nil {
		t.Fatalf("failed to start auto-refresh: %v", err)
	}

	if !searcher.IsAutoRefreshRunning() {
		t.Error("expected auto-refresh to be running")
	}

	time.Sleep(150 * time.Millisecond)

	if searcher.GetRefreshCount() < 1 {
		t.Error("expected at least 1 refresh from auto-refresh")
	}

	err = searcher.StopAutoRefresh()
	if err != nil {
		t.Fatalf("failed to stop auto-refresh: %v", err)
	}

	if searcher.IsAutoRefreshRunning() {
		t.Error("expected auto-refresh to be stopped")
	}
}

func TestNRTSearcher_StartAutoRefresh_AlreadyRunning(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sm, _ := NewSearcherManager(dir, nil)
	defer sm.Close()

	searcher, _ := NewNRTSearcher(sm)
	defer searcher.Close()

	searcher.StartAutoRefresh(100 * time.Millisecond)

	err := searcher.StartAutoRefresh(50 * time.Millisecond)
	if err == nil {
		t.Error("expected error when starting auto-refresh that is already running")
	}
}

func TestNRTSearcher_StartAutoRefresh_InvalidInterval(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sm, _ := NewSearcherManager(dir, nil)
	defer sm.Close()

	searcher, _ := NewNRTSearcher(sm)
	defer searcher.Close()

	err := searcher.StartAutoRefresh(0)
	if err == nil {
		t.Error("expected error for zero interval")
	}

	err = searcher.StartAutoRefresh(-1 * time.Second)
	if err == nil {
		t.Error("expected error for negative interval")
	}
}

func TestNRTSearcher_StartAutoRefresh_Closed(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sm, _ := NewSearcherManager(dir, nil)
	defer sm.Close()

	searcher, _ := NewNRTSearcher(sm)
	searcher.Close()

	err := searcher.StartAutoRefresh(100 * time.Millisecond)
	if err == nil {
		t.Error("expected error when starting auto-refresh on closed searcher")
	}
}

func TestNRTSearcher_GetManager(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sm, _ := NewSearcherManager(dir, nil)
	defer sm.Close()

	searcher, _ := NewNRTSearcher(sm)
	defer searcher.Close()

	if searcher.GetManager() != sm {
		t.Error("expected manager to be set")
	}
}

func TestNRTSearcher_Close(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sm, _ := NewSearcherManager(dir, nil)
	defer sm.Close()

	searcher, _ := NewNRTSearcher(sm)

	err := searcher.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if searcher.IsOpen() {
		t.Error("expected searcher to be closed")
	}

	err = searcher.Close()
	if err != nil {
		t.Errorf("second close failed: %v", err)
	}
}

func TestNRTSearcher_Close_WithAutoRefresh(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sm, _ := NewSearcherManager(dir, nil)
	defer sm.Close()

	searcher, _ := NewNRTSearcher(sm)
	searcher.StartAutoRefresh(100 * time.Millisecond)

	err := searcher.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if searcher.IsOpen() {
		t.Error("expected searcher to be closed")
	}

	if searcher.IsAutoRefreshRunning() {
		t.Error("expected auto-refresh to be stopped")
	}
}

func TestNRTSearcher_String(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sm, _ := NewSearcherManager(dir, nil)
	defer sm.Close()

	searcher, _ := NewNRTSearcher(sm)
	defer searcher.Close()

	str := searcher.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}
