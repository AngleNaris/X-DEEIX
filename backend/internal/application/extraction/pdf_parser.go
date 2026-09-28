package extraction

import (
	"errors"
	extractport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/extract"
)

func (s *Service) detectPDFPageCount(path string) int {
	if s == nil || s.factories.Builtin == nil {
		return 0
	}
	return s.factories.Builtin.DetectPDFPageCount(path)
}

func (s *Service) extractPDFPagesNative(path string, maxPages int) (extractport.PDFTextResult, error) {
	if s == nil || s.factories.Builtin == nil {
		return extractport.PDFTextResult{}, errors.New("builtin_unavailable")
	}
	return s.factories.Builtin.ExtractPDFPages(path, maxPages)
}
