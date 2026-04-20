// Package tui — memory.go (moved from views/memory.go)
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/k0-agent/k0/internal/config"
	"github.com/k0-agent/k0/internal/memory"
)

type memoryTab int

const (
	memTabEpisodes memoryTab = iota
	memTabKnowledge
	memTabReports
)

type memoryViewModel struct {
	cfg    *config.Config
	tab    memoryTab
	store  *memory.Store
	scroll int
	// itemCount caches the number of items in the current tab for scroll clamping.
	itemCount int
}

func newMemoryViewModel(cfg *config.Config) memoryViewModel {
	return memoryViewModel{
		cfg:   cfg,
		store: memory.NewStore(cfg),
	}
}

func (m memoryViewModel) Update(msg tea.Msg) (memoryViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "1":
			m.tab = memTabEpisodes
			m.scroll = 0
		case "2":
			m.tab = memTabKnowledge
			m.scroll = 0
		case "3":
			m.tab = memTabReports
			m.scroll = 0
		case "up", "k":
			if m.scroll > 0 {
				m.scroll--
			}
		case "down", "j":
			if m.scroll < m.itemCount-1 {
				m.scroll++
			}
		}
	}
	return m, nil
}

// clampScroll ensures scroll stays within [0, max-1].
func (m *memoryViewModel) clampScroll(max int) {
	m.itemCount = max
	if max <= 0 {
		m.scroll = 0
		m.itemCount = 0
		return
	}
	if m.scroll >= max {
		m.scroll = max - 1
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
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

	// Inner height available after Panel borders (2) and our own header+divider (2).
	// Panel.Height(h) renders h+2 lines total; inner content area = h-2 rows.
	// We use 2 rows for header + divider, leaving (h - 4) rows for body.
	innerBodyLines := height - 4
	if innerBodyLines < 1 {
		innerBodyLines = 1
	}

	var body string
	switch m.tab {
	case memTabEpisodes:
		body = m.renderEpisodes(innerBodyLines)
	case memTabKnowledge:
		body = m.renderKnowledge(innerBodyLines)
	case memTabReports:
		body = m.renderReports(innerBodyLines)
	}

	divider := Divider.Render(strings.Repeat("-", clampWidth(width-4, 10)))
	content := header + "\n" + divider + "\n" + body

	return Panel.Width(width).Height(height).Render(content)
}

// clampWidth ensures width is at least min.
func clampWidth(w, min int) int {
	if w < min {
		return min
	}
	return w
}

func (m memoryViewModel) renderEpisodes(maxLines int) string {
	episodes := m.store.ListEpisodes(50)

	// Clamp scroll to episode count
	m.clampScroll(len(episodes))

	if len(episodes) == 0 {
		return SectionTitle.Render("Recent Episodes") + "\n" +
			ToolLine.Render("No episodes yet. Run your first goal to begin.")
	}

	var b strings.Builder
	b.WriteString(SectionTitle.Render(fmt.Sprintf("Recent Episodes (%d)", len(episodes))))
	b.WriteString("\n")

	start := m.scroll
	lines := 0
	for i := start; i < len(episodes) && lines < maxLines; i++ {
		ep := episodes[i]
		outcomeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#2ecc71"))
		if ep.Outcome != "completed" {
			outcomeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#e74c3c"))
		}
		date := ep.StartTime.Format("2006-01-02 15:04")
		tags := ""
		if len(ep.Tags) > 0 {
			tags = fmt.Sprintf(" [%s]", strings.Join(ep.Tags, ","))
		}
		// Truncate goal if too long to prevent horizontal overflow
		goalText := ep.Goal
		if len(goalText) > 60 {
			goalText = goalText[:57] + "..."
		}
		line := fmt.Sprintf("%s  %s  %dt%s  %s",
			ToolLine.Render(date),
			outcomeStyle.Render(ep.Outcome),
			ep.Tasks,
			ToolLine.Render(tags),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#aaaaaa")).Render(goalText),
		)
		b.WriteString(line)
		b.WriteString("\n")
		lines++
	}

	if m.scroll > 0 {
		b.WriteString(ToolLine.Render("  ^ scroll up for more"))
	}
	if len(episodes)-m.scroll > maxLines {
		b.WriteString(ToolLine.Render("  v scroll down for more"))
	}
	return b.String()
}

func (m memoryViewModel) renderKnowledge(maxLines int) string {
	entries := m.store.ListKnowledge(50)

	// Clamp scroll to entry count
	m.clampScroll(len(entries))

	if len(entries) == 0 {
		return SectionTitle.Render("Knowledge Base") + "\n" +
			ToolLine.Render("Knowledge entries appear here after goals complete.")
	}

	var b strings.Builder
	b.WriteString(SectionTitle.Render(fmt.Sprintf("Knowledge Base (%d entries)", len(entries))))
	b.WriteString("\n")

	start := m.scroll
	lines := 0
	for i := start; i < len(entries) && lines < maxLines; i++ {
		e := entries[i]
		catStyle := lipgloss.NewStyle().Foreground(KaliPurple).Bold(true)
		date := e.CreatedAt.Format("2006-01-02")
		// Truncate summary if too long
		summaryText := e.Summary
		if len(summaryText) > 70 {
			summaryText = summaryText[:67] + "..."
		}
		line := fmt.Sprintf("%s  %s  %s",
			ToolLine.Render(date),
			catStyle.Render(fmt.Sprintf("[%s]", e.Category)),
			ToolLine.Render(summaryText),
		)
		b.WriteString(line)
		b.WriteString("\n")
		lines++
	}

	if m.scroll > 0 {
		b.WriteString(ToolLine.Render("  ^ scroll up for more"))
	}
	if len(entries)-m.scroll > maxLines {
		b.WriteString(ToolLine.Render("  v scroll down for more"))
	}
	return b.String()
}

func (m memoryViewModel) renderReports(maxLines int) string {
	reports := m.store.ListReports(50)

	// Clamp scroll to report count
	m.clampScroll(len(reports))

	if len(reports) == 0 {
		return SectionTitle.Render("Provisional Reports") + "\n" +
			ToolLine.Render("Reports saved at: ~/.kiai/memory/reports/")
	}

	var b strings.Builder
	b.WriteString(SectionTitle.Render(fmt.Sprintf("Provisional Reports (%d)", len(reports))))
	b.WriteString("\n")

	start := m.scroll
	lines := 0
	for i := start; i < len(reports) && lines < maxLines; i++ {
		name := reports[i]
		// Use ASCII bullet instead of Unicode emoji to avoid width issues
		b.WriteString(ToolLine.Render(fmt.Sprintf("  [R] %s", name)))
		b.WriteString("\n")
		lines++
	}

	b.WriteString("\n")
	b.WriteString(ToolLine.Render("  View: cat ~/.kiai/memory/reports/<filename>"))

	return b.String()
}