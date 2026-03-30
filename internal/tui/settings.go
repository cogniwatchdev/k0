// Package tui — settings.go (moved from views/settings.go)
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/k0-agent/k0/internal/config"
)

type settingsViewModel struct {
	cfg    *config.Config
	cursor int
	fields []settingField
}

type settingField struct {
	Label string
	Key   string
	Value string
}

func newSettingsViewModel(cfg *config.Config) settingsViewModel {
	return settingsViewModel{
		cfg: cfg,
		fields: []settingField{
			{Label: "Ollama Address", Key: "ollama_addr", Value: cfg.OllamaAddr},
			{Label: "Model", Key: "model", Value: cfg.Model},
			{Label: "Memory Path", Key: "memory_path", Value: cfg.MemoryPath},
			{Label: "Semantic Memory", Key: "semantic_memory", Value: fmt.Sprintf("%v", cfg.SemanticMemory)},
			{Label: "Web Search", Key: "web_search_enabled", Value: fmt.Sprintf("%v", cfg.WebSearchEnabled)},
			{Label: "Summarize Every (mins)", Key: "summarize_every_mins", Value: fmt.Sprintf("%d", cfg.SummarizeEvery)},
		},
	}
}

func (m settingsViewModel) Update(msg tea.Msg) (settingsViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.fields)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m settingsViewModel) View(width, height int) string {
	var sb strings.Builder
	sb.WriteString(SectionTitle.Render("Settings") + "\n")
	sb.WriteString(Divider.Render(strings.Repeat("─", width-4)) + "\n\n")

	for i, f := range m.fields {
		label := fmt.Sprintf("  %-28s", f.Label)
		if i == m.cursor {
			sb.WriteString(Prompt.Render("▶ ") + AgentLabel.Render(label) +
				FindingLow.Render(f.Value) + "\n")
		} else {
			sb.WriteString(ToolLine.Render("  "+label) +
				lipgloss.NewStyle().Foreground(TextPrimary).Render(f.Value) + "\n")
		}
	}

	sb.WriteString("\n" + KeyHint.Render("↑↓ navigate  •  enter: edit  •  s: save"))
	return Panel.Width(width).Height(height).Render(sb.String())
}
