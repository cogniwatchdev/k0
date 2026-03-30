// Package llm provides the client for interacting with the local Ollama instance.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Client is the K-0 LLM client (talks to bundled Ollama).
type Client struct {
	addr       string
	model      string
	http       *http.Client
	soulPrompt string
}

// NewClient creates an LLM client pointed at addr with the given model.
func NewClient(addr, model string) *Client {
	c := &Client{
		addr:  addr,
		model: model,
		http: &http.Client{
			Timeout: 180 * time.Second,
		},
	}
	c.soulPrompt = loadSoul()
	return c
}

// ─── Soul loading ──────────────────────────────────────────────────────────

func loadSoul() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return defaultSoul
	}

	searchDirs := []string{
		filepath.Join(home, "k0", "agent", "soul"),
		filepath.Join(home, ".kiai", "soul"),
		"/usr/local/share/k0/soul",
	}

	// Only load persona + mindset for system prompt — the full knowledge base
	// (TOOLS, METASPLOIT, OSINT, OWASP, REPORTING) is too large to prepend to every call.
	// Those are used selectively in task-specific prompts.
	soulFiles := []string{"PERSONA.md", "MINDSET.md", "TRADECRAFT.md"}

	var parts []string
	for _, dir := range searchDirs {
		found := false
		for _, f := range soulFiles {
			data, err := os.ReadFile(filepath.Join(dir, f))
			if err != nil {
				continue
			}
			found = true
			parts = append(parts, strings.TrimSpace(string(data)))
		}
		if found {
			break
		}
	}

	if len(parts) == 0 {
		return defaultSoul
	}
	return strings.Join(parts, "\n\n---\n\n")
}

func (c *Client) buildSystem(taskInstruction string) string {
	if c.soulPrompt == "" {
		return taskInstruction
	}
	return c.soulPrompt + "\n\n---\n\n" + taskInstruction
}

// ─── Ollama API types ──────────────────────────────────────────────────────

type generateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	System  string         `json:"system,omitempty"`
	Format  string         `json:"format,omitempty"` // "json" for guided JSON output
	Options map[string]any `json:"options,omitempty"`
	Stream  bool           `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// ─── Core methods ──────────────────────────────────────────────────────────

// Complete sends a prompt and returns the raw text response.
func (c *Client) Complete(ctx context.Context, system, prompt string) (string, error) {
	return c.complete(ctx, system, prompt, "", 0.2, 2048)
}

// CompleteJSON sends a prompt with Ollama's format:"json" mode and lower token count.
func (c *Client) CompleteJSON(ctx context.Context, system, prompt string) (string, error) {
	return c.complete(ctx, system, prompt, "json", 0.1, 512)
}

func (c *Client) complete(ctx context.Context, system, prompt, format string, temp float64, maxTokens int) (string, error) {
	fullSystem := c.buildSystem(system)

	body, err := json.Marshal(generateRequest{
		Model:  c.model,
		Prompt: prompt,
		System: fullSystem,
		Format: format,
		Stream: false,
		Options: map[string]any{
			"temperature": temp,
			"num_predict": maxTokens,
		},
	})
	if err != nil {
		return "", fmt.Errorf("llm: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.addr+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("llm: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm: status %d: %s", resp.StatusCode, b)
	}

	var result generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("llm: decode: %w", err)
	}
	return result.Response, nil
}

// ─── Task decomposition ───────────────────────────────────────────────────

// DecomposeGoal asks the model to break a goal into a JSON task list.
func (c *Client) DecomposeGoal(ctx context.Context, goal string) (string, error) {
	system := `You are a task planning API. Output ONLY a JSON array of task objects.
Each task object must have exactly these fields:
  "id": string (e.g. "scan-01")
  "type": string, one of: recon, scan, web, exploit, report
  "label": string (e.g. "Scan-01")
  "params": object with "target" and "cmd" string fields

The "cmd" field should be the exact shell command to run (e.g. "nmap -sV -sC -T4 --open 127.0.0.1").
Always end the array with a report task: {"id":"report-01","type":"report","label":"Report-01","params":{}}

Available tools: nmap, nikto, whatweb, ffuf, gobuster, wpscan, sqlmap, hydra, searchsploit, enum4linux, smbclient, crackmapexec, subfinder, whois, dig

