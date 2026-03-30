// Package agent — orchestrator.go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
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

// PlanPhase represents one step in a proposed plan.
type PlanPhase struct {
	ID       int    `json:"id"`
	Tool     string `json:"tool"`
	Command  string `json:"command"`
	Purpose  string `json:"purpose"`
	Estimate string `json:"estimate"`
}

// ToolStatus tracks whether a tool is available.
type ToolStatus struct {
	Tool      string
	Available bool
}

// PlanProposal is the structured plan from the LLM.
type PlanProposal struct {
	Goal          string       `json:"-"`
	Scope         string       `json:"scope"`
	Approach      string       `json:"approach"`
	Phases        []PlanPhase  `json:"phases"`
	TotalEstimate string       `json:"total_estimate"`
	Risks         string       `json:"risks"`
	ToolChecks    []ToolStatus `json:"-"` // populated after tool verification
	MissingTools  []string     `json:"-"` // tools that need installing
}

// PlanProposalMsg is sent to the TUI when a plan is ready for confirmation.
type PlanProposalMsg struct {
	Plan PlanProposal
}

// ToolInstallRequestMsg asks the user to approve installing missing tools.
type ToolInstallRequestMsg struct {
	Tools []string
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

// SubmitGoal checks if this is a direct command or needs planning.
func (o *Orchestrator) SubmitGoal(rawGoal string) tea.Cmd {
	if tasks := o.detectDirectCommand(rawGoal); tasks != nil {
		return func() tea.Msg {
			go o.executeTasks(rawGoal, tasks)
			return nil
		}
	}
	return func() tea.Msg {
		go o.planGoal(rawGoal)
		return nil
	}
}

// ExecuteApprovedPlan runs a plan the user has confirmed.
func (o *Orchestrator) ExecuteApprovedPlan(plan PlanProposal) tea.Cmd {
	return func() tea.Msg {
		go o.executePlan(plan)
		return nil
	}
}

// InstallTools installs missing tools via apt-get.
func (o *Orchestrator) InstallTools(tools []string) tea.Cmd {
	return func() tea.Msg {
		go func() {
			for _, tool := range tools {
				o.emit(TaskUpdateMsg{
					AgentLabel: "[K-0]",
					Line:       fmt.Sprintf("Installing %s...", tool),
					Timestamp:  time.Now(),
				})
				cmd := exec.Command("sudo", "apt-get", "install", "-y", tool)
				out, err := cmd.CombinedOutput()
				if err != nil {
					o.emit(TaskUpdateMsg{
						AgentLabel: "[K-0]",
						Line:       fmt.Sprintf("⚠️  Failed to install %s: %v", tool, err),
						Timestamp:  time.Now(),
					})
				} else {
					_ = out
					o.emit(TaskUpdateMsg{
						AgentLabel: "[K-0]",
						Line:       fmt.Sprintf("✓ %s installed", tool),
						Timestamp:  time.Now(),
					})
				}
			}
		}()
		return nil
	}
}

// ListenUpdates returns a Cmd that blocks on the next update message.
func (o *Orchestrator) ListenUpdates() tea.Cmd {
	return func() tea.Msg {
		return <-o.updates
	}
}

// PingLLM checks connectivity to the Ollama instance.
func (o *Orchestrator) PingLLM(ctx context.Context) error {
	return o.llm.Ping(ctx)
}

// ── Planning ──────────────────────────────────────────────────────────────

func (o *Orchestrator) planGoal(rawGoal string) {
	o.emit(TaskUpdateMsg{
		AgentLabel: "[K-0]",
		Line:       fmt.Sprintf("Analysing scope: %s", rawGoal),
		Timestamp:  time.Now(),
	})

	target := extractTargetFromGoal(rawGoal)

	// 1. Try to match a known plan template (instant, no LLM needed)
	if plan := o.matchTemplate(rawGoal, target); plan != nil {
		o.emit(TaskUpdateMsg{
			AgentLabel: "[K-0]",
			Line:       "Matched known scan pattern — plan generated instantly",
			Timestamp:  time.Now(),
		})
		plan.Goal = rawGoal
		o.verifyToolAvailability(plan)
		o.updates <- PlanProposalMsg{Plan: *plan}
		return
	}

	// 2. No template match — try LLM planning
	ctx := context.Background()
	o.emit(TaskUpdateMsg{
		AgentLabel: "[K-0]",
		Line:       "Generating custom plan via AI...",
		Timestamp:  time.Now(),
	})

	raw, err := o.llm.PlanGoal(ctx, rawGoal)
	if err != nil {
		o.emit(TaskUpdateMsg{
			AgentLabel: "[K-0]",
			Line:       "⚠️  LLM timed out — using basic recon plan",
			Timestamp:  time.Now(),
		})
		plan := fallbackPlan2(rawGoal)
		o.verifyToolAvailability(&plan)
		o.updates <- PlanProposalMsg{Plan: plan}
		return
	}

	var plan PlanProposal
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		o.emit(TaskUpdateMsg{
			AgentLabel: "[K-0]",
			Line:       "⚠️  Could not parse LLM plan — using basic recon plan",
			Timestamp:  time.Now(),
		})
		plan = fallbackPlan2(rawGoal)
		o.verifyToolAvailability(&plan)
		o.updates <- PlanProposalMsg{Plan: plan}
		return
	}

	if len(plan.Phases) == 0 {
		plan = fallbackPlan2(rawGoal)
	}
	plan.Goal = rawGoal
	o.verifyToolAvailability(&plan)
	o.updates <- PlanProposalMsg{Plan: plan}
}

