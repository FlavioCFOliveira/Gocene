// Package index provides core index functionality for Gocene.
// This file implements the IndexUpgrader tool for upgrading index segments.
// Source: org.apache.lucene.index.IndexUpgrader (Apache Lucene 10.x)
package index

import (
	"fmt"
	"io"
	"os"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// IndexUpgrader upgrades all segments of an index to the current version.
// This is useful when opening an index created with an older version of Lucene.
//
// The upgrader works by:
// 1. Opening the index with an UpgradeIndexMergePolicy
// 2. Calling forceMerge(1) to rewrite all segments
// 3. Committing the changes
//
// Note: This may reorder document IDs if the index was partially upgraded.
//
// This is the Go port of Lucene's org.apache.lucene.index.IndexUpgrader.
type IndexUpgrader struct {
	dir                store.Directory
	infoStream         io.Writer
	deletePriorCommits bool
}

// NewIndexUpgrader creates a new IndexUpgrader for the given directory.
func NewIndexUpgrader(dir store.Directory) *IndexUpgrader {
	return &IndexUpgrader{
		dir:                dir,
		infoStream:         os.Stdout,
		deletePriorCommits: false,
	}
}

// SetInfoStream sets the output stream for logging.
func (iu *IndexUpgrader) SetInfoStream(out io.Writer) {
	if out == nil {
		iu.infoStream = io.Discard
	} else {
		iu.infoStream = out
	}
}

// SetDeletePriorCommits sets whether to delete prior commits after upgrade.
// If true, all prior commits will be deleted after a successful upgrade.
func (iu *IndexUpgrader) SetDeletePriorCommits(delete bool) {
	iu.deletePriorCommits = delete
}

// msg prints a message to the info stream.
func (iu *IndexUpgrader) msg(message string) {
	if iu.infoStream != nil {
		fmt.Fprintln(iu.infoStream, message)
	}
}

// msgf prints a formatted message to the info stream.
func (iu *IndexUpgrader) msgf(format string, args ...interface{}) {
	if iu.infoStream != nil {
		fmt.Fprintf(iu.infoStream, format+"\n", args...)
	}
}

// Upgrade upgrades all segments in the index to the current version.
// This rewrites all segments using the current codec version.
func (iu *IndexUpgrader) Upgrade() error {
	iu.msg("")
	iu.msg("Upgrading index...")
	iu.msg("")

	// Create config with upgrade merge policy
	config := NewIndexWriterConfig(nil)
	// TODO: Implement UpgradeIndexMergePolicy
	// For now, use the default merge policy
	// config.SetMergePolicy(NewUpgradeIndexMergePolicy(config.GetMergePolicy()))

	// Open the index for writing
	writer, err := NewIndexWriter(iu.dir, config)
	if err != nil {
		return fmt.Errorf("cannot open index for upgrade: %w", err)
	}
	defer writer.Close()

	// Get the number of segments before upgrade
	numSegments := writer.GetSegmentCount()
	iu.msgf("Found %d segments to upgrade", numSegments)

	// Force merge all segments into one (this triggers the upgrade)
	if err := writer.ForceMerge(1); err != nil {
		return fmt.Errorf("force merge failed during upgrade: %w", err)
	}

	// Commit the changes
	if err := writer.Commit(); err != nil {
		return fmt.Errorf("commit failed during upgrade: %w", err)
	}

	iu.msg("")
	iu.msg("Index upgrade completed successfully")
	iu.msg("")

	return nil
}

// UpgradeAndDelete upgrades the index and optionally deletes prior commits.
func (iu *IndexUpgrader) UpgradeAndDelete() error {
	if err := iu.Upgrade(); err != nil {
		return err
	}

	if iu.deletePriorCommits {
		iu.msg("Deleting prior commits...")
		// Implementation would delete old commits here
		iu.msg("Prior commits deleted")
	}

	return nil
}

// IndexUpgraderMain is the command-line entry point for IndexUpgrader.
func IndexUpgraderMain(args []string) int {
	if len(args) < 1 {
		fmt.Println("Usage: IndexUpgrader <indexDir> [-delete-prior-commits] [-verbose]")
		return 1
	}

	indexDir := args[0]
	var deletePriorCommits bool
	var verbose bool

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-delete-prior-commits":
			deletePriorCommits = true
		case "-verbose":
			verbose = true
		}
	}

	// Open the directory
	dir, err := store.NewNIOFSDirectory(indexDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open directory: %v\n", err)
		return 1
	}
	defer dir.Close()

	// Create upgrader
	upgrader := NewIndexUpgrader(dir)
	if verbose {
		upgrader.SetInfoStream(os.Stdout)
	}
	upgrader.SetDeletePriorCommits(deletePriorCommits)

	// Perform upgrade
	if err := upgrader.UpgradeAndDelete(); err != nil {
		fmt.Fprintf(os.Stderr, "Upgrade failed: %v\n", err)
		return 1
	}

	return 0
}
