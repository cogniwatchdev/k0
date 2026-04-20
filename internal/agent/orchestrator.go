// Package agent — orchestrator.go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/k0-agent/k0/internal/config"
	"github.com/k0-agent/k0/internal/intel"
	"github.com/k0-agent/k0/internal/llm"
	"github.com/k0-agent/k0/internal/memory"
	"github.com/k0-agent/k0/internal/report"
	"github.com/k0-agent/k0/internal/skills"
)

// TaskUpdateMsg is sent to the TUI on each agent progress event.
type TaskUpdateMsg struct {
	AgentLabel string
	Line       string
	Timestamp  time.Time
}

// LLMStreamMsg carries a streaming token from the LLM to the TUI.
// The TUI can display these as they arrive for real-time feedback.
type LLMStreamMsg struct {
	Token   string // partial token text
	Done    bool   // true when stream is complete
	Err     error  // non-nil on stream error
	Full    string // accumulated text so far (set on Done=true)
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
	skills  *skills.Store
	intel   *intel.Browser
	updates chan interface{}
}

// NewOrchestrator creates a ready Orchestrator.
func NewOrchestrator(cfg *config.Config) *Orchestrator {
	o := &Orchestrator{
		cfg:     cfg,
		llm:     llm.NewClient(cfg.OllamaAddr, cfg.Model),
		memory:  memory.NewStore(cfg),
		intel:   intel.NewBrowser(),
		updates: make(chan interface{}, 128),
	}
	// Load pentest skills if configured
	if cfg.SkillsDir != "" {
		store, err := skills.Load(cfg.SkillsDir)
		if err != nil {
			// Non-fatal: skills are optional, log and continue
			fmt.Fprintf(os.Stderr, "[K-0] skills: %v\n", err)
		} else {
			o.skills = store
			fmt.Fprintf(os.Stderr, "[K-0] loaded %d skills from %s\n", len(store.List()), cfg.SkillsDir)
		}
	}
	return o
}

// PlanningResult describes how a goal would be handled by the planning pipeline.
// Used for testing and diagnostics — does not execute anything.
type PlanningResult struct {
	Goal          string       `json:"goal"`
	Target        string       `json:"target"`
	Method        string       `json:"method"`         // "direct_command", "skill_invocation", "skill_fuzzy", "template", "llm"
	Tasks         []Task       `json:"tasks,omitempty"`
	Plan          *PlanProposal `json:"plan,omitempty"`
	Skill         *skills.Skill `json:"skill,omitempty"`
	Command       string       `json:"command,omitempty"`
}

// AnalyzeGoal runs the planning pipeline as a dry run, returning which method
// would handle the goal and the resulting tasks/plan. Does NOT execute anything.
func (o *Orchestrator) AnalyzeGoal(rawGoal string) *PlanningResult {
	result := &PlanningResult{Goal: rawGoal}
	result.Target = extractTargetFromGoal(rawGoal)

	// 0. Explicit skill invocation
	if strings.HasPrefix(strings.TrimSpace(rawGoal), "_skill:") {
		if task := o.detectSkillCommand(rawGoal); task != nil {
			result.Method = "skill_invocation"
			result.Command = task.Params["cmd"]
			result.Tasks = []Task{*task, {ID: "report-01", Type: TaskTypeReport, Label: "Report-01", Goal: rawGoal}}
			return result
		}
	}

	// 0b. Explicit intel lookup
	if strings.HasPrefix(strings.TrimSpace(rawGoal), "_intel:") {
		if task := o.detectIntelCommand(rawGoal); task != nil {
			result.Method = "intel_lookup"
			result.Command = task.Params["cmd"]
			result.Tasks = []Task{*task, {ID: "report-01", Type: TaskTypeReport, Label: "Report-01", Goal: rawGoal}}
			return result
		}
	}

	// 1. Direct command
	if tasks := o.detectDirectCommand(rawGoal); tasks != nil {
		result.Method = "direct_command"
		result.Command = tasks[0].Params["cmd"]
		result.Tasks = tasks
		return result
	}

	// 2. Template matching
	if plan := o.matchTemplate(rawGoal, result.Target); plan != nil {
		result.Method = "template"
		result.Plan = plan
		return result
	}

	// 3. Skill fuzzy matching
	if o.skills != nil {
		if skill := o.skills.MatchGoal(rawGoal); skill != nil {
			result.Method = "skill_fuzzy"
			result.Skill = skill
			result.Command = o.skills.Command(skill, result.Target)
			result.Tasks = []Task{
				{ID: fmt.Sprintf("skill-%s", skill.ID), Type: TaskTypeSkill, Label: fmt.Sprintf("Skill-%s", capitalise(skill.Name)), Goal: rawGoal, Params: map[string]string{"cmd": result.Command, "skill_id": skill.ID}},
				{ID: "report-01", Type: TaskTypeReport, Label: "Report-01", Goal: rawGoal},
			}
			return result
		}
	}

	// 4. LLM fallback
	result.Method = "llm"
	return result
}

