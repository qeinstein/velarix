package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "http://localhost:8080"

type cli struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	stdout     io.Writer
	stderr     io.Writer
}

type apiResponse struct {
	StatusCode  int
	ContentType string
	Body        []byte
}

func main() {
	cli := &cli{
		baseURL: strings.TrimRight(defaultString(os.Getenv("VELARIX_BASE_URL"), defaultBaseURL), "/"),
		apiKey:  strings.TrimSpace(os.Getenv("VELARIX_API_KEY")),
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
	if err := cli.run(os.Args[1:]); err != nil {
		fmt.Fprintln(cli.stderr, "Error:", err)
		os.Exit(1)
	}
}

func (c *cli) run(args []string) error {
	if len(args) == 0 {
		c.printUsage()
		return nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		c.printUsage()
		return nil
	case "status":
		return c.runStatus(args[1:])
	case "slice":
		return c.runSlice(args[1:])
	case "review":
		return c.runReview(args[1:])
	case "invalidate":
		return c.runInvalidate(args[1:])
	case "retract":
		return c.runRetract(args[1:])
	case "compliance-export":
		return c.runComplianceExport(args[1:])
	case "benchmark":
		return c.runBenchmark(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (c *cli) printUsage() {
	fmt.Fprintln(c.stdout, `Usage: vlx <command> [options]

Commands:
  status              Check server health
  slice               Fetch a query-aware belief slice
  review              Approve, waive, reject, or pend a fact review
  invalidate          Invalidate a root fact
  retract             Retract a fact with an optional force override
  compliance-export   Export org-level compliance/audit data
  benchmark           Run the reproducible contradiction benchmark harness

Environment:
  VELARIX_BASE_URL    API base URL (default: http://localhost:8080)
  VELARIX_API_KEY     Bearer token used for authenticated commands
  VELARIX_BENCHMARK_BINARY
                      Optional prebuilt server binary for benchmark --spawn-server`)
}

func (c *cli) runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	baseURL := fs.String("url", c.baseURL, "Velarix base URL")
	full := fs.Bool("full", false, "Fetch /health/full instead of /health")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path := "/health"
	if *full {
		path = "/health/full"
	}
	resp, err := c.doRequest(http.MethodGet, strings.TrimRight(*baseURL, "/")+path, "", nil)
	if err != nil {
		return err
	}
	return c.printResponse(resp, true)
}

func (c *cli) runSlice(args []string) error {
	fs := flag.NewFlagSet("slice", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	baseURL := fs.String("url", c.baseURL, "Velarix base URL")
	sessionID := fs.String("session", "", "Session ID")
	format := fs.String("format", "json", "Slice format: json or markdown")
	query := fs.String("query", "", "Query used to rank the slice")
	strategy := fs.String("strategy", "hybrid", "Selection strategy")
	maxFacts := fs.Int("max-facts", 50, "Maximum facts to include")
	maxChars := fs.Int("max-chars", 0, "Soft cap for markdown output size")
	includeDependencies := fs.Bool("include-dependencies", true, "Include supporting dependencies")
	includeInvalid := fs.Bool("include-invalid", false, "Include invalid facts in ranking/output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*sessionID) == "" {
		return errors.New("slice requires --session")
	}

	params := url.Values{}
	params.Set("format", *format)
	params.Set("strategy", *strategy)
	params.Set("max_facts", strconv.Itoa(*maxFacts))
	if strings.TrimSpace(*query) != "" {
		params.Set("query", strings.TrimSpace(*query))
	}
	if *maxChars > 0 {
		params.Set("max_chars", strconv.Itoa(*maxChars))
	}
	if *includeDependencies {
		params.Set("include_dependencies", "true")
	}
	if *includeInvalid {
		params.Set("include_invalid", "true")
	}

	endpoint := fmt.Sprintf("%s/v1/s/%s/slice?%s", strings.TrimRight(*baseURL, "/"), url.PathEscape(*sessionID), params.Encode())
	resp, err := c.doRequest(http.MethodGet, endpoint, "", nil)
	if err != nil {
		return err
	}
	return c.printResponse(resp, *format != "markdown")
}

func (c *cli) runReview(args []string) error {
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	baseURL := fs.String("url", c.baseURL, "Velarix base URL")
	sessionID := fs.String("session", "", "Session ID")
	factID := fs.String("fact", "", "Fact ID")
	status := fs.String("status", "", "Review status: pending|approved|waived|rejected")
	reason := fs.String("reason", "", "Review reason")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*sessionID) == "" || strings.TrimSpace(*factID) == "" || strings.TrimSpace(*status) == "" {
		return errors.New("review requires --session, --fact, and --status")
	}

	resp, err := c.doJSON(
		http.MethodPost,
		fmt.Sprintf("%s/v1/s/%s/facts/%s/review", strings.TrimRight(*baseURL, "/"), url.PathEscape(*sessionID), url.PathEscape(*factID)),
		map[string]interface{}{
			"status": strings.TrimSpace(*status),
			"reason": strings.TrimSpace(*reason),
		},
	)
	if err != nil {
		return err
	}
	return c.printResponse(resp, true)
}

func (c *cli) runInvalidate(args []string) error {
	return c.runFactMutation("invalidate", args)
}

func (c *cli) runRetract(args []string) error {
	return c.runFactMutation("retract", args)
}

func (c *cli) runFactMutation(kind string, args []string) error {
	fs := flag.NewFlagSet(kind, flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	baseURL := fs.String("url", c.baseURL, "Velarix base URL")
	sessionID := fs.String("session", "", "Session ID")
	factID := fs.String("fact", "", "Fact ID")
	reason := fs.String("reason", "", "Reason for the mutation")
	force := fs.Bool("force", false, "Bypass governance protection (admin only)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*sessionID) == "" || strings.TrimSpace(*factID) == "" {
		return fmt.Errorf("%s requires --session and --fact", kind)
	}

	body := map[string]interface{}{}
	if strings.TrimSpace(*reason) != "" {
		body["reason"] = strings.TrimSpace(*reason)
	}
	if *force {
		body["force"] = true
	}

	resp, err := c.doJSON(
		http.MethodPost,
		fmt.Sprintf("%s/v1/s/%s/facts/%s/%s", strings.TrimRight(*baseURL, "/"), url.PathEscape(*sessionID), url.PathEscape(*factID), kind),
		body,
	)
	if err != nil {
		return err
	}
	return c.printResponse(resp, true)
}

func (c *cli) runComplianceExport(args []string) error {
	fs := flag.NewFlagSet("compliance-export", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	baseURL := fs.String("url", c.baseURL, "Velarix base URL")
	format := fs.String("format", "json", "Export format: json or ndjson")
	limit := fs.Int("limit", 500, "Maximum org activity/log/session items to include")
	outputPath := fs.String("output", "", "Optional file path for export contents")
	if err := fs.Parse(args); err != nil {
		return err
	}

	params := url.Values{}
	params.Set("format", *format)
	params.Set("limit", strconv.Itoa(*limit))
	endpoint := fmt.Sprintf("%s/v1/org/compliance-export?%s", strings.TrimRight(*baseURL, "/"), params.Encode())
	resp, err := c.doRequest(http.MethodGet, endpoint, "", nil)
	if err != nil {
		return err
	}
	if *outputPath != "" {
		if err := os.WriteFile(*outputPath, resp.Body, 0644); err != nil {
			return err
		}
		fmt.Fprintf(c.stdout, "Wrote %s export to %s\n", *format, *outputPath)
		return nil
	}
	return c.printResponse(resp, *format == "json")
}

func (c *cli) runBenchmark(args []string) error {
	fs := flag.NewFlagSet("benchmark", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	steps := fs.Int("steps", 120, "Number of mission steps")
	contradictionInterval := fs.Int("contradiction-interval", 17, "How often to inject contradictions")
	spawnServer := fs.Bool("spawn-server", false, "Build or launch a local Velarix server for the TMS run")
	outputPath := fs.String("output", "", "Optional JSON output path")
	pythonBin := fs.String("python", "python3", "Python interpreter to use for the harness")
	if err := fs.Parse(args); err != nil {
		return err
	}

	scriptPath := filepath.Join("tests", "reproducibility", "hallucination_benchmark.py")
	cmdArgs := []string{
		scriptPath,
		"--steps", strconv.Itoa(*steps),
		"--contradiction-interval", strconv.Itoa(*contradictionInterval),
	}
	if *spawnServer {
		cmdArgs = append(cmdArgs, "--spawn-server")
	}
	if strings.TrimSpace(*outputPath) != "" {
		cmdArgs = append(cmdArgs, "--output", strings.TrimSpace(*outputPath))
	}

	cmd := exec.Command(*pythonBin, cmdArgs...)
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (c *cli) doJSON(method string, endpoint string, body map[string]interface{}) (*apiResponse, error) {
	var payload []byte
	if body == nil {
		body = map[string]interface{}{}
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	payload = encoded
	return c.doRequest(method, endpoint, "application/json", payload)
}

func (c *cli) doRequest(method string, endpoint string, contentType string, body []byte) (*apiResponse, error) {
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if strings.TrimSpace(c.apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.apiKey))
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s %s returned %d: %s", method, endpoint, resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	return &apiResponse{
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		Body:        payload,
	}, nil
}

func (c *cli) printResponse(resp *apiResponse, prettyJSON bool) error {
	if resp == nil {
		return nil
	}
	if !prettyJSON {
		_, err := c.stdout.Write(resp.Body)
		if err == nil && len(resp.Body) > 0 && resp.Body[len(resp.Body)-1] != '\n' {
			_, err = fmt.Fprintln(c.stdout)
		}
		return err
	}

	var body interface{}
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		_, writeErr := c.stdout.Write(resp.Body)
		if writeErr == nil && len(resp.Body) > 0 && resp.Body[len(resp.Body)-1] != '\n' {
			_, writeErr = fmt.Fprintln(c.stdout)
		}
		return writeErr
	}
	pretty, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(c.stdout, string(pretty))
	return err
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
