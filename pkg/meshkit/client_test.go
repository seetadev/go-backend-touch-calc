package meshkit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUploadFile(t *testing.T) {
	var gotPath, gotContent, gotMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if r.URL.Path != "/files/put" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		gotPath, gotContent = body.Path, body.Content
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"cid": "Cid123", "path": body.Path})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	cid, err := client.UploadFile(context.Background(), "notes/hello.txt", "Hello Meshkit!")
	if err != nil {
		t.Fatalf("UploadFile returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "notes/hello.txt" {
		t.Errorf("expected path notes/hello.txt, got %s", gotPath)
	}
	if gotContent != "Hello Meshkit!" {
		t.Errorf("expected content Hello Meshkit!, got %s", gotContent)
	}
	if cid != "Cid123" {
		t.Errorf("expected cid Cid123, got %s", cid)
	}
}

func TestUploadFileServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.UploadFile(context.Background(), "p", "c")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention status 500, got: %v", err)
	}
}

func TestUploadFileMissingCID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"path": "p"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if _, err := client.UploadFile(context.Background(), "p", "c"); err == nil {
		t.Fatal("expected error for missing cid in response, got nil")
	}
}

func TestGetFile(t *testing.T) {
	var gotPath, gotMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "Hello Meshkit!")
	}))
	defer server.Close()

	client := NewClient(server.URL)
	data, err := client.GetFile(context.Background(), "Cid123")
	if err != nil {
		t.Fatalf("GetFile returned error: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("expected GET, got %s", gotMethod)
	}
	if gotPath != "/files/Cid123" {
		t.Errorf("expected path /files/Cid123, got %s", gotPath)
	}
	if string(data) != "Hello Meshkit!" {
		t.Errorf("expected content Hello Meshkit!, got %q", string(data))
	}
}

func TestGetFileNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.GetFile(context.Background(), "Missing")
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error to mention status 404, got: %v", err)
	}
}

func TestHealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if err := client.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck returned error: %v", err)
	}
}

func TestHealthCheckUnhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "degraded", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if err := client.HealthCheck(context.Background()); err == nil {
		t.Fatal("expected error for 503 response, got nil")
	}
}

func TestNewClientDefaults(t *testing.T) {
	t.Setenv("MESHKIT_SERVICE_URL", "http://meshkit:3001")
	if got := NewClient("").baseURL; got != "http://meshkit:3001" {
		t.Errorf("expected env base URL, got %s", got)
	}

	t.Setenv("MESHKIT_SERVICE_URL", "")
	if got := NewClient("").baseURL; got != defaultBaseURL {
		t.Errorf("expected default base URL %s, got %s", defaultBaseURL, got)
	}
}

func TestNewClientTrailingSlash(t *testing.T) {
	if got := NewClient("http://localhost:3001/").baseURL; got != "http://localhost:3001" {
		t.Errorf("expected trailing slash trimmed, got %s", got)
	}
}
