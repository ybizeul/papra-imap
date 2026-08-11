package papra

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(host, apiKey string) *Client {
	baseURL := host
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) UploadDocument(ctx context.Context, orgID, filename string, content []byte) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	partHeader := make(textproto.MIMEHeader)
	disposition := mime.FormatMediaType("form-data", map[string]string{
		"name":     "file",
		"filename": filename,
	})
	if disposition == "" {
		return fmt.Errorf("build content disposition")
	}
	partContentType := contentTypeForFile(filename, content)
	partHeader.Set("Content-Disposition", disposition)
	partHeader.Set("Content-Type", partContentType)

	fw, err := w.CreatePart(partHeader)
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}
	if _, err := fw.Write(content); err != nil {
		return fmt.Errorf("write file content: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	url := fmt.Sprintf("%s/api/organizations/%s/documents", c.baseURL, orgID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	slog.Debug("papra request",
		"method", req.Method,
		"url", req.URL.String(),
		"filename", filename,
		"file_size", len(content),
		"file_ext", strings.ToLower(filepath.Ext(filename)),
		"part_content_type", partContentType,
		"sniffed_content_type", http.DetectContentType(content),
		"headers", map[string][]string{
			"Authorization": {"Bearer [redacted]"},
			"Content-Type":  req.Header.Values("Content-Type"),
		},
		"body_bytes", buf.Len(),
		"body_sha256", sha256Hex(buf.Bytes()),
	)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	attrs := []any{
		"method", req.Method,
		"url", req.URL.String(),
		"status", resp.StatusCode,
		"headers", resp.Header,
		"body_bytes", len(respBody),
		"body_sha256", sha256Hex(respBody),
	}
	if preview, ok := textBodyPreview(resp.Header.Get("Content-Type"), respBody); ok {
		attrs = append(attrs, "body_preview", preview)
	}
	slog.Debug("papra response", attrs...)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("papra API error %d: %s", resp.StatusCode, respBody)
	}

	return nil
}

func contentTypeForFile(filename string, content []byte) string {
	ext := strings.ToLower(filepath.Ext(filename))
	detected := ""
	if len(content) > 0 {
		detected = http.DetectContentType(content)
	}

	if ext == ".pdf" {
		return "application/pdf"
	}

	if detected != "" && detected != "application/octet-stream" {
		return detected
	}

	if fromExt := mime.TypeByExtension(ext); fromExt != "" {
		mediaType, _, err := mime.ParseMediaType(fromExt)
		if err == nil && mediaType != "" && mediaType != "application/octet-stream" {
			return mediaType
		}
		if fromExt != "application/octet-stream" {
			return fromExt
		}
	}

	if detected != "" {
		return detected
	}

	return "application/octet-stream"
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func textBodyPreview(contentType string, body []byte) (string, bool) {
	if len(body) == 0 {
		return "", false
	}

	ct := strings.ToLower(contentType)
	if !(strings.HasPrefix(ct, "text/") || strings.Contains(ct, "json") || strings.Contains(ct, "xml")) {
		return "", false
	}

	if !utf8.Valid(body) {
		return "", false
	}

	const maxPreview = 4096
	if len(body) <= maxPreview {
		return string(body), true
	}

	return string(body[:maxPreview]) + "... (truncated)", true
}
