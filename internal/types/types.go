// Package types holds shared domain types used across agent, report, and memory packages.
// This package has zero internal dependencies to prevent import cycles.
package types

import "time"

// TaskType classifies the kind of work a subagent performs.
type TaskType string

const (
	TaskTypeRecon    TaskType = "recon"
	TaskTypeScan     TaskType = "scan"
	TaskTypeWeb      TaskType = "web"
	TaskTypeWireless TaskType = "wireless"
	TaskTypeExploit  TaskType = "exploit"
	TaskTypeReport   TaskType = "report"
)

// Task is a single unit of work decomposed from a user goal.
type Task struct {
	ID     string            `json:"id"`
	Type   TaskType          `json:"type"`
	Label  string            `json:"label"`
	Goal   string            `json:"goal,omitempty"`
	Params map[string]string `json:"params,omitempty"`
}

// SubagentResult is the output of a single subagent run.
type SubagentResult struct {
	Task      Task
	Label     string
	ToolCalls []ToolCall
	Output    string
	Findings  []Finding
	Err       error
}

// ToolCall records a single tool invocation.
type ToolCall struct {
	Tool     string    `json:"tool"`
	Args     string    `json:"args"`
	Output   string    `json:"output"`
	ExitCode int       `json:"exit_code"`
	RunAt    time.Time `json:"run_at"`
}

// Finding is a structured security finding from a subagent.
type Finding struct {
	Severity    Severity `json:"severity"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Target      string   `json:"target"`
	Evidence    string   `json:"evidence,omitempty"`
	CVE         string   `json:"cve,omitempty"`
}

// Severity levels for findings.
type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
	SeverityInfo     Severity = "INFO"
)