// SubmitGoal checks if this is a skill invocation, intel lookup, direct command, or needs planning.
func (o *Orchestrator) SubmitGoal(rawGoal string) tea.Cmd {
	// 0. Detect casual conversation — don't plan, just chat
	if o.isCasualConversation(rawGoal) {
		return o.respondToConversation(rawGoal)
	}

	// 0a. Check for explicit skill invocation: "_skill: subdomain_enum target.com"
	if o.skills != nil && strings.HasPrefix(strings.TrimSpace(rawGoal), "_skill:") {
		if task := o.detectSkillCommand(rawGoal); task != nil {
			tasks := []Task{*task, {ID: "report-01", Type: TaskTypeReport, Label: "Report-01", Goal: rawGoal}}
			return func() tea.Msg {
				go o.executeTasks(rawGoal, tasks)
				return nil
			}
		}
	}

	// 0b. Check for explicit intel lookup: "_intel: cve CVE-2021-44228" or "_intel: subdomains target.com"
	if strings.HasPrefix(strings.TrimSpace(rawGoal), "_intel:") {
		if task := o.detectIntelCommand(rawGoal); task != nil {
			tasks := []Task{*task, {ID: "report-01", Type: TaskTypeReport, Label: "Report-01", Goal: rawGoal}}
			return func() tea.Msg {
				go o.executeTasks(rawGoal, tasks)
				return nil
			}
		}
	}

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

// Updates returns the raw update channel for non-TUI consumers.
func (o *Orchestrator) Updates() chan interface{} {
	return o.updates
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

	// 2. Try to match a pentest skill (no LLM needed)
	if o.skills != nil {
		if skill := o.skills.MatchGoal(rawGoal); skill != nil {
			o.emit(TaskUpdateMsg{
				AgentLabel: "[K-0]",
				Line:       fmt.Sprintf("⚡ Matched skill: %s (%s)", skill.Name, skill.ID),
				Timestamp:  time.Now(),
			})
			cmd := o.skills.Command(skill, target)
			plan := &PlanProposal{
				Goal:          rawGoal,
				Scope:         target,
				Approach:      fmt.Sprintf("Skill: %s — %s", skill.Name, skill.Description),
				Phases: []PlanPhase{
					{ID: 1, Tool: "python3", Command: cmd, Purpose: skill.Description, Estimate: "~30s-2min"},
					{ID: 2, Tool: "report", Command: "generate report", Purpose: "Compile findings into provisional report", Estimate: "~30s"},
				},
				TotalEstimate: "~1-3 minutes",
				Risks:         fmt.Sprintf("Running external skill script from %s", skill.Source),
			}
			o.verifyToolAvailability(plan)
			o.updates <- PlanProposalMsg{Plan: *plan}
			return
		}
	}

	// 3. No template/skill match — try LLM planning with streaming feedback
	ctx := context.Background()
	o.emit(TaskUpdateMsg{
		AgentLabel: "[K-0]",
		Line:       "Generating custom plan via AI...",
		Timestamp:  time.Now(),
	})

	// Stream the LLM response token-by-token for real-time feedback
	system := `You are K-0, a tactical penetration testing AI. The user has given you a goal.
Produce a structured plan as a JSON object with these fields:
  "scope": string — summary of what is in scope (targets, networks, domains)
  "approach": string — high-level methodology (e.g. "Discovery → Enumeration → Vulnerability scan")
  "phases": array of objects, each with:
      "id": number (1, 2, 3...)
      "tool": string — the tool name (e.g. "nmap", "nikto")
      "command": string — the exact command to run
      "purpose": string — what this phase accomplishes
      "estimate": string — estimated duration (e.g. "~30s", "~2-4min")
  "total_estimate": string — estimated total duration
  "risks": string — any risks or caveats the operator should know

Rules:
- Use only tools available on Kali Linux: nmap, nikto, whatweb, ffuf, gobuster, wpscan, sqlmap, hydra, searchsploit, enum4linux, smbclient, crackmapexec, subfinder, whois, dig, curl
- Be specific with commands — include actual flags and targets
- Keep estimates realistic for CPU-only inference
- Always include a final report phase
- Output ONLY the JSON object. No explanation. No markdown fences.
/no_think`
	prompt := fmt.Sprintf("Goal: %s\n/no_think", rawGoal)

	var accumulated strings.Builder
	tokenCh := o.llm.StreamComplete(ctx, system, prompt)
	streamErr := false

	for token := range tokenCh {
		if token.Err != nil {
			o.emit(TaskUpdateMsg{
				AgentLabel: "[K-0]",
				Line:       fmt.Sprintf("LLM stream error: %v", token.Err),
				Timestamp:  time.Now(),
			})
			streamErr = true
			break
		}
		if token.Text != "" {
			accumulated.WriteString(token.Text)
			// Emit tokens as streaming messages so the TUI can show progress
			o.emit(LLMStreamMsg{
				Token: token.Text,
				Full:  accumulated.String(),
			})
		}
		// Empty token with no error = completion signal — loop will exit when channel closes
	}

	raw := accumulated.String()

	// If streaming failed, fall back to blocking call
	if streamErr || raw == "" {
		o.emit(TaskUpdateMsg{
			AgentLabel: "[K-0]",
			Line:       "Streaming failed — retrying with blocking call...",
			Timestamp:  time.Now(),
		})
		var err error
		raw, err = o.llm.PlanGoal(ctx, rawGoal)
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
	if containsAny(gl, "web vuln", "web scan", "website scan", "http scan", "web vulnerabilit", "owasp", "web assess", "security posture", "web app test", "web pentest") {
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
			Approach: "Host discovery → Service enumeration → Report",
			Phases: []PlanPhase{
				{ID: 1, Tool: "nmap", Command: fmt.Sprintf("nmap -sn %s", target), Purpose: "Host discovery (ping sweep)", Estimate: "~15-30s"},
				{ID: 2, Tool: "nmap", Command: fmt.Sprintf("nmap -sV -sC -T4 --open --top-ports 1000 --max-rtt-timeout 500ms %s", target), Purpose: "Service detection + NSE scripts on top 1000 ports", Estimate: "~1-3min"},
				{ID: 3, Tool: "report", Command: "generate report", Purpose: "Compile findings into provisional report", Estimate: "~30s"},
			},
			TotalEstimate: "~2-4 minutes",
			Risks:         "NSE scripts (-sC) may trigger IDS alerts on sensitive networks",
		}
	}


	// SMB/Windows/AD enum
	if containsAny(gl, "smb", "windows", "shares", "netbios", "active directory", "ad enum", "kerberos", "spn", "ldap enum") {
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
				{ID: 1, Tool: "nmap", Command: fmt.Sprintf("nmap -sV -sC -T4 --open --top-ports 1000 --max-rtt-timeout 500ms %s", target), Purpose: "Top 1000 ports with version detection", Estimate: "~1-2min"},
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
				"cmd":    fmt.Sprintf("nmap -sV -sC -T4 --open --top-ports 1000 --max-rtt-timeout 500ms %s", target),
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

	// Separate intel tasks (run via API) from regular tasks (run via subagents/shell)
	var intelTasks []Task
	var shellTasks []Task
	for _, t := range tasks {
		if t.Type == TaskTypeIntel {
			intelTasks = append(intelTasks, t)
		} else {
			shellTasks = append(shellTasks, t)
		}
	}

	results := make(chan SubagentResult, len(tasks))

	// Execute intel tasks via the intel browser (no shell execution needed)
	for _, t := range intelTasks {
		go func(t Task) {
			action := t.Params["action"]
			query := t.Params["query"]
			output, err := o.IntelLookup(ctx, action, query)
			sr := SubagentResult{Task: t, Label: t.Label}
			if err != nil {
				sr.Err = err
				o.emit(TaskUpdateMsg{
					AgentLabel: fmt.Sprintf("[%s]", t.Label),
					Line:       fmt.Sprintf("⚠️  Intel error: %v", err),
					Timestamp:   time.Now(),
				})
			} else {
				sr.ToolCalls = []ToolCall{{
					Tool:   fmt.Sprintf("intel:%s", action),
					Args:   query,
					Output: output,
					RunAt:  time.Now(),
				}}
				o.emit(TaskUpdateMsg{
					AgentLabel: fmt.Sprintf("[%s]", t.Label),
					Line:       output,
					Timestamp:  time.Now(),
				})
			}
			results <- sr
		}(t)
	}

	// Execute regular tasks via subagents (shell execution)
	for _, task := range shellTasks {
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
		Tags:      extractTags(rawGoal, allResults),
	}
	_ = o.memory.SaveEpisode(ep)

	o.emit(TaskUpdateMsg{
		AgentLabel: "[K-0]",
		Line:       "Saving episode and report...",
		Timestamp:  time.Now(),
	})

	prov, _ := report.Generate(ctx, o.llm, rawGoal, allResults)
	_ = o.memory.SaveReport(goalID, prov)

	// Extract knowledge from results (LLM-powered, may take a few seconds)
	o.emit(TaskUpdateMsg{
		AgentLabel: "[K-0]",
		Line:       "Extracting knowledge from findings...",
		Timestamp:  time.Now(),
	})
	o.extractAndSaveKnowledge(ctx, rawGoal, allResults)

	// Save discovered entities (targets, CVEs)
	o.saveEntities(allResults)

	o.emit(TaskUpdateMsg{
		AgentLabel: "[K-0]",
		Line:       "Memory persistence complete.",
		Timestamp:  time.Now(),
	})

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
		{ID: 1, Tool: "nmap", Command: fmt.Sprintf("nmap -sV -sC -T4 --open --top-ports 1000 --max-rtt-timeout 500ms %s", target), Purpose: "Service discovery and version detection", Estimate: "~1-2min"},
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
		// Strip URL schemes (http://, https://)
		clean := w
		for _, scheme := range []string{"https://", "http://"} {
			if strings.HasPrefix(strings.ToLower(clean), scheme) {
				clean = clean[len(scheme):]
				break
			}
		}
		// Strip trailing punctuation
		clean = strings.TrimRight(clean, ".,;:\"'/")
		// Strip trailing path components (e.g. example.com/path → example.com)
		// But preserve CIDR notation (e.g. 10.0.0.0/24)
		if idx := strings.Index(clean, "/"); idx >= 0 {
			// Check if this looks like CIDR (/N where N is 1-2 digits)
			rest := clean[idx+1:]
			if !isCIDR(rest) {
				clean = clean[:idx]
			}
		}
		// Strip port suffix (e.g. example.com:8080 → example.com)
		if idx := strings.LastIndex(clean, ":"); idx >= 0 {
			// But not for IPv6 or bare colons
			if idx > 0 && clean[idx-1] != ':' {
				clean = clean[:idx]
			}
		}
		if strings.Count(clean, ".") >= 1 {
			parts := strings.Split(clean, ".")
			if len(parts) >= 2 {
				return clean
			}
		}
		// Also match IP addresses
		if net.ParseIP(clean) != nil {
			return clean
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
	case "python3", "python":
		return TaskTypeSkill
	case "searchsploit":
		return TaskTypeIntel
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

// isCIDR checks if a string looks like a CIDR mask (e.g. "24", "16").
func isCIDR(s string) bool {
	if len(s) == 0 || len(s) > 3 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ── Skill Integration ────────────────────────────────────────────────────

// detectSkillCommand parses "_skill: skill_name [args...]" into a Task.
func (o *Orchestrator) detectSkillCommand(rawGoal string) *Task {
	if o.skills == nil {
		return nil
	}
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rawGoal), "_skill:"))
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return nil
	}

	skillName := parts[0]
	skill, ok := o.skills.Lookup(skillName)
	if !ok {
		o.emit(TaskUpdateMsg{
			AgentLabel: "[K-0]",
			Line:       fmt.Sprintf("⚠️  Unknown skill: %s", skillName),
			Timestamp:  time.Now(),
		})
		return nil
	}

	// Build command: python3 /path/to/skill.py [args...]
	args := parts[1:] // args after skill name (e.g. target)
	cmd := o.skills.Command(skill, args...)

	o.emit(TaskUpdateMsg{
		AgentLabel: "[K-0]",
		Line:       fmt.Sprintf("⚡ Skill match: %s → %s", skill.Name, cmd),
		Timestamp:  time.Now(),
	})

	return &Task{
		ID:    fmt.Sprintf("skill-%s", skill.ID),
		Type:  TaskTypeSkill,
		Label: fmt.Sprintf("Skill-%s", capitalise(skill.Name)),
		Goal:  rawGoal,
		Params: map[string]string{
			"cmd":       cmd,
			"skill_id":  skill.ID,
			"skill_src": skill.Source,
		},
	}
}

// ── Intel Integration ──────────────────────────────────────────────────

// detectIntelCommand parses "_intel: <action> <query>" into a Task.
// Supported actions: cve, cve-search, subdomains, dns, reversedns, headers, whois
// Examples:
//
//	_intel: cve CVE-2021-44228
//	_intel: cve-search log4j rce
//	_intel: subdomains example.com
//	_intel: dns example.com
//	_intel: reversedns 1.2.3.4
//	_intel: headers example.com
//	_intel: whois example.com
func (o *Orchestrator) detectIntelCommand(rawGoal string) *Task {
	if o.intel == nil {
		return nil
	}
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rawGoal), "_intel:"))
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return nil
	}

	action := strings.ToLower(parts[0])
	query := strings.Join(parts[1:], " ")

	// Build a synthetic command that the intel planTools handler will interpret
	cmd := fmt.Sprintf("_intel:%s %s", action, query)

	var label string
	switch action {
	case "cve":
		label = "Intel-CVE-Lookup"
	case "cve-search":
		label = "Intel-CVE-Search"
	case "subdomains":
		label = "Intel-Subdomains"
	case "dns":
		label = "Intel-DNS"
	case "reversedns":
		label = "Intel-ReverseDNS"
	case "headers":
		label = "Intel-HTTPHeaders"
	case "whois":
		label = "Intel-WHOIS"
	default:
		o.emit(TaskUpdateMsg{
			AgentLabel: "[K-0]",
			Line:       fmt.Sprintf("⚠️  Unknown intel action: %s (use: cve, cve-search, subdomains, dns, reversedns, headers, whois)", action),
			Timestamp:  time.Now(),
		})
		return nil
	}

	o.emit(TaskUpdateMsg{
		AgentLabel: "[K-0]",
		Line:       fmt.Sprintf("🔍 Intel lookup: %s %q", action, query),
		Timestamp:  time.Now(),
	})

	return &Task{
		ID:    fmt.Sprintf("intel-%s-01", action),
		Type:  TaskTypeIntel,
		Label: label,
		Goal:  rawGoal,
		Params: map[string]string{
			"cmd":    cmd,
			"action": action,
			"query":  query,
		},
	}
}

