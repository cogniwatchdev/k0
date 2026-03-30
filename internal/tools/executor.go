// Package tools - executor.go
// Executes Kali tool invocations safely via subprocess.
// Only tools on the allowlist can be run. Scope enforcement is applied.
package tools

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/k0-agent/k0/internal/config"
	"github.com/k0-agent/k0/internal/scope"
)

// Executor runs allowed Kali tools as subprocesses.
type Executor struct {
	cfg       *config.Config
	allowlist AllowList
	scope     *scope.Scope
}

// NewExecutor creates an Executor with the default Kali allowlist and scope.
func NewExecutor(cfg *config.Config) *Executor {
	// Load scope (nil = lab mode, everything allowed)
	sc, _ := scope.Load(cfg.MemoryPath + "/../")

	// Also try standard location
	if sc == nil {
		sc, _ = scope.Load(cfg.MemoryPath + "/../../.kiai")
	}

	return &Executor{
		cfg:       cfg,
		allowlist: DefaultAllowList(),
		scope:     sc,
	}
}

// Run executes a tool with args, captures output, enforces allowlist + scope.
// Returns: stdout+stderr combined, exit code, error.
func (e *Executor) Run(ctx context.Context, tool, args string) (string, int, error) {
	// Enforce allowlist
	if !e.allowlist.Allowed(tool) {
		return "", -1, fmt.Errorf("tool %q is not in the K-0 allowlist", tool)
	}

	// Scope check
	if e.scope != nil {
		if tgt := extractTarget(tool, args); tgt != "" {
			if ok, reason := e.scope.Check(tgt); !ok {
				return "", -1, fmt.Errorf("SCOPE_VIOLATION: %s", reason)
			}
		}
	}

	// Build command with per-tool timeout
	argv := append([]string{tool}, strings.Fields(args)...)
	toolCtx, cancel := context.WithTimeout(ctx, e.allowlist.Timeout(tool))
	defer cancel()
	cmd := exec.CommandContext(toolCtx, argv[0], argv[1:]...)

	// Capture output
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			return string(out), -1, fmt.Errorf("exec %s: %w", tool, err)
		}
	}

	return string(out), code, nil
}

// extractTarget attempts to extract the target hostname/IP from tool arguments.
// Returns empty string if unable to determine (safe: allows execution).
func extractTarget(tool, args string) string {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return ""
	}

	switch tool {
	case "nmap", "masscan", "nikto", "whois":
		// Target is typically the last non-flag argument
		return lastNonFlag(fields)

	case "ffuf", "gobuster", "feroxbuster":
		// Target comes after -u flag
		for i, f := range fields {
			if (f == "-u" || f == "--url") && i+1 < len(fields) {
				return hostnameFromURL(fields[i+1])
			}
		}

	case "whatweb", "curl", "wget":
		// First argument that looks like a URL or hostname
		for _, f := range fields {
			if strings.HasPrefix(f, "http") {
				return hostnameFromURL(f)
			}
			if !strings.HasPrefix(f, "-") {
				return f
			}
		}

	case "sqlmap":
		for i, f := range fields {
			if (f == "-u" || f == "--url") && i+1 < len(fields) {
				return hostnameFromURL(fields[i+1])
			}
		}

	case "hydra":
		// Last argument is usually the target
		return lastNonFlag(fields)

	case "wpscan":
		for i, f := range fields {
			if f == "--url" && i+1 < len(fields) {
				return hostnameFromURL(fields[i+1])
			}
		}

	case "smbclient", "enum4linux", "crackmapexec":
		return lastNonFlag(fields)
	}

	return ""
}

func lastNonFlag(fields []string) string {
	for i := len(fields) - 1; i >= 0; i-- {
		if !strings.HasPrefix(fields[i], "-") {
			return fields[i]
		}
	}
	return ""
}

func hostnameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host := u.Hostname()
	if host != "" {
		return host
	}
	return rawURL
}

// ─────────────────────────────────────────────────────────────────────────
// AllowList — the set of tools K-0 is permitted to run.
// ─────────────────────────────────────────────────────────────────────────

type AllowList map[string]ToolConfig

type ToolConfig struct {
	MaxRuntime   time.Duration
	RequiresRoot bool
	AutoApprove  bool
}

func (al AllowList) Allowed(tool string) bool {
	_, ok := al[tool]
	return ok
}

func (al AllowList) Timeout(tool string) time.Duration {
	if cfg, ok := al[tool]; ok && cfg.MaxRuntime > 0 {
		return cfg.MaxRuntime
	}
	return 5 * time.Minute
}

// DefaultAllowList returns the pre-approved Kali tool allowlist.
func DefaultAllowList() AllowList {
	m := 60 * time.Minute
	return AllowList{
		// Recon
		"nmap":        {MaxRuntime: m, AutoApprove: true},
		"assetfinder": {MaxRuntime: 10 * time.Minute, AutoApprove: true},
		"subfinder":   {MaxRuntime: 10 * time.Minute, AutoApprove: true},
		"amass":       {MaxRuntime: 30 * time.Minute, AutoApprove: true},
		"theHarvester":{MaxRuntime: 10 * time.Minute, AutoApprove: true},
		"whois":       {MaxRuntime: 30 * time.Second, AutoApprove: true},
		"dig":         {MaxRuntime: 30 * time.Second, AutoApprove: true},
		"masscan":     {MaxRuntime: m, AutoApprove: true},
		// Web
		"nikto":       {MaxRuntime: m, AutoApprove: true},
		"ffuf":        {MaxRuntime: m, AutoApprove: true},
		"gobuster":    {MaxRuntime: m, AutoApprove: true},
		"feroxbuster": {MaxRuntime: m, AutoApprove: true},
		"wpscan":      {MaxRuntime: m, AutoApprove: true},
		"whatweb":     {MaxRuntime: 5 * time.Minute, AutoApprove: true},
		"sqlmap":      {MaxRuntime: m, AutoApprove: false},
		// Network
		"netcat":      {MaxRuntime: 5 * time.Minute, AutoApprove: true},
		"nc":          {MaxRuntime: 5 * time.Minute, AutoApprove: true},
		"curl":        {MaxRuntime: 2 * time.Minute, AutoApprove: true},
		"wget":        {MaxRuntime: 5 * time.Minute, AutoApprove: true},
		// Password / Creds
		"hydra":       {MaxRuntime: m, AutoApprove: false},
		"medusa":      {MaxRuntime: m, AutoApprove: false},
		"hashcat":     {MaxRuntime: 2 * m, AutoApprove: false},
		"john":        {MaxRuntime: 2 * m, AutoApprove: false},
		// Wireless
		"airodump-ng": {MaxRuntime: m, RequiresRoot: true, AutoApprove: false},
		"aircrack-ng": {MaxRuntime: 2 * m, RequiresRoot: true, AutoApprove: false},
		// Exploitation
		"msfconsole":  {MaxRuntime: 2 * m, AutoApprove: false},
		"msfvenom":    {MaxRuntime: 5 * time.Minute, AutoApprove: false},
		// Misc
		"searchsploit":{MaxRuntime: 30 * time.Second, AutoApprove: true},
		"enum4linux":  {MaxRuntime: 10 * time.Minute, AutoApprove: true},
		"smbclient":   {MaxRuntime: 5 * time.Minute, AutoApprove: true},
		"crackmapexec":{MaxRuntime: m, AutoApprove: false},
	}
}
