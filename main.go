package main

import (
	"encoding/json"
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

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// run orchestrates the fetch-and-save workflow
func run(cfg Config) error {
	if cfg.Verbose {
		fmt.Fprintln(os.Stderr, "Creating output directory", cfg.OutputDir)
	}
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	if cfg.Verbose {
		fmt.Fprintln(os.Stderr, "Fetching budgets list from", cfg.BaseURL)
	}
	budgets, err := fetchBudgets(cfg)
	if err != nil {
		return fmt.Errorf("fetch budgets: %w", err)
	}

	for _, b := range budgets {
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Processing budget %s (%s)\n", b.Name, b.ID)
		}
		if err := downloadAndSave(cfg, b); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}
	return nil
}

// fetchBudgets calls the YNAB API to list budgets
func fetchBudgets(cfg Config) ([]Budget, error) {
	req, err := http.NewRequest("GET", cfg.BaseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := cfg.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	var wrapper struct {
		Data struct {
			Budgets []Budget `json:"budgets"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, err
	}
	return wrapper.Data.Budgets, nil
}

// downloadAndSave fetches a single budget's JSON and writes to file
func downloadAndSave(cfg Config, b Budget) error {
	url := fmt.Sprintf("%s/%s", cfg.BaseURL, b.ID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := cfg.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download budget %s: status %d", b.ID, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	filename := buildFilename(b)
	path := filepath.Join(cfg.OutputDir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
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
