package extraction

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// ExtractTemporaryFile reads caller-owned temporary files without object-store access.
func (s *Service) ExtractTemporaryFile(ctx context.Context, input ExtractInput) (Result, error) {
	path, err := validatedTemporaryExtractionPath(input.File.StoragePath)
	if err != nil {
		return Result{}, err
	}
	return s.extractLocalFile(ctx, input, path)
}

func validatedTemporaryExtractionPath(path string) (string, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(path) {
		return "", ErrInvalidStoredFilePath
	}
	root, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return "", ErrInvalidStoredFilePath
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", ErrInvalidStoredFilePath
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", ErrInvalidStoredFilePath
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", ErrInvalidStoredFilePath
	}
	return resolved, nil
}
