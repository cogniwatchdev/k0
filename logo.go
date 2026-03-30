// Package tui — logo.go
package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	logoBannerStyle = lipgloss.NewStyle().Foreground(KaliPurple).Bold(true)
	logoTagStyle    = lipgloss.NewStyle().Foreground(TextSecondary)
	logoSubStyle    = lipgloss.NewStyle().Foreground(KaliPurpleDim)
)

// RenderLogo returns the K-0 logo centred in width.
// Uses Unicode box-drawing chars — 8 lines total (6 art + divider + subtitle).
func RenderLogo(width int) string {
	artLines := []string{
		`██╗  ██╗       ██████╗ `,
		`██║ ██╔╝      ██╔═████╗`,
		`█████╔╝  ████╗██║██╔██║`,
		`██╔═██╗  ╚═══╝████╔╝██║`,
		`██║  ██╗      ╚██████╔╝`,
		`╚═╝  ╚═╝       ╚═════╝ `,
	}

	lines := make([]string, 0, 8)
	for _, l := range artLines {
		lines = append(lines, centre(logoBannerStyle.Render(l), width))
	}
	lines = append(lines, centre(logoSubStyle.Render(strings.Repeat("─", 32)), width))
	lines = append(lines, centre(logoTagStyle.Render("[ OPENCLAW ARCHITECTURE ]"), width))
	return strings.Join(lines, "\n")
}

// RenderStatusDot renders the status indicator line.
func RenderStatusDot(status, version, node string) string {
	var colour lipgloss.Color
	switch status {
	case "READY":
		colour = StatusOK
	case "BUSY":
		colour = StatusWarning
	default:
		colour = StatusError
	}
	dot := lipgloss.NewStyle().Foreground(colour).Render("●")
	text := lipgloss.NewStyle().Foreground(TextSecondary).Render(
		" " + status + " | " + version + " | " + node,
	)
	return dot + text
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// centre pads s to be visually centred in width display columns.
func centre(s string, width int) string {
	visWidth := lipgloss.Width(s)
	pad := (width - visWidth) / 2
	if pad <= 0 {
		return s
	}
	return strings.Repeat(" ", pad) + s
}

func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}
