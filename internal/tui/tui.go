// Package tui — entry point.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/k0-agent/k0/internal/config"
)

// Run launches the main interactive TUI.
func Run(cfg *config.Config) error {
	m := newModel(cfg)
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		// No mouse capture — allows text selection for copy/paste
	)
	_, err := p.Run()
	return err
}

// RunSetup launches the first-time setup wizard TUI.
func RunSetup(cfg *config.Config) error {
	m := newSetupModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
