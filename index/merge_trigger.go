package index

// MergeTrigger is passed to MergePolicy.FindMerges to indicate the event that triggered the merge.
type MergeTrigger int

const (
	MergeTriggerSegmentFlush MergeTrigger = iota
	MergeTriggerFullFlush
	MergeTriggerExplicit
	MergeTriggerMergeFinished
	MergeTriggerClosing
	MergeTriggerCommit
	MergeTriggerGetReader
	MergeTriggerAddIndexes
)

func (mt MergeTrigger) String() string {
	switch mt {
	case MergeTriggerSegmentFlush:
		return "SEGMENT_FLUSH"
	case MergeTriggerFullFlush:
		return "FULL_FLUSH"
	case MergeTriggerExplicit:
		return "EXPLICIT"
	case MergeTriggerMergeFinished:
		return "MERGE_FINISHED"
	case MergeTriggerClosing:
		return "CLOSING"
	case MergeTriggerCommit:
		return "COMMIT"
	case MergeTriggerGetReader:
		return "GET_READER"
	case MergeTriggerAddIndexes:
		return "ADD_INDEXES"
	default:
		return "UNKNOWN"
	}
}
