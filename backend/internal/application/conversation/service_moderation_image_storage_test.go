package conversation

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func TestOutputModerationUsesOriginalImageMetadata(t *testing.T) {
	data := []byte("\x89PNG\r\n\x1a\noriginal generated image")
	files := []model.FileObject{{FileID: "image", MimeType: "image/webp", DetectedMIME: "image/webp", SHA256: "stored-webp-hash"}}
	images := loadOutputImagesFromFiles(nil, files, map[string][]byte{"image": data})
	digest := sha256.Sum256(data)
	if len(images) != 1 || images[0].MimeType != "image/png" || images[0].SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("moderation metadata must describe original bytes: %+v", images)
	}
}
