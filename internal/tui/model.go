// Package tui — model.go
// Root Bubble Tea model.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/k0-agent/k0/internal/agent"
	"github.com/k0-agent/k0/internal/config"
	"github.com/k0-agent/k0/internal/types"
)

// FindingMsg carries a security finding to the TUI.
type FindingMsg struct {
	Finding    types.Finding
	TaskLabel  string
}

type view int

const (
	viewChat view = iota
	viewMemory
	viewFindings
	viewSettings
)

// spinnerFrames are the braille spinner animation frames.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// tickMsg drives the spinner and elapsed timer animation.
type tickMsg time.Time

type model struct {
	cfg          *config.Config
	width        int
	height       int
	activeView   view
	input        textinput.Model
	chatView     *chatViewModel
	memoryView   memoryViewModel
	findingsView findingsViewModel
	settingsView settingsViewModel
	orchestrator *agent.Orchestrator
	statusLine   string
	version      string
	nodeLabel    string
	busy         bool                // true while a goal is running
	busyStart    time.Time           // when the current goal started
	spinnerIdx   int                 // current spinner frame index
	lastActivity string              // last agent label that sent an update
	pendingPlan  *agent.PlanProposal // non-nil when waiting for y/n
	totalTasks   int                 // total tasks in current goal
	doneTasks    int                 // completed tasks in current goal
	totalFindings int                // findings count across entire session
}

func newModel(cfg *config.Config) model {
	ti := textinput.New()
	ti.Placeholder = "_goal: <your objective>"
	ti.Focus()
	ti.CharLimit = 512
	ti.Width = 80

	orch := agent.NewOrchestrator(cfg)
	cv := newChatViewModel()

	return model{
		cfg:          cfg,
		input:        ti,
		chatView:     &cv,
		memoryView:   newMemoryViewModel(cfg),
		findingsView: newFindingsViewModel(),
		settingsView: newSettingsViewModel(cfg),
		orchestrator: orch,
		statusLine:   "CHECKING",
		version:      "v0.4.0",
		nodeLabel:    "LOCAL",
		activeView:   viewChat,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.pingOllamaCmd(),
	)
}

type ollamaStatusMsg struct {
	ok  bool
	err error
}

func (m model) pingOllamaCmd() tea.Cmd {
	return func() tea.Msg {
		err := m.orchestrator.PingLLM(context.Background())
		return ollamaStatusMsg{ok: err == nil, err: err}
	}
}

