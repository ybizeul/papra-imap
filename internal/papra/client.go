package papra

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
	"time"
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
	disposition, err := mime.FormatMediaType("form-data", map[string]string{
		"name":     "file",
		"filename": filename,
	})
	if err != nil {
		return fmt.Errorf("build content disposition: %w", err)
	}
	partHeader.Set("Content-Disposition", disposition)
	partHeader.Set("Content-Type", contentTypeForFile(filename, content))

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

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("papra API error %d: %s", resp.StatusCode, body)
	}

	return nil
}

func contentTypeForFile(filename string, content []byte) string {
	ext := strings.ToLower(filepath.Ext(filename))

	if ext == ".pdf" {
		return "application/pdf"
	}

	if fromExt := mime.TypeByExtension(ext); fromExt != "" {
		mediaType, _, err := mime.ParseMediaType(fromExt)
		if err == nil && mediaType != "" {
			return mediaType
		}
		return fromExt
	}

	if len(content) > 0 {
		return http.DetectContentType(content)
	}

	return "application/octet-stream"
}