// IntelLookup runs an intel query and returns the raw result.
// This is the programmatic API for external callers (e.g. TUI, test harnesses).
func (o *Orchestrator) IntelLookup(ctx context.Context, action, query string) (string, error) {
	if o.intel == nil {
		return "", fmt.Errorf("intel browser not initialized")
	}

	switch strings.ToLower(action) {
	case "cve":
		detail, err := o.intel.LookupCVE(ctx, query)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("CVE: %s\nSeverity: %s (CVSS %.1f)\nCWE: %s\nPublished: %s\nDescription: %s\nReferences:\n  %s",
			detail.ID, detail.Severity, detail.CVSSScore, detail.CWE, detail.Published,
			detail.Description,
			strings.Join(detail.References, "\n  ")), nil

	case "cve-search":
		sr, err := o.intel.SearchCVEs(ctx, query, 5)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "CVE Search: %q (%d results)\n", query, sr.TotalResults)
		for _, cve := range sr.CVEs {
			fmt.Fprintf(&b, "  %s [%s %.1f] %s\n", cve.ID, cve.Severity, cve.CVSSScore,
				trunc(cve.Description, 120))
		}
		return b.String(), nil

	case "subdomains":
		result, err := o.intel.DiscoverSubdomains(ctx, query)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Subdomains for %s: %d found\n", result.Domain, result.Count)
		for _, sub := range result.Subdomains {
			fmt.Fprintf(&b, "  %s\n", sub)
		}
		return b.String(), nil

	case "dns":
		result, err := o.intel.DNSLookup(ctx, query)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "DNS records for %s:\n", result.Domain)
		for _, rec := range result.Records {
			fmt.Fprintf(&b, "  %s\n", rec)
		}
		return b.String(), nil

	case "reversedns":
		result, err := o.intel.ReverseDNS(ctx, query)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Reverse DNS for %s:\n", result.Domain)
		for _, rec := range result.Records {
			fmt.Fprintf(&b, "  %s\n", rec)
		}
		return b.String(), nil

	case "headers":
		headers, err := o.intel.HTTPHeaders(ctx, query)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "HTTP headers for %s:\n", query)
		for _, h := range headers {
			fmt.Fprintf(&b, "  %s\n", h)
		}
		return b.String(), nil

	case "whois":
		result, err := o.intel.WHOISLookup(ctx, query)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "WHOIS for %s:\n", result.Domain)
		for _, line := range result.Raw {
			fmt.Fprintf(&b, "  %s\n", line)
		}
		return b.String(), nil

	default:
		return "", fmt.Errorf("unknown intel action: %s", action)
	}
}

