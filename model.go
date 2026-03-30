// Package tui — model.go
// Root Bubble Tea model.
package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/k0-agent/k0/internal/agent"
	"github.com/k0-agent/k0/internal/config"
)

type view int

const (
	viewChat view = iota
	viewMemory
	viewSettings
)

type model struct {
	cfg          *config.Config
	width        int
	height       int
	activeView   view
	input        textinput.Model
	chatView     *chatViewModel
	memoryView   memoryViewModel
	settingsView settingsViewModel
	orchestrator *agent.Orchestrator
	statusLine   string
	version      string
	nodeLabel    string
	busy         bool // true while a goal is running — blocks new submissions
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
		settingsView: newSettingsViewModel(cfg),
		orchestrator: orch,
		statusLine:   "CHECKING",
		version:      "v0.1.0-dev",
		nodeLabel:    "LOCAL",
		activeView:   viewChat,
	}
}

func (m model) Init() tea.Cmd {
	// Do NOT arm ListenUpdates here — it creates a competing goroutine.
	// We arm it only when a goal is submitted (in the enter key handler).
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

	case tea.KeyMsg:
		// Quit keys — always handled first, regardless of focus
		switch msg.String() {
		case "ctrl+c", "ctrl+d":
			return m, tea.Quit
		case "q", "Q":
			// Quit only when input is empty (not mid-typing)
			if m.input.Value() == "" {
				return m, tea.Quit
			}
		case "esc":
			// Clear input / defocus — allows q to quit
			m.input.SetValue("")
			m.input.Blur()
			m.input.Focus()
		case "tab":
			m.activeView = (m.activeView + 1) % 3
		case "enter":
			if !m.busy && m.input.Focused() && m.input.Value() != "" {
				goal := m.input.Value()
				m.input.SetValue("")
				m.busy = true
				cmds = append(cmds,
					m.orchestrator.SubmitGoal(goal),
					m.orchestrator.ListenUpdates(),
				)
			}
		}

	case agent.TaskUpdateMsg:
		m.chatView.appendUpdate(msg)
		m.statusLine = "BUSY"
		// Re-arm listener for next message
		cmds = append(cmds, m.orchestrator.ListenUpdates())

	case agent.TaskDoneMsg:
		m.chatView.appendDone(msg)
		m.statusLine = "READY"
		m.busy = false // unlock for next goal
		// Done — do NOT re-arm listener
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

func (m model) View() string {
	if m.width == 0 {
		return "Loading K-0..."
	}

	contentH := m.height - 14
	if contentH < 4 {
		contentH = 4
	}

	logo := RenderLogo(m.width)
	statusDot := centre(RenderStatusDot(m.statusLine, m.version, m.nodeLabel), m.width)
	tabs := m.renderTabs()

	var content string
	switch m.activeView {
	case viewChat:
		content = m.chatView.View(m.width-4, contentH)
	case viewMemory:
		content = m.memoryView.View(m.width-4, contentH)
	case viewSettings:
		content = m.settingsView.View(m.width-4, contentH)
	}

	inputLine := InputBar.Width(m.width - 4).Render(
		Prompt.Render("[🐉] K-0> ") + m.input.View(),
	)

	hints := StatusBar.Width(m.width).Render(
		"  tab: panels  •  q/ctrl+d: quit  •  esc: clear  •  enter: submit goal",
	)

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
	labels := []string{"Chat", "Memory", "Settings"}
	tabs := make([]string, len(labels))
	for i, label := range labels {
		if view(i) == m.activeView {
			tabs[i] = lipgloss.NewStyle().
				Foreground(KaliPurple).
				Bold(true).
				Underline(true).
				Padding(0, 2).
				Render(label)
		} else {
			tabs[i] = lipgloss.NewStyle().
				Foreground(TextMuted).
				Padding(0, 2).
				Render(label)
		}
	}
	return StatusBar.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Left, tabs...),
	)
}
