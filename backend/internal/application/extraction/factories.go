package extraction

import (
	"context"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	extractport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/extract"
)

// DocumentExtractor 定义文档抽取引擎（Tika/Docling/MinerU）的调用端口。
type DocumentExtractor interface {
	ExtractText(ctx context.Context, req extractport.DocumentRequest) (string, error)
}

// OCRExtractor 定义 OCR 引擎的调用端口。
type OCRExtractor interface {
	ExtractText(ctx context.Context, req extractport.OCRRequest) (extractport.OCRResponse, error)
}

// BuiltinParser 定义内置本地解析（文本/Word/Excel/PDF）的调用端口。
type BuiltinParser interface {
	ExtractText(data []byte) string
	ExtractWordText(ctx context.Context, absolutePath string, data []byte, mimeType string, fileName string) extractport.WordTextResult
	ExtractExcelText(data []byte, mimeType string, fileName string) string
	ExtractPDFText(absolutePath string, maxPages int) (string, error)
	ExtractPDFPages(absolutePath string, maxPages int) (extractport.PDFTextResult, error)
	DetectPDFPageCount(absolutePath string) int
}

// EngineFactories 由组合根注入，按配置创建各抽取引擎客户端。
// 工厂在引擎不可用（未配置、提供方未知）时必须返回 nil 接口。
type EngineFactories struct {
	NewTika    func(cfg config.Config) DocumentExtractor
	NewDocling func(cfg config.Config) DocumentExtractor
	NewMinerU  func(cfg config.Config) DocumentExtractor
	NewOCR     func(provider string, cfg config.Config) OCRExtractor
	Builtin    BuiltinParser
}
