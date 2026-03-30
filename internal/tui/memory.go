// Package tui — memory.go (moved from views/memory.go)
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/k0-agent/k0/internal/config"
)

type memoryTab int

const (
	memTabEpisodes memoryTab = iota
	memTabKnowledge
	memTabReports
)

type memoryViewModel struct {
	cfg *config.Config
	tab memoryTab
}

func newMemoryViewModel(cfg *config.Config) memoryViewModel {
	return memoryViewModel{cfg: cfg}
}

func (m memoryViewModel) Update(msg tea.Msg) (memoryViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "1":
			m.tab = memTabEpisodes
		case "2":
			m.tab = memTabKnowledge
		case "3":
			m.tab = memTabReports
		}
	}
	return m, nil
}

func (m memoryViewModel) View(width, height int) string {
	tabs := []struct{ key, label string }{
		{"1", "Episodes"}, {"2", "Knowledge"}, {"3", "Reports"},
	}
	tabParts := make([]string, len(tabs))
	for i, t := range tabs {
		if memoryTab(i) == m.tab {
			tabParts[i] = lipgloss.NewStyle().Foreground(KaliPurple).Bold(true).
				Padding(0, 1).Render(t.key + ":" + t.label)
		} else {
			tabParts[i] = ToolLine.Padding(0, 1).Render(t.key + ":" + t.label)
		}
	}
	header := strings.Join(tabParts, "  ")

	var body string
	switch m.tab {
	case memTabEpisodes:
		body = SectionTitle.Render("Recent Episodes") + "\n\n" +
			ToolLine.Render("No episodes yet. Run your first _goal: to begin.")
	case memTabKnowledge:
		body = SectionTitle.Render("Knowledge Base") + "\n\n" +
			ToolLine.Render("Knowledge entries appear here after goals complete.")
	case memTabReports:
		body = SectionTitle.Render("Provisional Reports") + "\n\n" +
			ToolLine.Render("Reports saved at: ~/.kiai/memory/reports/")
	}

	divider := Divider.Render(strings.Repeat("─", width-4))
	content := header + "\n" + divider + "\n" + body
	return Panel.Width(width).Height(height).Render(content)
}
