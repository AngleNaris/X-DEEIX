package upload

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// Bound native encoder memory and CPU independently of the compressed upload size.
var imageEncodingSlots = make(chan struct{}, 2)

// convertStoredImage returns a smaller WebP, or nil to retain the original.
func convertStoredImage(ctx context.Context, path, mimeType, format string, quality int) ([]byte, error) {
	if (format != "webp_lossless" && format != "webp_lossy") || (mimeType != "image/png" && mimeType != "image/jpeg") {
		return nil, nil
	}
	select {
	case imageEncodingSlots <- struct{}{}:
		defer func() { <-imageEncodingSlots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(source))
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 32_000_000 {
		return nil, nil
	}
	if mimeType == "image/png" && !convertiblePNG(source) {
		return nil, nil
	}
	output, err := os.CreateTemp("", "deeix-webp-*.webp")
	if err != nil {
		return nil, err
	}
	outputPath := output.Name()
	defer os.Remove(outputPath)
	if err = output.Close(); err != nil {
		return nil, err
	}
	if quality < 1 || quality > 100 {
		quality = 85
	}
	args := []string{"-quiet", "-metadata", "all", "-q", strconv.Itoa(quality)}
	if format == "webp_lossless" {
		args = append(args, "-lossless", "-exact")
	}
	args = append(args, path, "-o", outputPath)
	encodeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err = exec.CommandContext(encodeCtx, "cwebp", args...).Run(); err != nil {
		return nil, err
	}
	stat, err := os.Stat(outputPath)
	if err != nil {
		return nil, err
	}
	if stat.Size() <= 0 || stat.Size() >= int64(len(source)) {
		return nil, nil
	}
	return os.ReadFile(outputPath)
}

// APNG and 16-bit PNG must not silently lose animation or precision.
func convertiblePNG(data []byte) bool {
	if len(data) < 33 || data[24] > 8 {
		return false
	}
	for offset := 8; offset+12 <= len(data); {
		length := uint64(binary.BigEndian.Uint32(data[offset : offset+4]))
		if length+12 > uint64(len(data)-offset) {
			return false
		}
		if string(data[offset+4:offset+8]) == "acTL" {
			return false
		}
		offset += int(length) + 12
	}
	return true
}
