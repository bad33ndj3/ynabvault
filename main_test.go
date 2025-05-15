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
		_, err := io.WriteString(w, `{"data":{"budgets":[{"id":"1","name":"A","last_modified_on":"2025-05-14T10:00:00Z"},{"id":"2","name":"B","last_modified_on":"2025-05-14T11:00:00Z"}]}}`)
		if err != nil {
			t.Fatalf("writing response: %v", err)
		}
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
		_, err := io.WriteString(w, budgetJSON)
		if err != nil {
			t.Fatalf("writing response: %v", err)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(handler))
	defer srv.Close()

	// Setup config with temp dir
	tmpDir := t.TempDir()
	b := Budget{ID: "x", Name: "X", LastModifiedOn: time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)}
	cfg := Config{Token: "tok", BaseURL: srv.URL, OutputDir: tmpDir, Client: srv.Client()}

	// Run download
	path, err := downloadAndSave(cfg, b)
	if err != nil {
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
	// Verify returned path matches
	if got := filepath.Base(path); got != fname {
		t.Errorf("returned path %q does not match file %q", got, fname)
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

// Optionally, add tests for new helpers if desired
func TestDecodeBudgets(t *testing.T) {
	jsonData := []byte(`{"data":{"budgets":[{"id":"1","name":"A","last_modified_on":"2025-05-14T10:00:00Z"}]}}`)
	budgets, err := decodeBudgets(jsonData)
	if err != nil {
		t.Fatalf("decodeBudgets error: %v", err)
	}
	if len(budgets) != 1 || budgets[0].ID != "1" {
		t.Errorf("unexpected decodeBudgets result: %+v", budgets)
	}
}

// TestHttpGet verifies proper handling of various HTTP scenarios
func TestHttpGet(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
	}{
		{"success", http.StatusOK, "test data", false},
		{"unauthorized", http.StatusUnauthorized, "", true},
		{"server error", http.StatusInternalServerError, "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify auth header is set
				if r.Header.Get("Authorization") != "Bearer testtoken" {
					t.Errorf("Expected Authorization header with bearer token")
				}
				w.WriteHeader(tc.statusCode)
				_, err := w.Write([]byte(tc.body))
				if err != nil {
					t.Fatalf("writing response: %v", err)
				}
			}))
			defer server.Close()

			data, err := httpGet(server.Client(), server.URL, "testtoken")

			if tc.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if string(data) != tc.body {
					t.Errorf("Expected body %q, got %q", tc.body, string(data))
				}
			}
		})
	}
}

// TestDecodeBudgetsError verifies error handling with malformed JSON
func TestDecodeBudgetsError(t *testing.T) {
	invalidJSON := []byte(`{"data":{"budgets":[{"id":1,"name`)
	_, err := decodeBudgets(invalidJSON)
	if err == nil {
		t.Error("Expected error with invalid JSON but got nil")
	}
}

// TestWriteFile verifies file writing functionality
func TestWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "test.json")
	testData := []byte(`{"test":"data"}`)

	err := writeFile(testPath, testData)
	if err != nil {
		t.Fatalf("writeFile error: %v", err)
	}

	// Verify content was written correctly
	content, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("Expected file content %q, got %q", testData, content)
	}

	// Test write error with bad path
	badPath := filepath.Join(os.DevNull, "impossible.txt")
	err = writeFile(badPath, testData)
	if err == nil {
		t.Error("Expected error writing to invalid path but got nil")
	}
}

// TestDownloadAndSaveError checks error handling in downloadAndSave
func TestDownloadAndSaveError(t *testing.T) {
	// Server that always returns an error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	b := Budget{ID: "x", Name: "X", LastModifiedOn: time.Now()}
	cfg := Config{Token: "tok", BaseURL: srv.URL, OutputDir: tmpDir, Client: srv.Client()}

	_, err := downloadAndSave(cfg, b)
	if err == nil {
		t.Error("Expected error from downloadAndSave but got nil")
	}
}

// TestFetchBudgetsError verifies error handling in fetchBudgets
func TestFetchBudgetsError(t *testing.T) {
	// Server that returns invalid JSON
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"data":{"budgets":[{"invalid`)) // Malformed JSON
		if err != nil {
			t.Fatalf("writing response: %v", err)
		}
	}))
	defer srv.Close()

	cfg := Config{Token: "testtoken", BaseURL: srv.URL, Client: srv.Client()}
	_, err := fetchBudgets(cfg)
	if err == nil {
		t.Error("Expected error with invalid JSON but got nil")
	}
}

// TestRun verifies the main orchestration function
func TestRun(t *testing.T) {
	// Mock server that returns a list of budgets and then budget details
	var requestCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")

		// First request: budgets list
		if requestCount == 1 {
			_, err := io.WriteString(w, `{"data":{"budgets":[{"id":"test1","name":"Budget1","last_modified_on":"2025-01-01T00:00:00Z"}]}}`)
			if err != nil {
				t.Fatalf("writing response: %v", err)
			}
			return
		}

		// Budget detail requests
		_, err := io.WriteString(w, `{"budget":{"name":"Budget1","id":"test1"}}`)
		if err != nil {
			t.Fatalf("writing response: %v", err)
		}
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	cfg := Config{
		Token:     "testtoken",
		BaseURL:   srv.URL,
		OutputDir: tmpDir,
		Verbose:   true,
		Client:    srv.Client(),
	}

	count, err := run(cfg)
	if err != nil {
		t.Fatalf("run() error: %v", err)
	}

	// Verify count is correct
	if count != 1 {
		t.Errorf("Expected to process 1 budget, got %d", count)
	}

	// Verify file was created
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Error reading output dir: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("Expected 1 file in output dir, got %d", len(files))
	}
}
