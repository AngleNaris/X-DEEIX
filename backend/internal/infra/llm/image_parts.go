package llm

import (
	"strings"
)

// collectImageInputParts 收集消息中可发送给图片编辑类协议的原始图片输入。
// 图片可以是内联字节（base64 传输）或 URL（上游自行拉取），二者至少有其一。
func collectImageInputParts(messages []Message) []ContentPart {
	images := make([]ContentPart, 0)
	for _, msg := range messages {
		for _, part := range msg.Parts {
			if part.Kind != ContentPartImage || (len(part.Data) == 0 && strings.TrimSpace(part.URL) == "") {
				continue
			}
			images = append(images, part)
		}
	}
	return images
}
