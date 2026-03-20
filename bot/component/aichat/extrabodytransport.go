package aichat

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

type extraBodyTransport struct {
	base      http.RoundTripper
	extraBody map[string]any
}

func (t *extraBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil && req.Header.Get("Content-Type") == "application/json" {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}

		var bodyMap map[string]any
		if err := json.Unmarshal(bodyBytes, &bodyMap); err != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			return t.base.RoundTrip(req)
		}

		for k, v := range t.extraBody {
			bodyMap[k] = v
		}

		newBody, err := json.Marshal(bodyMap)
		if err != nil {
			return nil, err
		}

		req.Body = io.NopCloser(bytes.NewReader(newBody))
		req.ContentLength = int64(len(newBody))
	}
	return t.base.RoundTrip(req)
}
