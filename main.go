package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config holds CLI parameters and dependencies
type Config struct {
	Token     string
	BaseURL   string
	OutputDir string
	Verbose   bool
	Client    *http.Client
}

// Budget holds basic info from YNAB list endpoint
type Budget struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	LastModifiedOn time.Time `json:"last_modified_on"`
}

func main() {
	// CLI flags
	token := flag.String("token", "", "YNAB API bearer token (or set YNAB_BEARER_TOKEN env var)")
	output := flag.String("output", "budgets", "Directory to save budget JSON files")
	url := flag.String("url", "https://api.youneedabudget.com/v1/budgets", "Base API URL for budgets endpoint")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	// Resolve token
	tok := *token
	if tok == "" {
		tok = os.Getenv("YNAB_BEARER_TOKEN")
	}
	if tok == "" {
		fmt.Fprintln(os.Stderr, "Error: bearer token must be provided via --token or YNAB_BEARER_TOKEN env var")
		os.Exit(1)
	}

	cfg := Config{
		Token:     tok,
		BaseURL:   *url,
		OutputDir: *output,
		Verbose:   *verbose,
		Client:    http.DefaultClient,
	}

	if count, err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	} else if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "Processed %d budgets\n", count)
	}
}

// run orchestrates the fetch-and-save workflow and returns number of budgets processed
func run(cfg Config) (int, error) {
	if cfg.Verbose {
		fmt.Fprintln(os.Stderr, "Creating output directory", cfg.OutputDir)
	}
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create output dir: %w", err)
	}

	if cfg.Verbose {
		fmt.Fprintln(os.Stderr, "Fetching budgets list from", cfg.BaseURL)
	}
	budgets, err := fetchBudgets(cfg)
	if err != nil {
		return 0, fmt.Errorf("fetch budgets: %w", err)
	}

	count := 0
	for _, b := range budgets {
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Processing budget %s (%s)\n", b.Name, b.ID)
		}
		if path, err := downloadAndSave(cfg, b); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		} else if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Saved to %s\n", path)
		}
		count++
	}
	return count, nil
}

// fetchBudgets calls the YNAB API to list budgets and logs count if verbose
func fetchBudgets(cfg Config) ([]Budget, error) {
	data, err := httpGet(cfg.Client, cfg.BaseURL, cfg.Token)
	if err != nil {
		return nil, err
	}
	budgets, err := decodeBudgets(data)
	if err != nil {
		return nil, err
	}
	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "Fetched %d budgets\n", len(budgets))
	}
	return budgets, nil
}

// httpGet performs a GET request with bearer token and returns response body
func httpGet(client *http.Client, url, token string) (data []byte, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %d", resp.StatusCode)
		return
	}
	data, err = io.ReadAll(resp.Body)
	return
}

// decodeBudgets decodes a budgets list JSON
func decodeBudgets(data []byte) ([]Budget, error) {
	var wrapper struct {
		Data struct {
			Budgets []Budget `json:"budgets"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Data.Budgets, nil
}

// downloadAndSave fetches a single budget's JSON, writes to file, and returns the file path
func downloadAndSave(cfg Config, b Budget) (string, error) {
	url := fmt.Sprintf("%s/%s", cfg.BaseURL, b.ID)
	data, err := httpGet(cfg.Client, url, cfg.Token)
	if err != nil {
		return "", fmt.Errorf("download budget: %w", err)
	}
	filename := buildFilename(b)
	path := filepath.Join(cfg.OutputDir, filename)
	if err := writeFile(path, data); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}
	return path, nil
}

// writeFile writes data to a file with 0644 permissions
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

// buildFilename constructs a safe filename for a budget
func buildFilename(b Budget) string {
	safe := sanitizeFileName(b.Name)
	ts := b.LastModifiedOn.UTC().Format("20060102T150405Z")
	return fmt.Sprintf("%s_%s_%s.json", safe, b.ID, ts)
}

// sanitizeFileName replaces or removes unsupported characters
func sanitizeFileName(name string) string {
	s := strings.ReplaceAll(name, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune("_-+().A-Za-z0-9", r) || r >= '0' && r <= '9' ||
			r >= 'A' && r <= 'Z' ||
			r >= 'a' && r <= 'z' {
			return r
		}
		return -1
	}, s)
	return s
}
