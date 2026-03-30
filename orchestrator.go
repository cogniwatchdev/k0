// Package agent — orchestrator.go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/k0-agent/k0/internal/config"
	"github.com/k0-agent/k0/internal/llm"
	"github.com/k0-agent/k0/internal/memory"
	"github.com/k0-agent/k0/internal/report"
)

// TaskUpdateMsg is sent to the TUI on each agent progress event.
type TaskUpdateMsg struct {
	AgentLabel string
	Line       string
	Timestamp  time.Time
}

// TaskDoneMsg is sent when all subagents for a goal have completed.
type TaskDoneMsg struct {
	GoalID  string
	Summary string
	Report  *report.Provisional
	Error   error
}

// Orchestrator coordinates goal decomposition and subagent lifecycle.
type Orchestrator struct {
	cfg     *config.Config
	llm     *llm.Client
	memory  *memory.Store
	updates chan interface{}
}

// NewOrchestrator creates a ready Orchestrator.
func NewOrchestrator(cfg *config.Config) *Orchestrator {
	return &Orchestrator{
		cfg:     cfg,
		llm:     llm.NewClient(cfg.OllamaAddr, cfg.Model),
		memory:  memory.NewStore(cfg),
		updates: make(chan interface{}, 128),
	}
}

// SubmitGoal launches goal processing asynchronously. Returns immediately.
// Updates stream back via ListenUpdates().
func (o *Orchestrator) SubmitGoal(rawGoal string) tea.Cmd {
	return func() tea.Msg {
		go o.runGoal(rawGoal)
		return nil
	}
}

// ListenUpdates returns a Cmd that blocks on the next update message.
// Must be re-armed by caller after every message received.
func (o *Orchestrator) ListenUpdates() tea.Cmd {
	return func() tea.Msg {
		return <-o.updates
	}
}

// PingLLM checks connectivity to the Ollama instance.
func (o *Orchestrator) PingLLM(ctx context.Context) error {
	return o.llm.Ping(ctx)
}

// runGoal is the blocking goal execution — runs in its own goroutine.
func (o *Orchestrator) runGoal(rawGoal string) {
	ctx := context.Background()
	goalID := uuid.New().String()[:8]

	o.emit(TaskUpdateMsg{
		AgentLabel: "[K-0]",
		Line:       fmt.Sprintf("Decomposing goal: %s", rawGoal),
		Timestamp:  time.Now(),
	})

	tasks, err := o.decomposeGoal(ctx, rawGoal)
	if err != nil {
		o.updates <- TaskDoneMsg{GoalID: goalID, Error: err}
		return
	}

	results := make(chan SubagentResult, len(tasks))
	for _, task := range tasks {
		go func(t Task) {
			sa := NewSubagent(t, o.cfg, o.llm, o.emit)
			results <- sa.Run(ctx)
		}(task)
	}

	var allResults []SubagentResult
	for range tasks {
		r := <-results
		allResults = append(allResults, r)
		if r.Err != nil {
			o.emit(TaskUpdateMsg{
				AgentLabel: fmt.Sprintf("[%s]", r.Label),
				Line:       fmt.Sprintf("⚠️  Error: %v", r.Err),
				Timestamp:  time.Now(),
			})
		}
	}

	ep := memory.Episode{
		ID:        goalID,
		Goal:      rawGoal,
		StartTime: time.Now(),
		Tasks:     len(tasks),
		Outcome:   "completed",
	}
	_ = o.memory.SaveEpisode(ep)

	prov, _ := report.Generate(ctx, o.llm, rawGoal, allResults)
	_ = o.memory.SaveReport(goalID, prov)

	o.updates <- TaskDoneMsg{
		GoalID:  goalID,
		Summary: prov.Summary,
		Report:  prov,
	}
}

func (o *Orchestrator) decomposeGoal(ctx context.Context, goal string) ([]Task, error) {
	raw, err := o.llm.DecomposeGoal(ctx, goal)
	if err != nil {
		// Fallback plan when LLM is unavailable
		return fallbackPlan(goal), nil
	}

	raw = cleanJSON(raw)
	var tasks []Task
	if err := json.Unmarshal([]byte(raw), &tasks); err != nil {
		o.emit(TaskUpdateMsg{
			AgentLabel: "[K-0]",
			Line:       fmt.Sprintf("⚠️  LLM returned bad JSON — using fallback plan"),
			Timestamp:  time.Now(),
		})
		return fallbackPlan(goal), nil
	}

	if len(tasks) == 0 || tasks[len(tasks)-1].Type != TaskTypeReport {
		tasks = append(tasks, Task{ID: "report-01", Type: TaskTypeReport, Label: "Report-01"})
	}
	return tasks, nil
}

func fallbackPlan(goal string) []Task {
	return []Task{
		{ID: "recon-01", Type: TaskTypeRecon, Label: "Recon-01", Goal: goal},
		{ID: "report-01", Type: TaskTypeReport, Label: "Report-01", Goal: goal},
	}
}

func cleanJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	for _, p := range []string{"```json\n", "```json", "```"} {
		if strings.HasPrefix(raw, p) {
			raw = strings.TrimSuffix(strings.TrimSpace(raw[len(p):]), "```")
			break
		}
	}
	return strings.TrimSpace(raw)
}

func (o *Orchestrator) emit(msg interface{}) {
	select {
	case o.updates <- msg:
	default:
	}
}