// tickCmd starts the 200ms animation tick — only runs while busy.
func tickCmd() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case ollamaStatusMsg:
		if msg.ok {
			m.statusLine = "READY"
		} else {
			m.statusLine = "LLM OFFLINE"
			m.chatView.appendUpdate(agent.TaskUpdateMsg{
				AgentLabel: "[K-0]",
				Line:       fmt.Sprintf("⚠️  Cannot reach Ollama at %s — run: systemctl start k0-ollama", m.cfg.OllamaAddr),
				Timestamp:  time.Now(),
			})
		}

	case tickMsg:
		if m.busy {
			m.spinnerIdx = (m.spinnerIdx + 1) % len(spinnerFrames)
			cmds = append(cmds, tickCmd())
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+d":
			return m, tea.Quit
		case "q", "Q":
			if m.input.Value() == "" && m.pendingPlan == nil {
				return m, tea.Quit
			}
		case "ctrl+l":
			if !m.busy {
				m.chatView.clear()
			}
		case "esc":
			if m.pendingPlan != nil {
				// Cancel the pending plan
				m.pendingPlan = nil
				m.statusLine = "READY"
				m.busy = false
				m.chatView.appendUpdate(agent.TaskUpdateMsg{
					AgentLabel: "[K-0]",
					Line:       "Plan cancelled.",
					Timestamp:  time.Now(),
				})
			}
			m.input.SetValue("")
			m.input.Blur()
			m.input.Focus()
		case "tab":
			m.activeView = (m.activeView + 1) % 4
		case "enter":
			if m.pendingPlan != nil {
				// Handle plan confirmation (y/n)
				val := strings.TrimSpace(strings.ToLower(m.input.Value()))
				m.input.SetValue("")
				if val == "y" || val == "yes" {
					plan := *m.pendingPlan
					m.pendingPlan = nil
					m.busy = true
					m.busyStart = time.Now()
					m.spinnerIdx = 0
					m.statusLine = "BUSY"
					m.lastActivity = "K-0"
					m.chatView.appendUpdate(agent.TaskUpdateMsg{
						AgentLabel: "[K-0]",
						Line:       "✓ Plan approved — executing...",
						Timestamp:  time.Now(),
					})
					cmds = append(cmds,
						m.orchestrator.ExecuteApprovedPlan(plan),
						m.orchestrator.ListenUpdates(),
						tickCmd(),
					)
				} else if val == "n" || val == "no" {
					m.pendingPlan = nil
					m.busy = false
					m.statusLine = "READY"
					m.chatView.appendUpdate(agent.TaskUpdateMsg{
						AgentLabel: "[K-0]",
						Line:       "Plan rejected. Awaiting new objective.",
						Timestamp:  time.Now(),
					})
				} else if val == "i" || val == "install" {
					if m.pendingPlan != nil && len(m.pendingPlan.MissingTools) > 0 {
						tools := m.pendingPlan.MissingTools
						m.chatView.appendUpdate(agent.TaskUpdateMsg{
							AgentLabel: "[K-0]",
							Line:       fmt.Sprintf("Installing %d missing tool(s)...", len(tools)),
							Timestamp:  time.Now(),
						})
						cmds = append(cmds,
							m.orchestrator.InstallTools(tools),
							m.orchestrator.ListenUpdates(),
						)
					} else {
						m.chatView.appendUpdate(agent.TaskUpdateMsg{
							AgentLabel: "[K-0]",
							Line:       "No missing tools to install.",
							Timestamp:  time.Now(),
						})
					}
				} else {
					m.chatView.appendUpdate(agent.TaskUpdateMsg{
						AgentLabel: "[K-0]",
						Line:       "Type 'y' to execute, 'n' to cancel, or 'i' to install missing tools.",
						Timestamp:  time.Now(),
					})
				}
			} else if !m.busy && m.input.Focused() && m.input.Value() != "" {
				goal := m.input.Value()
				m.input.SetValue("")
				m.busy = true
				m.busyStart = time.Now()
				m.spinnerIdx = 0
				m.lastActivity = "K-0"
				m.statusLine = "BUSY"
				cmds = append(cmds,
					m.orchestrator.SubmitGoal(goal),
					m.orchestrator.ListenUpdates(),
					tickCmd(),
				)
			}
		}

	case agent.PlanProposalMsg:
		m.pendingPlan = &msg.Plan
		m.chatView.appendPlan(msg.Plan)
		m.statusLine = "CONFIRM"
		m.busy = false // stop spinner, wait for user input
		// Do NOT re-arm listener — we're waiting for user y/n

	case agent.TaskUpdateMsg:
		m.chatView.appendUpdate(msg)
		m.statusLine = "BUSY"
		m.lastActivity = msg.AgentLabel
		// Detect task start/done lines for progress tracking
		if strings.Contains(msg.Line, "Running") && strings.Contains(msg.Line, "task(s)") {
			// Parse "Running N task(s)..." to set total
			var n int
			fmt.Sscanf(msg.Line, "Running %d task", &n)
			if n > 0 {
				m.totalTasks = n
				m.doneTasks = 0
			}
		}
		// Detect findings in update lines
		for _, sev := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "INFO"} {
			if strings.Contains(msg.Line, "["+sev+"]") {
				m.totalFindings++
			}
		}
		cmds = append(cmds, m.orchestrator.ListenUpdates())

	case FindingMsg:
		m.findingsView.addFinding(msg.Finding, msg.TaskLabel)
		m.totalFindings = len(m.findingsView.findings)

	case agent.LLMStreamMsg:
		// Show LLM stream tokens in the chat as a single updating line
		m.chatView.updateStreamingLine(msg)

	case agent.TaskDoneMsg:
		elapsed := time.Since(m.busyStart).Round(time.Second)
		m.chatView.appendDone(msg)
		m.chatView.appendUpdate(agent.TaskUpdateMsg{
			AgentLabel: "[K-0]",
			Line:       fmt.Sprintf("⏱  Completed in %s", elapsed),
			Timestamp:  time.Now(),
		})
		m.statusLine = "READY"
		m.busy = false
		m.lastActivity = ""
		m.doneTasks = m.totalTasks
	}

	switch m.activeView {
	case viewChat:
		var cmd tea.Cmd
		cv, cmd := m.chatView.Update(msg)
		m.chatView = cv
		cmds = append(cmds, cmd)
	case viewMemory:
		var cmd tea.Cmd
		m.memoryView, cmd = m.memoryView.Update(msg)
		cmds = append(cmds, cmd)
	case viewFindings:
		var cmd tea.Cmd
		m.findingsView, cmd = m.findingsView.Update(msg)
		cmds = append(cmds, cmd)
	case viewSettings:
		var cmd tea.Cmd
		m.settingsView, cmd = m.settingsView.Update(msg)
		cmds = append(cmds, cmd)
	}

	var inputCmd tea.Cmd
	m.input, inputCmd = m.input.Update(msg)
	cmds = append(cmds, inputCmd)

	return m, tea.Batch(cmds...)
}