// matchTemplate checks whether the goal matches known pentest patterns
// and returns a pre-built plan. Returns nil if no match.
func (o *Orchestrator) matchTemplate(goal, target string) *PlanProposal {
	gl := strings.ToLower(goal)

	// Web vulnerability scan
	if containsAny(gl, "web vuln", "web scan", "website scan", "http scan", "web vulnerabilit", "owasp") {
		return &PlanProposal{
			Scope:    target,
			Approach: "Port scan → Service detection → Web technology fingerprint → Vulnerability scan → Report",
			Phases: []PlanPhase{
				{ID: 1, Tool: "nmap", Command: fmt.Sprintf("nmap -sV -sC -T4 --open -p 80,443,8080,8443 %s", target), Purpose: "Web port discovery and service detection", Estimate: "~30s"},
				{ID: 2, Tool: "whatweb", Command: fmt.Sprintf("whatweb -a 3 %s", target), Purpose: "Web technology fingerprinting", Estimate: "~15s"},
				{ID: 3, Tool: "nikto", Command: fmt.Sprintf("nikto -h %s", target), Purpose: "Web vulnerability scan (misconfigs, defaults, known CVEs)", Estimate: "~2-5min"},
				{ID: 4, Tool: "report", Command: "generate report", Purpose: "Compile findings into provisional report", Estimate: "~30s"},
			},
			TotalEstimate: "~3-6 minutes",
			Risks:         "Active scanning — target will see connection attempts in logs",
		}
	}

	// Full port scan
	if containsAny(gl, "full scan", "full port", "all ports", "comprehensive scan", "thorough scan") {
		return &PlanProposal{
			Scope:    target,
			Approach: "Full TCP scan → Service detection → Script scan → Report",
			Phases: []PlanPhase{
				{ID: 1, Tool: "nmap", Command: fmt.Sprintf("nmap -sV -sC -T4 -p- --open %s", target), Purpose: "Full 65535-port TCP scan with version detection", Estimate: "~5-15min"},
				{ID: 2, Tool: "report", Command: "generate report", Purpose: "Compile findings into provisional report", Estimate: "~30s"},
			},
			TotalEstimate: "~5-16 minutes",
			Risks:         "Full port scan is noisy — IDS/IPS may trigger alerts",
		}
	}

	// DNS / OSINT recon (check BEFORE generic recon — "dns recon" should match here)
	if containsAny(gl, "dns", "subdomain", "domain recon", "whois", "osint") {
		return &PlanProposal{
			Scope:    target,
			Approach: "WHOIS lookup → DNS enumeration → Subdomain discovery → Report",
			Phases: []PlanPhase{
				{ID: 1, Tool: "whois", Command: fmt.Sprintf("whois %s", target), Purpose: "Domain registration and ownership data", Estimate: "~5s"},
				{ID: 2, Tool: "dig", Command: fmt.Sprintf("dig ANY %s", target), Purpose: "DNS record enumeration", Estimate: "~5s"},
				{ID: 3, Tool: "subfinder", Command: fmt.Sprintf("subfinder -d %s -silent", target), Purpose: "Subdomain discovery via passive sources", Estimate: "~30s"},
				{ID: 4, Tool: "report", Command: "generate report", Purpose: "Compile findings into provisional report", Estimate: "~30s"},
			},
			TotalEstimate: "~1-2 minutes",
			Risks:         "Passive only — no active connections to target infrastructure",
		}
	}

	// Network discovery / recon
	if containsAny(gl, "recon", "discover", "network scan", "host discover", "ping sweep", "find hosts", "enumerate") {
		return &PlanProposal{
			Scope:    target,
			Approach: "Host discovery → Service enumeration → OS detection → Report",
			Phases: []PlanPhase{
				{ID: 1, Tool: "nmap", Command: fmt.Sprintf("nmap -sn %s", target), Purpose: "Host discovery (ping sweep)", Estimate: "~15-30s"},
				{ID: 2, Tool: "nmap", Command: fmt.Sprintf("nmap -sV -sC -O -T4 --open %s", target), Purpose: "Service and OS detection on discovered hosts", Estimate: "~2-5min"},
				{ID: 3, Tool: "report", Command: "generate report", Purpose: "Compile findings into provisional report", Estimate: "~30s"},
			},
			TotalEstimate: "~3-6 minutes",
			Risks:         "OS detection (-O) requires raw sockets — may need root/sudo",
		}
	}


	// SMB/Windows enum
	if containsAny(gl, "smb", "windows", "shares", "netbios", "active directory", "ad enum") {
		return &PlanProposal{
			Scope:    target,
			Approach: "SMB enumeration → Share discovery → Vulnerability scan → Report",
			Phases: []PlanPhase{
				{ID: 1, Tool: "nmap", Command: fmt.Sprintf("nmap -sV -p 139,445 --script smb-enum-shares,smb-vuln* %s", target), Purpose: "SMB service and vulnerability scan", Estimate: "~1-2min"},
				{ID: 2, Tool: "enum4linux", Command: fmt.Sprintf("enum4linux -a %s", target), Purpose: "Full SMB/NetBIOS enumeration", Estimate: "~1-3min"},
				{ID: 3, Tool: "report", Command: "generate report", Purpose: "Compile findings into provisional report", Estimate: "~30s"},
			},
			TotalEstimate: "~3-6 minutes",
			Risks:         "enum4linux generates significant authentication traffic",
		}
	}

	// Quick scan / basic scan / open ports
	if containsAny(gl, "quick scan", "scan for open", "open ports", "port scan", "basic scan") {
		return &PlanProposal{
			Scope:    target,
			Approach: "SYN scan → Version detection → Report",
			Phases: []PlanPhase{
				{ID: 1, Tool: "nmap", Command: fmt.Sprintf("nmap -sV -sC -T4 --open %s", target), Purpose: "Top 1000 ports with version detection", Estimate: "~1-2min"},
				{ID: 2, Tool: "report", Command: "generate report", Purpose: "Compile findings into provisional report", Estimate: "~30s"},
			},
			TotalEstimate: "~1-3 minutes",
			Risks:         "Standard scan — moderate noise level",
		}
	}

	return nil // No template match — will use LLM
}