// trunc shortens a string to maxLen with ellipsis.
func trunc(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// extractTags generates tags from the goal and results for episode categorization.
func extractTags(goal string, results []SubagentResult) []string {
	tags := map[string]bool{}
	goalLower := strings.ToLower(goal)

	// Goal-based tags
	for _, kw := range []string{"recon", "scan", "web", "wireless", "exploit", "enum", "subdomain", "port", "ssl", "smb", "dns"} {
		if strings.Contains(goalLower, kw) {
			tags[kw] = true
		}
	}

	// Task type tags
	for _, r := range results {
		tags[string(r.Task.Type)] = true
	}

	// Tool tags
	for _, r := range results {
		for _, tc := range r.ToolCalls {
			tags[tc.Tool] = true
		}
	}

	var out []string
	for t := range tags {
		out = append(out, t)
	}
	return out
}

// extractAndSaveKnowledge uses the LLM to distill lessons from the engagement.
func (o *Orchestrator) extractAndSaveKnowledge(ctx context.Context, goal string, results []SubagentResult) {
	// Collect all findings
	var allFindings []Finding
	for _, r := range results {
		allFindings = append(allFindings, r.Findings...)
	}
	if len(allFindings) == 0 {
		return // nothing to learn from
	}

	// Summarize findings for knowledge extraction
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Goal: %s\nFindings:\n", goal))
	for _, f := range allFindings {
		sb.WriteString(fmt.Sprintf("- [%s] %s on %s", f.Severity, f.Title, f.Target))
		if f.CVE != "" {
			sb.WriteString(fmt.Sprintf(" (CVE: %s)", f.CVE))
		}
		sb.WriteString("\n")
	}

	// Extract knowledge via LLM
	prompt := fmt.Sprintf(`Given these security findings from a pentest engagement, extract 1-3 concise knowledge entries as JSON array.
Each entry: {"category":"recon|vuln|exploit|tool","summary":"one-line lesson","cve":"CVE-XXXX-XXXXX if applicable","tool":"toolname if applicable"}
Keep summaries under 100 chars. Be specific and actionable.

%s

Respond with JSON array only./no_think`, sb.String())

	raw, err := o.llm.CompleteJSON(ctx, "You extract concise security knowledge entries from findings.", prompt)
	if err != nil {
		return // non-fatal
	}

	var entries []memory.KnowledgeEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		// Try wrapping in array
		var single memory.KnowledgeEntry
		if err2 := json.Unmarshal([]byte(raw), &single); err2 != nil {
			return
		}
		entries = append(entries, single)
	}

	for i := range entries {
		entries[i].ID = fmt.Sprintf("k-%d", time.Now().UnixNano()+int64(i))
		_ = o.memory.AppendKnowledge(entries[i])
	}
}

