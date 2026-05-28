package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ExecuteAPICall performs an HTTP request defined by config.
func ExecuteAPICall(ctx context.Context, fc *Context, config map[string]any) (string, error) {
	rawURL, _ := config["url"].(string)
	method, _ := config["method"].(string)
	if method == "" {
		method = "GET"
	}
	bodyType, _ := config["bodyType"].(string)
	bodyStr, _ := config["body"].(string)
	hdrsRaw, _ := config["headers"].([]any)
	formDataRaw, _ := config["formData"].([]any)

	// Variable interpolation on URL + body
	rawURL = Interpolate(rawURL, fc.Variables)
	bodyStr = Interpolate(bodyStr, fc.Variables)

	// SSRF guard
	if err := CheckURL(rawURL, fc.AllowPrivateIPs); err != nil {
		return "", err
	}

	fc.Emit(fmt.Sprintf("API call: %s %s", method, rawURL))

	// Build body based on bodyType
	var body io.Reader
	var contentType string
	switch bodyType {
	case "json":
		var v any
		if err := json.Unmarshal([]byte(bodyStr), &v); err == nil {
			data, _ := json.Marshal(v)
			body = bytes.NewReader(data)
		} else {
			body = strings.NewReader(bodyStr) // raw fall-back
		}
		contentType = "application/json"
	case "form":
		form := url.Values{}
		for _, item := range formDataRaw {
			m, _ := item.(map[string]any)
			k, _ := m["key"].(string)
			v, _ := m["value"].(string)
			form.Add(k, Interpolate(v, fc.Variables))
		}
		body = strings.NewReader(form.Encode())
		contentType = "application/x-www-form-urlencoded"
	case "text":
		body = strings.NewReader(bodyStr)
		contentType = "text/plain"
	default:
		if bodyStr != "" {
			body = strings.NewReader(bodyStr)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return "", err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	// Forward headers
	for _, item := range hdrsRaw {
		m, _ := item.(map[string]any)
		k, _ := m["key"].(string)
		v, _ := m["value"].(string)
		req.Header.Set(k, Interpolate(v, fc.Variables))
	}

	resp, err := fc.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("api call: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 4 MiB cap

	if resp.StatusCode >= 400 {
		fc.Emit(fmt.Sprintf("API call failed: %d", resp.StatusCode))
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	return string(respBody), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
