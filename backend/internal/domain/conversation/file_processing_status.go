package conversation

// IsFileProcessing includes queued embedding work even after extraction completes.
func IsFileProcessing(file FileObject) bool {
	switch file.ProcessingStatus {
	case FileProcessingStatusUploaded, FileProcessingStatusQueued, FileProcessingStatusExtracting, FileProcessingStatusEmbedding:
		return true
	default:
		return file.ExtractStatus == FileSubprocessStatusProcessing || file.EmbedStatus == FileSubprocessStatusQueued || file.EmbedStatus == FileSubprocessStatusProcessing
	}
}

const (
	FileProcessingStatusUploaded   = "uploaded"
	FileProcessingStatusQueued     = "queued"
	FileProcessingStatusExtracting = "extracting"
	FileProcessingStatusEmbedding  = "embedding"
	FileSubprocessStatusQueued     = "queued"
	FileSubprocessStatusProcessing = "processing"
	// FileSubprocessStatusEmpty is a successful terminal state with no text.
	FileSubprocessStatusEmpty      = "empty"
	FileErrorCodeNoExtractableText = "no_extractable_text"
)