// saveEntities persists discovered targets and CVEs to the entity store.
func (o *Orchestrator) saveEntities(results []SubagentResult) {
	// Collect unique targets and CVEs
	entities := make(map[string]string) // key -> type
	for _, r := range results {
		for _, f := range r.Findings {
			if f.Target != "" {
				key := f.Target
				if _, exists := entities[key]; !exists {
					entities[key] = "target"
				}
			}
			if f.CVE != "" {
				key := "cve:" + f.CVE
				entities[key] = "cve"
			}
		}
	}

	// Write entities as JSONL
	for key, etype := range entities {
		entry := map[string]string{
			"id":         key,
			"type":       etype,
			"discovered": time.Now().Format(time.RFC3339),
		}
		path := filepath.Join(config.MemoryDir(o.cfg, "entities"), "discovered.jsonl")
		_ = appendJSONLine(path, entry)
	}
}

func appendJSONLine(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}

// isCasualConversation detects if the user input is casual chat rather than a pentest goal.
// Returns true for greetings, thanks, questions about K-0, etc.
func (o *Orchestrator) isCasualConversation(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	
	// Greetings
	greetings := []string{"hi", "hello", "hey", "yo", "greetings", "good morning", "good afternoon", "good evening"}
	for _, g := range greetings {
		if s == g || strings.HasPrefix(s, g+" ") || strings.HasPrefix(s, g+",") {
			return true
		}
	}
	
	// Farewells
	farewells := []string{"bye", "goodbye", "see you", "later", "quit", "exit"}
	for _, f := range farewells {
		if s == f || strings.HasPrefix(s, f+" ") || strings.HasPrefix(s, f+",") {
			return true
		}
	}
	
	// Thanks
	thanks := []string{"thanks", "thank you", "thx", "cheers"}
	for _, t := range thanks {
		if s == t || strings.HasPrefix(s, t+" ") || strings.HasPrefix(s, t+",") || strings.HasSuffix(s, " "+t) {
			return true
		}
	}
	
	// Status/help questions
	statusQuestions := []string{"how are you", "what can you do", "what's your name", "who are you", "help", "status", "version"}
	for _, q := range statusQuestions {
		if strings.Contains(s, q) {
			return true
		}
	}
	
	// Yes/no responses (not planning-related)
	if s == "yes" || s == "no" || s == "y" || s == "n" || s == "sure" || s == "ok" || s == "okay" {
		return true
	}
	
	return false
}

