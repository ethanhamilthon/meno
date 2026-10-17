package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// No overall Timeout: it would cut off long streaming responses. Requests end on
// completion or when the caller's context is canceled. Connection setup still
// uses the default transport's dial/TLS timeouts.
var httpClient = &http.Client{}

func post(ctx context.Context, p Provider, path string, body any, headers map[string]string) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, permanentError{err}
	}
	url := strings.TrimRight(p.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, permanentError{err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, &HTTPError{Status: resp.StatusCode, Body: strings.TrimSpace(string(msg)), RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	}
	return resp, nil
}

func NewClient(p Provider) Client {
	if p.Protocol == ProtoChat {
		return chatClient{}
	}
	return responsesClient{}
}