Output ONLY the JSON array. No explanation. No markdown fences.`

	prompt := fmt.Sprintf("Goal: %s", goal)

	raw, err := c.CompleteJSON(ctx, system, prompt)
	if err != nil {
		return "", err
	}
	// Extract JSON array from response (model may include prose around it)
	return extractJSONArray(raw), nil
}

// ExtractFindings analyses tool output and returns structured findings JSON.
func (c *Client) ExtractFindings(ctx context.Context, toolOutput, goal string) (string, error) {
	// Truncate output to avoid overwhelming the context
	if len(toolOutput) > 3000 {
		toolOutput = toolOutput[:3000] + "\n...[truncated]"
	}

	system := `You are a security findings extractor. Analyse the tool output and extract security findings.
Output a JSON array of finding objects. Each must have:
  "severity": one of CRITICAL, HIGH, MEDIUM, LOW, INFO
  "title": short finding title
  "description": what the finding means
  "target": the affected host/URL
  "evidence": the relevant line from the tool output
  "cve": CVE ID if known, empty string otherwise

If there are no security findings, output an empty array: []
Output ONLY the JSON array.`

	prompt := fmt.Sprintf("Tool output from goal '%s':\n\n%s", goal, toolOutput)

	raw, err := c.CompleteJSON(ctx, system, prompt)
	if err != nil {
		return "", err
	}
	return extractJSONArray(raw), nil
}

// SuggestNextSteps uses the LLM to generate recommended follow-up actions.
func (c *Client) SuggestNextSteps(ctx context.Context, goal string, findingsSummary string) ([]string, error) {
	system := `Based on the security assessment results, suggest 3-5 specific next steps.
Output a JSON array of strings. Each string is one actionable next step.
Example: ["Run wpscan for WordPress vulnerabilities","Check /admin for default credentials"]
Output ONLY the JSON array of strings.`

	prompt := fmt.Sprintf("Goal: %s\nFindings summary:\n%s", goal, findingsSummary)

	raw, err := c.CompleteJSON(ctx, system, prompt)
	if err != nil {
		return nil, err
	}

	raw = extractJSONArray(raw)
	var steps []string
	if err := json.Unmarshal([]byte(raw), &steps); err != nil {
		return []string{"Review findings and prioritise by severity."}, nil
	}
	return steps, nil
}

// Ping checks connectivity to the Ollama instance.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.addr+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ollama unreachable at %s: %w", c.addr, err)
	}
	resp.Body.Close()
	return nil
}

// ─── JSON extraction helpers ──────────────────────────────────────────────

// extractJSONArray scans the raw LLM output for the first [...] JSON array.
// Handles common model habits: prose before/after JSON, markdown fences, etc.
func extractJSONArray(raw string) string {
	raw = strings.TrimSpace(raw)

	// Strip markdown fences
	for _, prefix := range []string{"```json\n", "```json", "```\n", "```"} {
		if strings.HasPrefix(raw, prefix) {
			raw = strings.TrimPrefix(raw, prefix)
			if idx := strings.LastIndex(raw, "```"); idx >= 0 {
				raw = raw[:idx]
			}
			raw = strings.TrimSpace(raw)
			break
		}
	}

	// Find the first [ and last ] to extract the array
	start := strings.Index(raw, "[")
	if start == -1 {
		return "[]"
	}

	// Walk from start, track bracket depth to find matching ]
	depth := 0
	end := -1
	for i := start; i < len(raw); i++ {
		switch raw[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
		if end > 0 {
			break
		}
	}

	if end <= start {
		return "[]"
	}

	candidate := raw[start:end]

	// Quick validation — is it valid JSON?
	var check json.RawMessage
	if json.Unmarshal([]byte(candidate), &check) == nil {
		return candidate
	}

	return "[]"
}

// ─── Fallback soul ────────────────────────────────────────────────────────

const defaultSoul = `You are K-0, a tactical AI security agent running on Kali Linux.
You think like a red team operator: methodical, precise, scope-aware.
You enumerate before you exploit. You document everything.
You never touch out-of-scope targets.
You communicate in short, technical sentences.
When you find something, you say: what it is, what it means, what to do next.`
