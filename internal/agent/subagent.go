// Package agent — subagent.go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/k0-agent/k0/internal/config"
	"github.com/k0-agent/k0/internal/llm"
	"github.com/k0-agent/k0/internal/tools"
	"github.com/k0-agent/k0/internal/types"
)

type emitFn func(interface{})

// Subagent executes a single Task.
type Subagent struct {
	task     Task
	cfg      *config.Config
	executor *tools.Executor
	llm      *llm.Client
	emit     emitFn
}

// NewSubagent creates a Subagent for the given task.
func NewSubagent(task Task, cfg *config.Config, llmClient *llm.Client, emit emitFn) *Subagent {
	return &Subagent{
		task:     task,
		cfg:      cfg,
		executor: tools.NewExecutor(cfg),
		llm:      llmClient,
		emit:     emit,
	}
}

// Run executes the task and returns a SubagentResult.
func (sa *Subagent) Run(ctx context.Context) SubagentResult {
	result := SubagentResult{
		Task:  sa.task,
		Label: sa.task.Label,
	}

	sa.emit(TaskUpdateMsg{
		AgentLabel: fmt.Sprintf("[%s]", sa.task.Label),
		Line:       fmt.Sprintf("Starting %s task...", sa.task.Type),
		Timestamp:  time.Now(),
	})

	plan := sa.planTools()

	for _, step := range plan {
		sa.emit(TaskUpdateMsg{
			AgentLabel: fmt.Sprintf("[%s]", sa.task.Label),
			Line:       fmt.Sprintf("Running: %s %s", step.Tool, step.Args),
			Timestamp:  time.Now(),
		})

		out, code, err := sa.executor.Run(ctx, step.Tool, step.Args)
		tc := ToolCall{
			Tool:     step.Tool,
			Args:     step.Args,
			Output:   out,
			ExitCode: code,
			RunAt:    time.Now(),
		}
		result.ToolCalls = append(result.ToolCalls, tc)

		if err != nil {
			sa.emit(TaskUpdateMsg{
				AgentLabel: fmt.Sprintf("[%s]", sa.task.Label),
				Line:       fmt.Sprintf("⚠️  Tool error: %v", err),
				Timestamp:  time.Now(),
			})
			result.Err = err
			break
		}
		result.Output += out

		// Extract findings from tool output (skip for report tasks)
		if out != "" && sa.task.Type != TaskTypeReport {
			sa.extractAndEmitFindings(ctx, &result, out)
		}
	}

	return result
}

// extractAndEmitFindings calls the LLM to find security issues in tool output.
func (sa *Subagent) extractAndEmitFindings(ctx context.Context, result *SubagentResult, toolOutput string) {
	raw, err := sa.llm.ExtractFindings(ctx, toolOutput, sa.task.Goal)
	if err != nil {
		// Non-fatal — just skip findings extraction
		return
	}

	var findings []types.Finding
	if err := json.Unmarshal([]byte(raw), &findings); err != nil {
		return
	}

	result.Findings = append(result.Findings, findings...)

	for _, f := range findings {
		severity := string(f.Severity)
		sa.emit(TaskUpdateMsg{
			AgentLabel: fmt.Sprintf("[%s]", sa.task.Label),
			Line:       fmt.Sprintf("[%s] %s — %s", severity, f.Title, f.Target),
			Timestamp:  time.Now(),
		})
	}

	if len(findings) > 0 {
		sa.emit(TaskUpdateMsg{
			AgentLabel: fmt.Sprintf("[%s]", sa.task.Label),
			Line:       fmt.Sprintf("📋 Extracted %d finding(s)", len(findings)),
			Timestamp:  time.Now(),
		})
	}
}

type toolStep struct {
	Tool string
	Args string
}

// planTools determines what commands to run for this task.
// Priority: 1) Use LLM-provided cmd param  2) Fall back to hardcoded defaults
func (sa *Subagent) planTools() []toolStep {
	// If the LLM provided an explicit command, use it
	if cmd := sa.task.Params["cmd"]; cmd != "" {
		tool, args := parseCmd(cmd)
		if tool != "" {
			return []toolStep{{Tool: tool, Args: args}}
		}
	}

	// Fallback to sensible defaults per task type
	target := sa.task.Params["target"]
	if target == "" {
		target = "127.0.0.1"
	}

	switch sa.task.Type {
	case TaskTypeRecon:
		return []toolStep{
			{Tool: "nmap", Args: fmt.Sprintf("-sn %s", target)},
		}
	case TaskTypeScan:
		return []toolStep{
			{Tool: "nmap", Args: fmt.Sprintf("-sV -sC -T4 --open %s", target)},
		}
	case TaskTypeWeb:
		return []toolStep{
			{Tool: "whatweb", Args: fmt.Sprintf("-a 3 %s", target)},
			{Tool: "nikto", Args: fmt.Sprintf("-h %s", target)},
		}
	case TaskTypeSkill:
		// Skill tasks have cmd pre-built in detectSkillCommand
		if cmd := sa.task.Params["cmd"]; cmd != "" {
			tool, args := parseCmd(cmd)
			if tool != "" {
				return []toolStep{{Tool: tool, Args: args}}
			}
		}
		return nil
	default:
		return nil
	}
}

// parseCmd splits "nmap -sV -sC target" into tool="nmap", args="-sV -sC target"
func parseCmd(cmd string) (string, string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "", ""
	}
	parts := strings.SplitN(cmd, " ", 2)
	tool := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}
	return tool, args
}
