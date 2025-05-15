package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSanitizeFileName ensures name cleaning matches expectations
func TestSanitizeFileName(t *testing.T) {
	tests := []struct{ input, want string }{
		{"My Budget/Name", "My_Budget_Name"},
		{"Budget:Special*Chars?", "BudgetSpecialChars"},
		{"  Leading and Trailing  ", "__Leading_and_Trailing__"},
		{"Complex-Name_123+()", "Complex-Name_123+()"},
	}
	for _, tc := range tests {
		got := sanitizeFileName(tc.input)
		if got != tc.want {
			t.Errorf("sanitizeFileName(%q) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

// TestBuildFilename checks timestamp formatting and filename structure
func TestBuildFilename(t *testing.T) {
	b := Budget{
		ID:             "abc123",
		Name:           "My Budget",
		LastModifiedOn: time.Date(2025, time.May, 14, 15, 30, 45, 0, time.UTC),
	}
	want := "My_Budget_abc123_20250514T153045Z.json"
	if got := buildFilename(b); got != want {
		t.Errorf("buildFilename() = %q; want %q", got, want)
	}
}

// TestFetchBudgets mocks the YNAB list endpoint to verify parsing
func TestFetchBudgets(t *testing.T) {
	// Prepare fake server
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer testtoken" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"data":{"budgets":[{"id":"1","name":"A","last_modified_on":"2025-05-14T10:00:00Z"},{"id":"2","name":"B","last_modified_on":"2025-05-14T11:00:00Z"}]}}`)
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	cfg := Config{Token: "testtoken", BaseURL: srv.URL, Client: srv.Client()}
	list, err := fetchBudgets(cfg)
	if err != nil {
		t.Fatalf("fetchBudgets error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 budgets; got %d", len(list))
	}
	if list[0].ID != "1" || list[0].Name != "A" {
		t.Errorf("first budget mismatch: %+v", list[0])
	}
}

// TestDownloadAndSave uses a temp dir and fake budget endpoint
func TestDownloadAndSave(t *testing.T) {
	// Fake budget JSON
	budgetJSON := `{"budget":{"id":"x","name":"X"}}`
	handler := func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/x") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, budgetJSON)
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	// Setup config with temp dir
	tmpDir := t.TempDir()
	b := Budget{ID: "x", Name: "X", LastModifiedOn: time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)}
	cfg := Config{Token: "tok", BaseURL: srv.URL, OutputDir: tmpDir, Client: srv.Client()}

	// Run download
	if err := downloadAndSave(cfg, b); err != nil {
		t.Fatalf("downloadAndSave error: %v", err)
	}

	// Verify file exists
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file; got %d", len(files))
	}
	fname := files[0].Name()
	// Should contain ID and timestamp
	if !strings.Contains(fname, "x_") || !strings.HasSuffix(fname, ".json") {
		t.Errorf("unexpected filename: %s", fname)
	}
	// Verify content
	data, err := os.ReadFile(filepath.Join(tmpDir, fname))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != budgetJSON {
		t.Errorf("file content mismatch: %s", data)
	}
}
