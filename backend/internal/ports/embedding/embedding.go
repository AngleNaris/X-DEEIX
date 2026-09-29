// Package embedding defines the application-facing contract for embeddings.
//
// Inputs are multimodal: a batch may mix text and image items. Which
// modalities a deployment can actually embed depends on the configured
// protocol; callers consult Capabilities before building image inputs.
package embedding

import "errors"

// Protocol identifies the wire format spoken by the embedding endpoint.
type Protocol string

const (
	// ProtocolOpenAI is the OpenAI-compatible POST /embeddings request with a string array input.
	ProtocolOpenAI Protocol = "openai"
	// ProtocolGemini is the Google Generative Language batchEmbedContents request.
	ProtocolGemini Protocol = "gemini"
	// ProtocolVoyage is the Voyage AI POST /multimodalembeddings request.
	ProtocolVoyage Protocol = "voyage"
	// ProtocolJina is the Jina AI POST /embeddings request with typed input objects.
	ProtocolJina Protocol = "jina"
)

// Purpose tells providers that distinguish indexing from querying which side a
// batch is on; providers without that distinction ignore it.
type Purpose string

const (
	PurposeDocument Purpose = "document"
	PurposeQuery    Purpose = "query"
)

// InputKind names the modality of one Input.
type InputKind string

const (
	InputText  InputKind = "text"
	InputImage InputKind = "image"
)

// Input is one item in an embedding batch.
type Input struct {
	Kind InputKind
	// Text is set when Kind is InputText.
	Text string
	// MimeType and Data are set when Kind is InputImage. Data holds the raw
	// image bytes; the protocol adapter decides how to encode them.
	MimeType string
	Data     []byte
}

// TextInputs wraps plain strings as text inputs.
func TextInputs(texts []string) []Input {
	inputs := make([]Input, 0, len(texts))
	for _, text := range texts {
		inputs = append(inputs, Input{Kind: InputText, Text: text})
	}
	return inputs
}

// Request describes one batch sent to an embedding provider.
type Request struct {
	Protocol   Protocol
	APIBase    string
	APIKey     string
	Model      string
	Inputs     []Input
	Purpose    Purpose
	Dimensions int
	// OmitDimensions controls only request serialization. Dimensions remains the
	// expected response width and is always used for validation.
	OmitDimensions bool
	TimeoutSeconds int
}

// Capabilities reports which modalities a protocol can embed.
type Capabilities struct {
	Image bool
}

// ErrModalityUnsupported is returned when a batch contains an input kind the
// selected protocol cannot embed.
var ErrModalityUnsupported = errors.New("embedding: input modality is not supported by the configured protocol")

// ProtocolCapabilities returns the modality support for a protocol. Unknown
// protocols are treated as text-only.
func ProtocolCapabilities(protocol Protocol) Capabilities {
	switch protocol {
	case ProtocolGemini, ProtocolVoyage, ProtocolJina:
		return Capabilities{Image: true}
	default:
		return Capabilities{}
	}
}
