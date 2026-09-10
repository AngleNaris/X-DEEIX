package upload

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/image/webp"
)

func storageTestPNG(t *testing.T) ([]byte, *image.NRGBA) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 256, 192))
	for y := 0; y < 192; y++ {
		for x := 0; x < 256; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 96, A: uint8(x)})
		}
	}
	var buffer bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.NoCompression}
	if err := encoder.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes(), img
}

func TestImageStorageConvertsUploadsAndGeneratedImages(t *testing.T) {
	if _, err := exec.LookPath("cwebp"); err != nil {
		t.Fatal("install cwebp to run image storage tests")
	}
	source, original := storageTestPNG(t)
	for _, mode := range []string{"webp_lossless", "webp_lossy"} {
		for _, purpose := range []string{"conversation_attachment", "generated_image"} {
			t.Run(mode+"/"+purpose, func(t *testing.T) {
				store := newUploadTestStore()
				service := newUploadTestService(newUploadTestRepo(), store)
				cfg := service.cfg.Snapshot()
				cfg.ImageStorageFormat, cfg.ImageStorageQuality = mode, 85
				service.cfg.Store(cfg)
				input := uploadTestInput("picture.png", string(source))
				input.MimeType, input.Purpose = "image/png", purpose
				result, err := service.UploadFile(t.Context(), input)
				if err != nil {
					t.Fatal(err)
				}
				file := result.File
				stored := store.objects[file.StoragePath]
				if file.FileName != "picture.webp" || file.MimeType != "image/webp" || file.DetectedMIME != "image/webp" || !strings.HasSuffix(file.StoragePath, ".webp") {
					t.Fatalf("inconsistent converted metadata: %+v", file)
				}
				digest := sha256.Sum256(stored)
				if len(stored) >= len(source) || file.SizeBytes != int64(len(stored)) || result.Quota.UsedBytes != int64(len(stored)) || file.SHA256 != hex.EncodeToString(digest[:]) {
					t.Fatal("size, hash or quota does not match stored WebP")
				}
				decoded, err := webp.Decode(bytes.NewReader(stored))
				if err != nil {
					t.Fatal(err)
				}
				if decoded.Bounds() != original.Bounds() {
					t.Fatal("image dimensions changed")
				}
				for _, x := range []int{0, 127, 255} {
					_, _, _, alpha := decoded.At(x, 100).RGBA()
					_, _, _, wantAlpha := original.At(x, 100).RGBA()
					if alpha != wantAlpha {
						t.Fatal("transparency changed")
					}
					if mode == "webp_lossless" && color.NRGBAModel.Convert(decoded.At(x, 100)) != color.NRGBAModel.Convert(original.At(x, 100)) {
						t.Fatal("lossless pixels changed")
					}
				}
				input.Reader = bytes.NewReader(source)
				duplicate, err := service.UploadFile(t.Context(), input)
				if err != nil || !duplicate.Reused || duplicate.File.FileID != file.FileID || duplicate.Quota.UsedBytes != file.SizeBytes {
					t.Fatalf("converted deduplication failed: %v", err)
				}
				t.Logf("%s: %d -> %d bytes", purpose, len(source), len(stored))
			})
		}
	}
}

func TestImageStoragePreservesOriginalAndUploadPolicy(t *testing.T) {
	source, _ := storageTestPNG(t)
	for _, scenario := range []string{"original", "encoder_missing", "webp_disallowed", "source_disallowed", "oversized", "animated_png"} {
		t.Run(scenario, func(t *testing.T) {
			store := newUploadTestStore()
			service := newUploadTestService(newUploadTestRepo(), store)
			cfg := service.cfg.Snapshot()
			cfg.ImageStorageFormat = "webp_lossless"
			data := source
			switch scenario {
			case "original":
				cfg.ImageStorageFormat = "original"
			case "encoder_missing":
				t.Setenv("PATH", t.TempDir())
			case "webp_disallowed":
				cfg.FileAllowedMIMETypes = "image/png"
			case "source_disallowed":
				cfg.FileAllowedMIMETypes = "image/webp"
			case "oversized":
				cfg.FileImageMaxBytes = int64(len(source) - 1)
			case "animated_png":
				// The animation control chunk must prevent flattening, even before full decoding.
				data = append(append(append([]byte{}, source[:33]...), []byte{0, 0, 0, 0, 'a', 'c', 'T', 'L', 0, 0, 0, 0}...), source[33:]...)
			}
			service.cfg.Store(cfg)
			input := uploadTestInput("image.png", string(data))
			input.MimeType = "image/png"
			result, err := service.UploadFile(t.Context(), input)
			if scenario == "source_disallowed" || scenario == "oversized" {
				if err == nil || store.objectCount() != 0 {
					t.Fatal("conversion bypassed upload policy")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.File.FileName != "image.png" || !bytes.Equal(store.objects[result.File.StoragePath], data) {
				t.Fatal("original was not preserved")
			}
		})
	}
}
