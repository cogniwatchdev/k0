// Package tui — chat.go
// Chat view model — displays agent messages, plans, and activity indicators.
package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/k0-agent/k0/internal/agent"
)

// thinkingPhrases cycle to show the user K-0 is alive.
var thinkingPhrases = []string{
	"analysing",
	"scanning",
	"thinking",
	"processing",
	"enumerating",
	"inspecting",
	"correlating",
	"probing",
}

type chatLine struct {
	Label     string
	Text      string
	Timestamp time.Time
	IsDone    bool
	IsSystem  bool
	IsPlan    bool // plan block — multi-line, formatted
}

type chatViewModel struct {
	lines []chatLine
}

func newChatViewModel() chatViewModel {
	return chatViewModel{}
}

func (m *chatViewModel) Update(msg tea.Msg) (*chatViewModel, tea.Cmd) {
	return m, nil
}

func (m *chatViewModel) clear() {
	m.lines = nil
}

func (m *chatViewModel) appendUpdate(msg agent.TaskUpdateMsg) {
	m.lines = append(m.lines, chatLine{
		Label:     msg.AgentLabel,
		Text:      msg.Line,
		Timestamp: msg.Timestamp,
	})
}

func (m *chatViewModel) appendDone(msg agent.TaskDoneMsg) {
	m.lines = append(m.lines, chatLine{
		Label:  "[K-0]",
		Text:   "✅ All tasks complete.",
		IsDone: true,
	})
	if msg.Summary != "" {
		m.lines = append(m.lines, chatLine{Label: "[K-0]", Text: msg.Summary})
	}
}

// appendPlan renders the LLM plan as a formatted block in the chat.
func (m *chatViewModel) appendPlan(plan agent.PlanProposal) {
	// Header
	m.lines = append(m.lines, chatLine{
		Label: "[K-0]",
		Text:  "═══ PROPOSED PLAN ═══",
		IsPlan: true,
	})

	// Scope
	m.lines = append(m.lines, chatLine{
		Label:  "     ",
		Text:   fmt.Sprintf("Scope:     %s", plan.Scope),
		IsPlan: true,
	})

	// Approach
	m.lines = append(m.lines, chatLine{
		Label:  "     ",
		Text:   fmt.Sprintf("Approach:  %s", plan.Approach),
		IsPlan: true,
	})

	// Tool availability
	if len(plan.ToolChecks) > 0 {
		m.lines = append(m.lines, chatLine{Label: "     ", Text: "", IsPlan: true})
		m.lines = append(m.lines, chatLine{Label: "     ", Text: "Tools:", IsPlan: true})
		for _, tc := range plan.ToolChecks {
			status := "✓"
			if !tc.Available {
				status = "✗ MISSING"
			}
			m.lines = append(m.lines, chatLine{
				Label:    "     ",
				Text:     fmt.Sprintf("  %s %s", status, tc.Tool),
				IsSystem: !tc.Available, // highlight missing in muted
				IsPlan:   tc.Available,
			})
		}
	}

	m.lines = append(m.lines, chatLine{Label: "     ", Text: "", IsPlan: true})

	// Phases
	for _, phase := range plan.Phases {
		m.lines = append(m.lines, chatLine{
			Label:  "     ",
			Text:   fmt.Sprintf("Phase %d:  %s", phase.ID, phase.Purpose),
			IsPlan: true,
		})
		if phase.Command != "" {
			est := phase.Estimate
			if est == "" {
				est = "unknown"
			}
			m.lines = append(m.lines, chatLine{
				Label:    "     ",
				Text:     fmt.Sprintf("          $ %s  (%s)", phase.Command, est),
				IsSystem: true,
			})
		}
	}

	m.lines = append(m.lines, chatLine{Label: "     ", Text: "", IsPlan: true})

	// Estimate + risks
	est := plan.TotalEstimate
	if est == "" {
		est = "unknown"
	}
	m.lines = append(m.lines, chatLine{
		Label:  "     ",
		Text:   fmt.Sprintf("⏱  Estimated: %s", est),
		IsPlan: true,
	})

	if plan.Risks != "" {
		m.lines = append(m.lines, chatLine{
			Label:    "     ",
			Text:     fmt.Sprintf("⚠  %s", plan.Risks),
			IsSystem: true,
		})
	}

	// Missing tools warning
	if len(plan.MissingTools) > 0 {
		m.lines = append(m.lines, chatLine{
			Label: "[K-0]",
			Text:  fmt.Sprintf("⚠  Missing tools: %s — install with 'i' or proceed without", strings.Join(plan.MissingTools, ", ")),
			IsPlan: true,
		})
	}

	// Confirmation prompt
	m.lines = append(m.lines, chatLine{
		Label: "[K-0]",
		Text:  "Proceed? (y/n)",
		IsPlan: true,
	})
}

func (m *chatViewModel) View(width, height int, busy bool, spinnerIdx int, busyStart time.Time) string {
	if len(m.lines) == 0 && !busy {
		empty := centreText(
			lipgloss.NewStyle().Foreground(TextMuted).Render("Type _goal: <objective> to start"),
			width, height,
		)
		return PanelFocused.Width(width).Height(height).Render(empty)
	}

	// Reserve 1 line for the thinking indicator when busy
	availLines := height - 2
	if busy {
		availLines--
	}
	if availLines < 1 {
		availLines = 1
	}

	start := 0
	if len(m.lines) > availLines {
		start = len(m.lines) - availLines
	}

	var sb strings.Builder
	for _, line := range m.lines[start:] {
		var label, text string
		if line.IsDone {
			label = lipgloss.NewStyle().Foreground(StatusOK).Bold(true).Render(line.Label)
			text = lipgloss.NewStyle().Foreground(StatusOK).Render(" " + line.Text)
		} else if line.IsPlan {
			label = lipgloss.NewStyle().Foreground(KaliPurpleBright).Bold(true).Render(line.Label)
			text = lipgloss.NewStyle().Foreground(KaliPurpleBright).Render(" " + line.Text)
		} else if line.IsSystem {
			label = lipgloss.NewStyle().Foreground(TextMuted).Render(line.Label)
			text = lipgloss.NewStyle().Foreground(TextMuted).Render(" " + line.Text)
		} else {
			label = AgentLabel.Render(line.Label)
			text = ToolLine.Render(" " + line.Text)
		}
		sb.WriteString(label + text + "\n")
	}

	// Thinking indicator at bottom
	if busy {
		elapsed := time.Since(busyStart).Round(time.Second)
		phrase := thinkingPhrases[(spinnerIdx/2)%len(thinkingPhrases)]
		dots := strings.Repeat(".", (spinnerIdx%3)+1) + strings.Repeat(" ", 2-(spinnerIdx%3))

		spinner := lipgloss.NewStyle().Foreground(KaliPurple).Render(spinnerFrames[spinnerIdx])
		thinkText := lipgloss.NewStyle().Foreground(KaliPurpleDim).Italic(true).Render(
			fmt.Sprintf(" %s%s [%s]", phrase, dots, elapsed),
		)
		sb.WriteString(spinner + thinkText + "\n")
	}

	return PanelFocused.Width(width).Height(height).Render(sb.String())
}

func centreText(s string, w, h int) string {
	padTop := (h - 1) / 2
	raw := stripANSI(s)
	padLeft := (w - len(raw)) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	return strings.Repeat("\n", padTop) + strings.Repeat(" ", padLeft) + s
}
