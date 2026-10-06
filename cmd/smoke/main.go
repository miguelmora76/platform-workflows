// Command smoke sends one HTTP request to a deployed service and checks the response,
// retrying with backoff until it passes or runs out of attempts.
//
// Exit codes: 0 check passed, 1 check failed, 2 bad usage.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const maxBody = 1 << 20 // read at most 1 MiB of a response

// headers collects repeated --header "Name: value" flags.
type headers []string

func (h *headers) String() string     { return strings.Join(*h, ", ") }
func (h *headers) Set(v string) error { *h = append(*h, v); return nil }

type config struct {
	url            string
	method         string
	body           string
	headers        headers
	expectStatus   int
	expectContains string
	retries        int
	interval       time.Duration
	timeout        time.Duration
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseFlags(args, stderr)
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stderr, "smoke:", err)
		}
		return 2
	}

	client := &http.Client{Timeout: cfg.timeout}
	var lastErr error
	wait := cfg.interval
	for attempt := 1; attempt <= cfg.retries; attempt++ {
		lastErr = check(client, cfg)
		if lastErr == nil {
			fmt.Fprintf(stdout, "ok: %s %s (attempt %d/%d)\n", cfg.method, cfg.url, attempt, cfg.retries)
			return 0
		}
		fmt.Fprintf(stderr, "attempt %d/%d failed: %v\n", attempt, cfg.retries, lastErr)
		if attempt < cfg.retries {
			time.Sleep(wait)
			wait = min(wait*2, 10*time.Second)
		}
	}
	fmt.Fprintf(stderr, "FAIL: %s %s after %d attempts: %v\n", cfg.method, cfg.url, cfg.retries, lastErr)
	return 1
}

func parseFlags(args []string, stderr io.Writer) (config, error) {
	var cfg config
	fs := flag.NewFlagSet("smoke", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.url, "url", "", "URL to request (required)")
	fs.StringVar(&cfg.method, "method", "", "HTTP method (default GET, or POST when --body is set)")
	fs.StringVar(&cfg.body, "body", "", "request body")
	fs.Var(&cfg.headers, "header", `request header "Name: value" (repeatable)`)
	fs.IntVar(&cfg.expectStatus, "expect-status", 200, "required HTTP status")
	fs.StringVar(&cfg.expectContains, "expect-contains", "", "text the response body must contain")
	fs.IntVar(&cfg.retries, "retries", 5, "total attempts")
	fs.DurationVar(&cfg.interval, "interval", time.Second, "wait before the first retry; doubles each time, capped at 10s")
	fs.DurationVar(&cfg.timeout, "timeout", 5*time.Second, "timeout for each request")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if cfg.url == "" {
		return cfg, errors.New("--url is required")
	}
	if cfg.retries < 1 {
		return cfg, errors.New("--retries must be at least 1")
	}
	if cfg.method == "" {
		cfg.method = http.MethodGet
		if cfg.body != "" {
			cfg.method = http.MethodPost
		}
	}
	cfg.method = strings.ToUpper(cfg.method)
	for _, h := range cfg.headers {
		if name, _, ok := strings.Cut(h, ":"); !ok || strings.TrimSpace(name) == "" {
			return cfg, fmt.Errorf("invalid --header %q, want \"Name: value\"", h)
		}
	}
	return cfg, nil
}

// check makes one request and compares the response with what the config expects.
func check(client *http.Client, cfg config) error {
	req, err := http.NewRequest(cfg.method, cfg.url, strings.NewReader(cfg.body))
	if err != nil {
		return err
	}
	if cfg.body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, h := range cfg.headers {
		name, value, _ := strings.Cut(h, ":")
		req.Header.Set(strings.TrimSpace(name), strings.TrimSpace(value))
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != cfg.expectStatus {
		return fmt.Errorf("status %d, want %d", resp.StatusCode, cfg.expectStatus)
	}
	if cfg.expectContains != "" && !strings.Contains(string(body), cfg.expectContains) {
		return fmt.Errorf("response does not contain %q", cfg.expectContains)
	}
	return nil
}
