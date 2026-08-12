package papra

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestUploadDocument_AppliesConfiguredTags(t *testing.T) {
	var mu sync.Mutex
	calls := make([]string, 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/organizations/org_1/documents":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"document":{"id":"doc_1"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/organizations/org_1/tags":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tags":[{"id":"tag_1","name":"inbox"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/organizations/org_1/documents/doc_1/tags":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			defer r.Body.Close()

			var payload map[string]string
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if payload["tagId"] != "tag_1" {
				t.Fatalf("unexpected tagId: %q", payload["tagId"])
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	err := client.UploadDocument(context.Background(), "org_1", "invoice.pdf", []byte("pdf"), []string{"inbox"})
	if err != nil {
		t.Fatalf("UploadDocument returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 3 {
		t.Fatalf("unexpected number of calls: got %d, calls=%v", len(calls), calls)
	}
}

func TestUploadDocument_FailsWhenConfiguredTagDoesNotExist(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/organizations/org_1/documents":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"document":{"id":"doc_1"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/organizations/org_1/tags":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tags":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	err := client.UploadDocument(context.Background(), "org_1", "invoice.pdf", []byte("pdf"), []string{"inbox"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "configured tags not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUploadDocument_WithNoTags_DoesNotQueryTagsAPI(t *testing.T) {
	var listTagsCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/organizations/org_1/documents":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"document":{"id":"doc_1"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/organizations/org_1/tags":
			listTagsCalled = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tags":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	err := client.UploadDocument(context.Background(), "org_1", "invoice.pdf", []byte("pdf"), nil)
	if err != nil {
		t.Fatalf("UploadDocument returned error: %v", err)
	}
	if listTagsCalled {
		t.Fatalf("tags API should not be called when no tags are configured")
	}
}