func containsAny(s string, patterns ...string) bool {
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// verifyToolAvailability checks each phase's tool exists on the system.
func (o *Orchestrator) verifyToolAvailability(plan *PlanProposal) {
	seen := make(map[string]bool)
	for _, phase := range plan.Phases {
		tool := strings.ToLower(phase.Tool)
		if tool == "report" || tool == "k-0" || tool == "" {
			continue
		}
		if seen[tool] {
			continue
		}
		seen[tool] = true

		_, err := exec.LookPath(tool)
		available := err == nil
		plan.ToolChecks = append(plan.ToolChecks, ToolStatus{
			Tool:      tool,
			Available: available,
		})
		if !available {
			plan.MissingTools = append(plan.MissingTools, tool)
		}
	}
}

// ── Execution ─────────────────────────────────────────────────────────────

// executePlan converts an approved PlanProposal into tasks and runs them.
func (o *Orchestrator) executePlan(plan PlanProposal) {
	var tasks []Task

	o.emit(TaskUpdateMsg{
		AgentLabel: "[K-0]",
		Line:       fmt.Sprintf("Executing approved plan (%d phases)...", len(plan.Phases)),
		Timestamp:  time.Now(),
	})

	for _, phase := range plan.Phases {
		toolLower := strings.ToLower(phase.Tool)
		// Skip report/meta phases — we add report at the end
		if toolLower == "report" || toolLower == "k-0" || toolLower == "k0" || toolLower == "" {
			continue
		}

		taskType := guessTaskType(phase.Tool)

		// Build the command — use the phase command, or construct from tool name
		cmd := phase.Command
		if cmd == "" {
			cmd = phase.Tool
		}

		tasks = append(tasks, Task{
			ID:    fmt.Sprintf("phase-%d", phase.ID),
			Type:  taskType,
			Label: fmt.Sprintf("%s-%02d", capitalise(phase.Tool), phase.ID),
			Goal:  plan.Goal,
			Params: map[string]string{
				"cmd":    cmd,
				"target": extractTarget(cmd),
			},
		})

		o.emit(TaskUpdateMsg{
			AgentLabel: "[K-0]",
			Line:       fmt.Sprintf("  Phase %d: %s → %s", phase.ID, phase.Tool, cmd),
			Timestamp:  time.Now(),
		})
	}

	if len(tasks) == 0 {
		o.emit(TaskUpdateMsg{
			AgentLabel: "[K-0]",
			Line:       "⚠️  No executable phases — running basic recon",
			Timestamp:  time.Now(),
		})
		target := extractTargetFromGoal(plan.Goal)
		tasks = append(tasks, Task{
			ID:    "recon-01",
			Type:  TaskTypeScan,
			Label: "Nmap-01",
			Goal:  plan.Goal,
			Params: map[string]string{
				"cmd":    fmt.Sprintf("nmap -sV -sC -T4 --open %s", target),
				"target": target,
			},
		})
	}

	// Always end with report
	tasks = append(tasks, Task{
		ID:    "report-01",
		Type:  TaskTypeReport,
		Label: "Report-01",
		Goal:  plan.Goal,
	})

	o.executeTasks(plan.Goal, tasks)
}

// executeTasks is the core execution loop.
func (o *Orchestrator) executeTasks(rawGoal string, tasks []Task) {
	ctx := context.Background()
	goalID := uuid.New().String()[:8]

	o.emit(TaskUpdateMsg{
		AgentLabel: "[K-0]",
		Line:       fmt.Sprintf("Running %d task(s)...", len(tasks)),
		Timestamp:  time.Now(),
	})

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

// ── Helpers ───────────────────────────────────────────────────────────────

func fallbackPlan2(goal string) PlanProposal {
	target := extractTargetFromGoal(goal)
	isWeb := strings.Contains(strings.ToLower(goal), "web") ||
		strings.Contains(strings.ToLower(goal), "http") ||
		strings.Contains(strings.ToLower(goal), "vulnerabilit")

	phases := []PlanPhase{
		{ID: 1, Tool: "nmap", Command: fmt.Sprintf("nmap -sV -sC -T4 --open %s", target), Purpose: "Service discovery and version detection", Estimate: "~1-2min"},
	}
	if isWeb {
		phases = append(phases, PlanPhase{
			ID: 2, Tool: "nikto", Command: fmt.Sprintf("nikto -h %s", target), Purpose: "Web vulnerability scan", Estimate: "~2-5min",
		})
	}
	phases = append(phases, PlanPhase{
		ID: len(phases) + 1, Tool: "report", Command: "generate report", Purpose: "Compile findings into provisional report", Estimate: "~30s",
	})

	estimate := "~2-3 minutes"
	if isWeb {
		estimate = "~4-8 minutes"
	}

	return PlanProposal{
		Goal:          goal,
		Scope:         target,
		Approach:      "Discovery → Enumeration → Analysis → Report",
		Phases:        phases,
		TotalEstimate: estimate,
		Risks:         "Template plan — LLM was unavailable for intelligent decomposition",
	}
}

func extractTargetFromGoal(goal string) string {
	words := strings.Fields(goal)
	for _, w := range words {
		if strings.Count(w, ".") >= 1 {
			clean := strings.TrimRight(w, ".,;:\"'")
			parts := strings.Split(clean, ".")
			if len(parts) >= 2 {
				return clean
			}
		}
	}
	return "127.0.0.1"
}

func (o *Orchestrator) decomposeGoal(ctx context.Context, goal string) ([]Task, error) {
	raw, err := o.llm.DecomposeGoal(ctx, goal)
	if err != nil {
		return fallbackTaskPlan(goal), nil
	}

	raw = cleanJSON(raw)
	var tasks []Task
	if err := json.Unmarshal([]byte(raw), &tasks); err != nil {
		o.emit(TaskUpdateMsg{
			AgentLabel: "[K-0]",
			Line:       "⚠️  LLM returned bad JSON — using fallback plan",
			Timestamp:  time.Now(),
		})
		return fallbackTaskPlan(goal), nil
	}

	if len(tasks) == 0 || tasks[len(tasks)-1].Type != TaskTypeReport {
		tasks = append(tasks, Task{ID: "report-01", Type: TaskTypeReport, Label: "Report-01"})
	}
	return tasks, nil
}

func fallbackTaskPlan(goal string) []Task {
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

// detectDirectCommand checks if the goal starts with a known tool name.
func (o *Orchestrator) detectDirectCommand(rawGoal string) []Task {
	goal := strings.TrimSpace(rawGoal)
	goal = strings.TrimPrefix(goal, "_goal:")
	goal = strings.TrimSpace(goal)

	knownTools := []string{
		"nmap", "nikto", "whatweb", "ffuf", "gobuster", "feroxbuster",
		"wpscan", "sqlmap", "hydra", "masscan", "subfinder", "amass",
		"theHarvester", "whois", "dig", "curl", "wget", "searchsploit",
		"enum4linux", "smbclient", "crackmapexec", "msfconsole", "msfvenom",
		"aircrack-ng", "airodump-ng",
	}

	parts := strings.SplitN(goal, " ", 2)
	if len(parts) == 0 {
		return nil
	}
	cmdName := parts[0]

	for _, t := range knownTools {
		if cmdName == t {
			o.emit(TaskUpdateMsg{
				AgentLabel: "[K-0]",
				Line:       fmt.Sprintf("Direct command detected: %s", cmdName),
				Timestamp:  time.Now(),
			})

			taskType := guessTaskType(cmdName)

			return []Task{
				{
					ID:    "cmd-01",
					Type:  taskType,
					Label: fmt.Sprintf("%s-01", capitalise(cmdName)),
					Goal:  rawGoal,
					Params: map[string]string{
						"cmd": goal,
					},
				},
				{ID: "report-01", Type: TaskTypeReport, Label: "Report-01", Goal: rawGoal},
			}
		}
	}
	return nil
}

func guessTaskType(toolName string) TaskType {
	switch strings.ToLower(toolName) {
	case "nmap", "masscan":
		return TaskTypeScan
	case "whatweb", "nikto", "ffuf", "gobuster", "feroxbuster", "wpscan", "sqlmap":
		return TaskTypeWeb
	case "whois", "dig", "subfinder", "amass", "theharvester":
		return TaskTypeRecon
	default:
		return TaskTypeScan
	}
}

func extractTarget(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
