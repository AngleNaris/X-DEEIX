package llm

import (
	"bytes"
	"context"
	"fmt"
	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"net/http"
	"strings"
	"time"
)

func (c *Client) postOpenAIImageJSON(
	ctx context.Context,
	route portllm.RouteConfig,
	requestURL string,
	payload []byte,
	debugPayload []byte,
	outputFormat string,
) (*portllm.GenerateOutput, error) {
	requestCtx, cancel := context.WithTimeout(ctx, resolveReadTimeout(route.ReadTimeoutMS))
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey := strings.TrimSpace(route.APIKey); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	setOpenRouterAttributionHeaders(req, route)
	setAdditionalHeaders(req, route.HeadersJSON)

	resp, err := c.doRouteRequest(route, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := readUpstreamBody(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseUpstreamError(resp.StatusCode, body, upstreamDebugSnapshot(req, debugPayload, resp, body))
	}

	return parseOpenAIImageOutput(body, outputFormat)
}

func (c *Client) postOpenAIImageJSONStream(
	ctx context.Context,
	route portllm.RouteConfig,
	requestURL string,
	payload []byte,
	debugPayload []byte,
	outputFormat string,
	onEvent func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	firstByteCtx, firstByteCancel := context.WithCancel(ctx)
	defer firstByteCancel()

	readTimeout := resolveReadTimeout(route.ReadTimeoutMS)
	firstByteTimer := time.AfterFunc(readTimeout, func() {
		firstByteCancel()
	})

	req, err := http.NewRequestWithContext(firstByteCtx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		firstByteTimer.Stop()
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if apiKey := strings.TrimSpace(route.APIKey); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	setOpenRouterAttributionHeaders(req, route)
	setAdditionalHeaders(req, route.HeadersJSON)

	resp, err := c.doRouteRequest(route, req)
	firstByteTimer.Stop()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := readUpstreamBody(resp.Body)
		if readErr != nil {
			return nil, readErr
		}
		return nil, parseUpstreamError(resp.StatusCode, body, upstreamDebugSnapshot(req, debugPayload, resp, body))
	}

	if !isEventStreamContentType(resp.Header.Get("Content-Type")) {
		body, readErr := readUpstreamBody(resp.Body)
		if readErr != nil {
			return nil, readErr
		}
		output, parseErr := parseOpenAIImageOutput(body, outputFormat)
		if parseErr != nil {
			return nil, parseErr
		}
		if output.Usage != (portllm.Usage{}) && onEvent != nil {
			if err := onEvent(portllm.GenerateStreamEvent{Usage: output.Usage}); err != nil {
				return nil, err
			}
		}
		return output, nil
	}

	result := &portllm.GenerateOutput{
		ResponseID:      "",
		Usage:           portllm.Usage{},
		ToolCalls:       make([]portllm.ToolCall, 0),
		ServerToolCalls: make([]portllm.ToolCall, 0),
	}
	idleTimeout := resolveStreamIdleTimeout(route.StreamIdleTimeoutMS)
	idleReader := newIdleTimeoutReader(resp.Body, idleTimeout)
	streamBody := newUpstreamBodyRecorder(idleReader)
	if err = consumeOpenAIImageStream(streamBody, outputFormat, result, onEvent); err != nil {
		return nil, attachUpstreamDebug(err, upstreamDebugSnapshot(req, debugPayload, resp, streamErrorBody(streamBody, err)))
	}
	return result, nil
}

func declaredImageMediaType(payload map[string]any) string {
	for _, key := range []string{"media_type", "mime_type"} {
		value := strings.ToLower(strings.TrimSpace(getString(payload[key])))
		if strings.HasPrefix(value, "image/") {
			return value
		}
	}
	return ""
}

func (c *Client) listModelsFromURL(ctx context.Context, route portllm.RouteConfig, requestURL string) ([]portllm.ModelItem, error) {
	if requestURL == "" {
		return nil, fmt.Errorf("invalid base url")
	}

	requestCtx, cancel := context.WithTimeout(ctx, resolveReadTimeout(route.ReadTimeoutMS))
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey := strings.TrimSpace(route.APIKey); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	setOpenRouterAttributionHeaders(req, route)
	setAdditionalHeaders(req, route.HeadersJSON)

	resp, err := c.doRouteRequest(route, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := readUpstreamBody(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseUpstreamError(resp.StatusCode, body, upstreamDebugSnapshot(req, nil, resp, body))
	}

	return parseOpenAIModelList(body)
}