// logoLineCount returns how many lines the logo takes for a given height.
func logoLineCount(height int) int {
	if height < 38 {
		return 2 // compact logo — saves space on smaller terminals
	}
	return 8 // full Unicode logo: 6 art + 1 separator + 1 tagline
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading K-0..."
	}

	logo := RenderLogo(m.width, m.height)
	logoLines := logoLineCount(m.height)

	// Dynamic vertical budget — exact line count:
	//   logo (8 full / 2 compact) + status (1) + blank (1) + tabs (1) + content + blank (1) + hints (1) + input (2)
	//   PanelFocused.Height(contentH) renders contentH+2 lines (border top/bottom)
	//   InputBar renders 2 lines (BorderTop + content)
	//   So total = logoLines + 1 + 1 + 1 + (contentH + 2) + 1 + 1 + 2 = logoLines + 9 + contentH
	//   For fit: contentH = height - logoLines - 9
	//
	//   NOTE: safetyMargin = 0 because terminal emulators (iTerm2, Terminal.app)
	//   report window height including title bar. Any positive margin pushes
	//   the logo off-screen.
	const safetyMargin = 0
	chromeLines := logoLines + 9 + safetyMargin
	contentH := m.height - chromeLines
	if contentH < 4 {
		contentH = 4
	}

	// Status dot
	var statusDotStr string
	if m.busy {
		elapsed := time.Since(m.busyStart).Round(time.Second)
		spinner := lipgloss.NewStyle().Foreground(KaliPurple).Render(spinnerFrames[m.spinnerIdx])
		elapsedStr := lipgloss.NewStyle().Foreground(TextMuted).Render(fmt.Sprintf(" [%s]", elapsed))
		activityStr := lipgloss.NewStyle().Foreground(TextSecondary).Render(
			fmt.Sprintf(" WORKING %s", m.lastActivity),
		)
		versionStr := lipgloss.NewStyle().Foreground(TextMuted).Render(
			fmt.Sprintf(" | %s | %s", m.version, m.nodeLabel),
		)
		statusDotStr = spinner + activityStr + elapsedStr + versionStr
	} else {
		statusDotStr = RenderStatusDot(m.statusLine, m.version, m.nodeLabel)
	}
	statusDot := centre(statusDotStr, m.width)

	tabs := m.renderTabs()

	var content string
	switch m.activeView {
	case viewChat:
		content = m.chatView.View(m.width-4, contentH, m.busy, m.spinnerIdx, m.busyStart)
	case viewMemory:
		content = m.memoryView.View(m.width-4, contentH)
	case viewFindings:
		content = m.findingsView.View(m.width-4, contentH)
	case viewSettings:
		content = m.settingsView.View(m.width-4, contentH)
	}

	// Input line — changes based on state
	var inputLine string
	if m.pendingPlan != nil {
		// Waiting for confirmation
		confirmHint := lipgloss.NewStyle().Foreground(KaliPurpleBright).Render("Proceed? ")
		inputLine = InputBar.Width(m.width - 4).Render(
			Prompt.Render("[K0] > ") + confirmHint + m.input.View(),
		)
	} else if m.busy {
		elapsed := time.Since(m.busyStart).Round(time.Second)
		spinner := lipgloss.NewStyle().Foreground(KaliPurple).Render(spinnerFrames[m.spinnerIdx])
		waitMsg := lipgloss.NewStyle().Foreground(TextMuted).Italic(true).Render(
			fmt.Sprintf(" K-0 is working... %s elapsed", elapsed),
		)
		inputLine = InputBar.Width(m.width - 4).Render(
			Prompt.Render("[K0] > ") + spinner + waitMsg,
		)
	} else {
		inputLine = InputBar.Width(m.width - 4).Render(
			Prompt.Render("[K0] > ") + m.input.View(),
		)
	}

	// Hints — context-sensitive
	var hintsText string
	if m.pendingPlan != nil {
		hintsText = "  y: approve  •  n: reject  •  i: install missing tools  •  esc: cancel"
	} else {
		hintsText = "  tab: panels  •  ctrl+l: clear  •  q/ctrl+d: quit  •  enter: submit goal"
	}
	hints := StatusBar.Width(m.width).Render(hintsText)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		logo,
		statusDot,
		"",
		tabs,
		content,
		"",
		hints,
		inputLine,
	)
}

func (m model) renderTabs() string {
	type tabDef struct {
		label  string
		badge  string // optional count badge
	}
	tabs := []tabDef{
		{label: "Chat"},
		{label: "Memory"},
		{label: "Findings", badge: func() string {
			if m.totalFindings > 0 {
				return fmt.Sprintf("(%d)", m.totalFindings)
			}
			return ""
		}()},
		{label: "Settings"},
	}
	parts := make([]string, len(tabs))
	for i, t := range tabs {
		display := t.label
		if t.badge != "" {
			display += " " + t.badge
		}
		if view(i) == m.activeView {
			parts[i] = lipgloss.NewStyle().
				Foreground(KaliPurple).
				Bold(true).
				Underline(true).
				Padding(0, 2).
				Render(display)
		} else {
			parts[i] = lipgloss.NewStyle().
				Foreground(TextMuted).
				Padding(0, 2).
				Render(display)
		}
	}

	// Task progress indicator on the right side
	progressStr := ""
	if m.totalTasks > 0 {
		done := m.doneTasks
		total := m.totalTasks
		pct := 0
		if total > 0 {
			pct = (done * 100) / total
		}
		barW := 10
		filled := (pct * barW) / 100
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barW-filled)
		progressStr = lipgloss.NewStyle().
			Foreground(KaliPurpleBright).
				Render(fmt.Sprintf(" %s %d/%d", bar, done, total))
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Left, parts...)
	return StatusBar.Width(m.width).Render(tabBar + progressStr)
}
