package index

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestSetLiveCommitData(t *testing.T) {
	dir := util.NewBytesDirectory()
	conf := NewIndexWriterConfig()
	iw, err := NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("failed to create IndexWriter: %v", err)
	}
	defer iw.Close()

	// 1. Test dynamic provider (late-binding)
	counter := 0
	provider := func() map[string]string {
		counter++
		return map[string]string{"commit_id": strconv.Itoa(counter)}
	}

	iw.SetLiveCommitData(provider)

	// First commit
	_, err = iw.Commit()
	if err != nil {
		t.Fatalf("first commit failed: %v", err)
	}

	reader, err := iw.GetReader(true)
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}
	userData := reader.GetSegmentInfos().GetUserData()
	if userData["commit_id"] != "1" {
		t.Errorf("expected commit_id 1, got %q", userData["commit_id"])
	}

	// Second commit
	_, err = iw.Commit()
	if err != nil {
		t.Fatalf("second commit failed: %v", err)
	}

	reader2, err := iw.GetReader(true)
	if err != nil {
		t.Fatalf("failed to get reader: %v", err)
	}
	userData2 := reader2.GetSegmentInfos().GetUserData()
	if userData2["commit_id"] != "2" {
		t.Errorf("expected commit_id 2, got %q", userData2["commit_id"])
	}
}
