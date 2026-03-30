// Package tui — setup.go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/k0-agent/k0/internal/config"
)

type setupStep int

const (
	stepWelcome setupStep = iota
	stepModel
	stepMemory
	stepWebSearch
	stepDone
)

type setupModel struct {
	cfg  *config.Config
	step setupStep
}

func newSetupModel(cfg *config.Config) setupModel {
	return setupModel{cfg: cfg, step: stepWelcome}
}

func (m setupModel) Init() tea.Cmd { return nil }

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "1":
			m.applyStep(1)
			if m.step < stepDone {
				m.step++
			}
		case "2":
			m.applyStep(2)
			if m.step < stepDone {
				m.step++
			}
		case "enter", " ":
			m.applyStep(1) // default = option 1
			if m.step < stepDone {
				m.step++
			} else {
				_ = config.Save(m.cfg)
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m *setupModel) applyStep(choice int) {
	switch m.step {
	case stepModel:
		if choice == 1 {
			m.cfg.OllamaAddr = "http://127.0.0.1:11435"
			m.cfg.Model = "nemotron-pentest-k0:latest"
		}
	case stepMemory:
		m.cfg.SemanticMemory = choice == 2
	case stepWebSearch:
		m.cfg.WebSearchEnabled = choice == 2
	case stepDone:
		_ = config.Save(m.cfg)
	}
}

func (m setupModel) View() string {
	accent := lipgloss.NewStyle().Foreground(KaliPurple).Bold(true)
	dim := lipgloss.NewStyle().Foreground(TextSecondary)
	ok := lipgloss.NewStyle().Foreground(StatusOK).Bold(true)

	switch m.step {
	case stepWelcome:
		return Panel.Render(
			accent.Render("Welcome to K-0 Setup") + "\n\n" +
				dim.Render("This wizard configures your K-0 agent.\n\n") +
				KeyHint.Render("enter: next  •  q: quit"),
		)
	case stepModel:
		return Panel.Render(
			accent.Render("[1] Model Selection") + "\n\n" +
				accent.Render("  1 ") + dim.Render("Bundled local model (offline, recommended)\n") +
				dim.Render("  2   Custom Ollama instance\n\n") +
				KeyHint.Render("1/2: select  •  enter: default (1)"),
		)
	case stepMemory:
		return Panel.Render(
			accent.Render("[2] Memory Mode") + "\n\n" +
				accent.Render("  1 ") + dim.Render("Episodic + knowledge (recommended)\n") +
				dim.Render("  2   + Semantic memory (~500MB download)\n\n") +
				KeyHint.Render("1/2: select  •  enter: default (1)"),
		)
	case stepWebSearch:
		return Panel.Render(
			accent.Render("[3] Passive Recon") + "\n\n" +
				accent.Render("  1 ") + dim.Render("Offline mode (no web search)\n") +
				dim.Render("  2   Enable web search (requires internet)\n\n") +
				KeyHint.Render("1/2: select  •  enter: default (1)"),
		)
	default:
		return Panel.Render(
			ok.Render("✅ Setup complete!\n\n") +
				dim.Render("Run k0 to launch the agent.\n\n") +
				KeyHint.Render("enter: exit"),
		)
	}
}
