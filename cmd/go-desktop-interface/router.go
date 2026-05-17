package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
)

type response struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
	Code  int         `json:"code,omitempty"`
}

func handleRequest(method, path string, body []byte) response {
	if globalServer == nil {
		return response{OK: false, Error: "server not initialized", Code: 503}
	}

	rec := httptest.NewRecorder()
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	// Use chi router directly — zero changes to api/*.go
	globalServer.Router().ServeHTTP(rec, req)

	// Parse response
	contentType := rec.Header().Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var result map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			return response{OK: true, Data: map[string]string{"raw": rec.Body.String()}}
		}
		if rec.Code >= 400 {
			errMsg := "request failed"
			if msg, ok := result["error"].(string); ok && msg != "" {
				errMsg = msg
			}
			return response{OK: false, Error: errMsg, Code: rec.Code}
		}
		return response{OK: true, Data: result}
	}

	// Non-JSON response (e.g., raw text, HTML)
	return response{OK: true, Data: map[string]string{"raw": rec.Body.String()}}
}