// respondToConversation sends a friendly chat response instead of a plan.
func (o *Orchestrator) respondToConversation(input string) tea.Cmd {
	s := strings.ToLower(strings.TrimSpace(input))
	
	var response string
	
	// Greetings
	if strings.Contains(s, "hi") || strings.Contains(s, "hello") || strings.Contains(s, "hey") || strings.Contains(s, "yo") {
		response = "Hey! 👋 I'm K-0, your offensive security agent. Type your pentest goal (e.g., 'goal: scan 192.168.1.0/24' or 'web scan example.com')."
	} else if strings.Contains(s, "bye") || strings.Contains(s, "goodbye") || s == "quit" || s == "exit" {
		response = "Stay safe out there. Type 'q' or press Ctrl+C to exit."
	} else if strings.Contains(s, "thanks") || strings.Contains(s, "thank you") || s == "thx" || s == "cheers" {
		response = "Anytime. Ready when you are — just drop your next goal."
	} else if strings.Contains(s, "how are you") {
		response = "Systems nominal. All tools loaded. What's the target?"
	} else if strings.Contains(s, "what can you do") || strings.Contains(s, "help") {
		response = "I'm built for pentesting: recon, web scans, DNS enum, brute-force, vuln scanning, and more. Try: 'goal: full recon on 10.0.0.0/24' or 'web scan target.com'. I'll generate a plan and wait for your approval before executing."
	} else if strings.Contains(s, "version") {
		response = "Running K-0 v0.4.0-dev with LFM2.5-350M embedded model. Offline-first, zero config."
	} else if s == "yes" || s == "y" || s == "sure" || s == "ok" || s == "okay" {
		response = "👍 Awaiting your goal. What's the target?"
	} else if s == "no" || s == "n" {
		response = "No problem. Let me know when you're ready."
	} else {
		response = "Got it. What's your pentest goal? (e.g., 'goal: scan 192.168.1.1' or 'web recon example.com')"
	}
	
	return func() tea.Msg {
		o.emit(TaskUpdateMsg{
			AgentLabel: "[K-0]",
			Line:       response,
			Timestamp:  time.Now(),
		})
		return nil
	}
}
