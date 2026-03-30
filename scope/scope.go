// Package scope provides in-scope/out-of-scope enforcement for K-0 engagements.
package scope

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Scope defines what targets K-0 is allowed to touch.
type Scope struct {
	Engagement   string   `json:"engagement"`
	AuthorisedBy string   `json:"authorised_by"`
	InScope      []string `json:"in_scope"`
	OutOfScope   []string `json:"out_of_scope"`
	Expires      string   `json:"expires"` // YYYY-MM-DD
}

// Load reads scope.json from the data directory. Returns nil, nil if no scope file exists
// (no scope = everything allowed — for lab use).
func Load(dataDir string) (*Scope, error) {
	path := filepath.Join(dataDir, "scope.json")
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scope: read: %w", err)
	}
	var s Scope
	return &s, json.Unmarshal(b, &s)
}

// Check validates whether a target is in scope.
// Returns (allowed bool, reason string).
func (s *Scope) Check(target string) (bool, string) {
	if s == nil {
		return true, "" // no scope file = lab mode, everything allowed
	}

	// Expired scope = block everything
	if s.Expires != "" {
		exp, err := time.Parse("2006-01-02", s.Expires)
		if err == nil && time.Now().After(exp) {
			return false, fmt.Sprintf("scope expired on %s", s.Expires)
		}
	}

	// Always allow loopback
	if target == "127.0.0.1" || target == "localhost" || target == "::1" {
		return true, ""
	}

	// Check out-of-scope first (takes priority)
	for _, oos := range s.OutOfScope {
		if matchTarget(target, oos) {
			return false, fmt.Sprintf("%q is explicitly out of scope", target)
		}
	}

	// If in_scope is defined, target must be in it
	if len(s.InScope) > 0 {
		for _, in := range s.InScope {
			if matchTarget(target, in) {
				return true, ""
			}
		}
		return false, fmt.Sprintf("%q not in in_scope list", target)
	}

	return true, ""
}

// matchTarget checks if target matches a scope pattern.
// Supports: exact match, CIDR (10.0.0.0/24), wildcard domain (*.example.com)
func matchTarget(target, pattern string) bool {
	// CIDR match
	if strings.Contains(pattern, "/") {
		if _, n, err := net.ParseCIDR(pattern); err == nil {
			if ip := net.ParseIP(target); ip != nil {
				return n.Contains(ip)
			}
		}
	}

	// Wildcard domain: *.example.com matches sub.example.com and example.com
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		return strings.HasSuffix(target, suffix) || target == pattern[2:]
	}

	// Exact match
	return target == pattern
}
