// Package agent — goal.go
// Type aliases pointing to the shared types package.
// Using aliases preserves backward compatibility with all existing agent code.
package agent

import "github.com/k0-agent/k0/internal/types"

// Type aliases — these ARE the types package types (not copies).
type Task = types.Task
type TaskType = types.TaskType
type SubagentResult = types.SubagentResult
type ToolCall = types.ToolCall
type Finding = types.Finding
type Severity = types.Severity

// TaskType constants.
const (
	TaskTypeRecon    = types.TaskTypeRecon
	TaskTypeScan     = types.TaskTypeScan
	TaskTypeWeb      = types.TaskTypeWeb
	TaskTypeWireless = types.TaskTypeWireless
	TaskTypeExploit  = types.TaskTypeExploit
	TaskTypeSkill    = types.TaskTypeSkill
	TaskTypeIntel    = types.TaskTypeIntel
	TaskTypeReport   = types.TaskTypeReport
)

// Severity constants.
const (
	SeverityCritical = types.SeverityCritical
	SeverityHigh     = types.SeverityHigh
	SeverityMedium   = types.SeverityMedium
	SeverityLow      = types.SeverityLow
	SeverityInfo     = types.SeverityInfo
)
