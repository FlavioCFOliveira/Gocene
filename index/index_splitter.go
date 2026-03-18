// Package index provides core index functionality for Gocene.
// This file implements the IndexSplitter tool for splitting/copying/deleting segments.
// Source: org.apache.lucene.index.IndexSplitter (Apache Lucene 10.x)
package index

import (
	"fmt"
	"io"
	"os"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// IndexSplitter provides operations to list, copy, or delete specific segments from an index.
//
// Limitations:
//   - Cannot split a single segment into multiple segments
//   - Only works with FSDirectory implementations (uses file system operations)
//
// Warning: This API is experimental. Be careful when using it to avoid accidentally
// removing segments.
//
// This is the Go port of Lucene's org.apache.lucene.index.IndexSplitter.
type IndexSplitter struct {
	srcDir     store.Directory
	infoStream io.Writer
}

// NewIndexSplitter creates a new IndexSplitter for the given source directory.
func NewIndexSplitter(srcDir store.Directory) (*IndexSplitter, error) {
	// Check if the directory is an FSDirectory (required for file operations)
	if _, ok := srcDir.(*store.NIOFSDirectory); !ok {
		if _, ok := srcDir.(*store.MMapDirectory); !ok {
			return nil, fmt.Errorf("IndexSplitter only works with FSDirectory implementations")
		}
	}

	return &IndexSplitter{
		srcDir:     srcDir,
		infoStream: os.Stdout,
	}, nil
}

// SetInfoStream sets the output stream for logging.
func (is *IndexSplitter) SetInfoStream(out io.Writer) {
	if out == nil {
		is.infoStream = io.Discard
	} else {
		is.infoStream = out
	}
}

// msg prints a message to the info stream.
func (is *IndexSplitter) msg(message string) {
	if is.infoStream != nil {
		fmt.Fprintln(is.infoStream, message)
	}
}

// msgf prints a formatted message to the info stream.
func (is *IndexSplitter) msgf(format string, args ...interface{}) {
	if is.infoStream != nil {
		fmt.Fprintf(is.infoStream, format+"\n", args...)
	}
}

// ListSegments lists all segments in the index with their information.
func (is *IndexSplitter) ListSegments() error {
	is.msg("")
	is.msg("Listing segments...")
	is.msg("")

	// Read segment infos
	segmentInfos, err := ReadSegmentInfos(is.srcDir)
	if err != nil {
		return fmt.Errorf("cannot read segment infos: %w", err)
	}

	is.msgf("Total segments: %d", segmentInfos.Size())
	is.msg("")

	for i := 0; i < segmentInfos.Size(); i++ {
		sci := segmentInfos.Get(i)
		if sci == nil {
			continue
		}

		segInfo := sci.SegmentInfo()
		is.msgf("Segment %d:", i)
		is.msgf("  Name: %s", segInfo.Name())
		is.msgf("  DocCount: %d", segInfo.DocCount())
		is.msgf("  NumDocs: %d", sci.NumDocs())
		is.msgf("  DelCount: %d", sci.DelCount())
		is.msgf("  HasDeletions: %v", sci.HasDeletions())
		is.msgf("  Codec: %s", segInfo.Codec())
		is.msgf("  CompoundFile: %v", segInfo.IsCompoundFile())
		is.msg("")
	}

	return nil
}

// Split copies the specified segments to a destination directory.
// The destination will contain only the specified segments.
func (is *IndexSplitter) Split(destDir string, segmentNames []string) error {
	if len(segmentNames) == 0 {
		return fmt.Errorf("no segments specified")
	}

	is.msg("")
	is.msgf("Splitting segments to %s...", destDir)
	is.msg("")

	// Read segment infos
	segmentInfos, err := ReadSegmentInfos(is.srcDir)
	if err != nil {
		return fmt.Errorf("cannot read segment infos: %w", err)
	}

	// Create destination directory
	destDirectory, err := store.NewNIOFSDirectory(destDir)
	if err != nil {
		return fmt.Errorf("cannot create destination directory: %w", err)
	}
	defer destDirectory.Close()

	// Build a set of segment names to copy
	segmentsToCopy := make(map[string]bool)
	for _, name := range segmentNames {
		segmentsToCopy[name] = true
	}

	// Copy segment files
	copiedCount := 0
	for i := 0; i < segmentInfos.Size(); i++ {
		sci := segmentInfos.Get(i)
		if sci == nil {
			continue
		}

		segName := sci.Name()
		if !segmentsToCopy[segName] {
			continue
		}

		// Copy segment files
		segInfo := sci.SegmentInfo()
		files := segInfo.Files()

		for _, file := range files {
			if err := is.copyFile(file, is.srcDir, destDirectory); err != nil {
				return fmt.Errorf("cannot copy file %s: %w", file, err)
			}
		}

		// Copy deletion files if present
		if sci.HasDeletions() {
			delFile := sci.GetDelFileName()
			if delFile != "" {
				if err := is.copyFile(delFile, is.srcDir, destDirectory); err != nil {
					return fmt.Errorf("cannot copy deletion file %s: %w", delFile, err)
				}
			}
		}

		copiedCount++
		is.msgf("Copied segment %s", segName)
	}

	// Create new segments file for the destination
	newSegmentInfos := NewSegmentInfos()
	for i := 0; i < segmentInfos.Size(); i++ {
		sci := segmentInfos.Get(i)
		if sci == nil {
			continue
		}

		if segmentsToCopy[sci.Name()] {
			newSegmentInfos.Add(sci)
		}
	}

	// Write the segments file
	if err := WriteSegmentInfos(newSegmentInfos, destDirectory); err != nil {
		return fmt.Errorf("cannot write segment infos: %w", err)
	}

	is.msg("")
	is.msgf("Split completed: copied %d segments", copiedCount)
	is.msg("")

	return nil
}

// Remove removes the specified segments from the source index.
// Warning: This permanently deletes data. Use with caution.
func (is *IndexSplitter) Remove(segmentNames []string) error {
	if len(segmentNames) == 0 {
		return fmt.Errorf("no segments specified")
	}

	is.msg("")
	is.msg("WARNING: This will permanently delete the following segments:")
	for _, name := range segmentNames {
		is.msgf("  - %s", name)
	}
	is.msg("")

	// Read segment infos
	segmentInfos, err := ReadSegmentInfos(is.srcDir)
	if err != nil {
		return fmt.Errorf("cannot read segment infos: %w", err)
	}

	// Build a set of segment names to remove
	segmentsToRemove := make(map[string]bool)
	for _, name := range segmentNames {
		segmentsToRemove[name] = true
	}

	// Create new segment infos without the removed segments
	newSegmentInfos := NewSegmentInfos()
	removedCount := 0

	for i := 0; i < segmentInfos.Size(); i++ {
		sci := segmentInfos.Get(i)
		if sci == nil {
			continue
		}

		segName := sci.Name()
		if segmentsToRemove[segName] {
			// Remove segment files
			segInfo := sci.SegmentInfo()
			files := segInfo.Files()

			for _, file := range files {
				if err := is.srcDir.DeleteFile(file); err != nil {
					is.msgf("Warning: cannot delete file %s: %v", file, err)
				}
			}

			// Remove deletion files if present
			if sci.HasDeletions() {
				delFile := sci.GetDelFileName()
				if delFile != "" {
					if err := is.srcDir.DeleteFile(delFile); err != nil {
						is.msgf("Warning: cannot delete deletion file %s: %v", delFile, err)
					}
				}
			}

			removedCount++
			is.msgf("Removed segment %s", segName)
		} else {
			newSegmentInfos.Add(sci)
		}
	}

	// Write the new segments file
	if err := WriteSegmentInfos(newSegmentInfos, is.srcDir); err != nil {
		return fmt.Errorf("cannot write segment infos: %w", err)
	}

	is.msg("")
	is.msgf("Remove completed: removed %d segments", removedCount)
	is.msg("")

	return nil
}

// copyFile copies a file from source to destination directory.
func (is *IndexSplitter) copyFile(fileName string, srcDir, destDir store.Directory) error {
	srcLength, err := srcDir.FileLength(fileName)
	if err != nil {
		return err
	}

	if srcLength == 0 {
		// Empty file, just create it
		out, err := destDir.CreateOutput(fileName, store.IOContextWrite)
		if err != nil {
			return err
		}
		return out.Close()
	}

	// Open source input
	in, err := srcDir.OpenInput(fileName, store.IOContextRead)
	if err != nil {
		return err
	}
	defer in.Close()

	// Create destination output
	out, err := destDir.CreateOutput(fileName, store.IOContextWrite)
	if err != nil {
		return err
	}
	defer out.Close()

	// Copy data
	// For simplicity, we read the entire file into memory
	// In production, this should use a buffer for large files
	data := make([]byte, srcLength)
	if err := in.ReadBytes(data); err != nil {
		return err
	}

	if err := out.WriteBytes(data); err != nil {
		return err
	}

	return out.Close()
}

// IndexSplitterMain is the command-line entry point for IndexSplitter.
func IndexSplitterMain(args []string) int {
	if len(args) < 2 {
		fmt.Println("Usage: IndexSplitter <command> <indexDir> [options]")
		fmt.Println("")
		fmt.Println("Commands:")
		fmt.Println("  list                    List all segments")
		fmt.Println("  split <destDir> <seg1> [seg2...]  Copy segments to destination")
		fmt.Println("  remove <seg1> [seg2...]           Remove segments from index")
		return 1
	}

	command := args[0]
	indexDir := args[1]

	// Open the source directory
	srcDir, err := store.NewNIOFSDirectory(indexDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open directory: %v\n", err)
		return 1
	}
	defer srcDir.Close()

	// Create splitter
	splitter, err := NewIndexSplitter(srcDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create splitter: %v\n", err)
		return 1
	}
	splitter.SetInfoStream(os.Stdout)

	switch command {
	case "list":
		if err := splitter.ListSegments(); err != nil {
			fmt.Fprintf(os.Stderr, "List failed: %v\n", err)
			return 1
		}

	case "split":
		if len(args) < 4 {
			fmt.Println("Usage: IndexSplitter split <indexDir> <destDir> <seg1> [seg2...]")
			return 1
		}
		destDir := args[2]
		segmentNames := args[3:]
		if err := splitter.Split(destDir, segmentNames); err != nil {
			fmt.Fprintf(os.Stderr, "Split failed: %v\n", err)
			return 1
		}

	case "remove":
		if len(args) < 3 {
			fmt.Println("Usage: IndexSplitter remove <indexDir> <seg1> [seg2...]")
			return 1
		}
		segmentNames := args[2:]
		if err := splitter.Remove(segmentNames); err != nil {
			fmt.Fprintf(os.Stderr, "Remove failed: %v\n", err)
			return 1
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		return 1
	}

	return 0
}
