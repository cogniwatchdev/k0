// Package tui — theme.go
// Central design tokens for K-0's Kali Purple visual identity.
// All styles are defined here; views import from this file only.
package tui

import "github.com/charmbracelet/lipgloss"

// ── Palette ────────────────────────────────────────────────────────────────

var (
	// KaliPurple is the brand accent (#A37DFF — 24-bit Kali purple)
	KaliPurple = lipgloss.Color("#A37DFF")
	// KaliPurpleDim is a muted accent for secondary elements
	KaliPurpleDim = lipgloss.Color("#6B4FBF")
	// KaliPurpleBright is used for active/focused highlights
	KaliPurpleBright = lipgloss.Color("#C4A8FF")

	// Background layers
	BgBase    = lipgloss.Color("#0D0D12") // deepest background
	BgSurface = lipgloss.Color("#131320") // panel background
	BgBorder  = lipgloss.Color("#1E1E30") // border / separator

	// Text
	TextPrimary   = lipgloss.Color("#E8E8F0")
	TextSecondary = lipgloss.Color("#8888AA")
	TextMuted     = lipgloss.Color("#4A4A6A")
	TextAccent    = KaliPurple

	// Severity colours (for findings)
	SevCritical = lipgloss.Color("#FF4444")
	SevHigh     = lipgloss.Color("#FF8C00")
	SevMedium   = lipgloss.Color("#FFD700")
	SevLow      = lipgloss.Color("#44AAFF")
	SevInfo     = lipgloss.Color("#888888")

	// Status
	StatusOK      = lipgloss.Color("#44FF88")
	StatusWarning = lipgloss.Color("#FFD700")
	StatusError   = lipgloss.Color("#FF4444")
)

// ── Borders ────────────────────────────────────────────────────────────────

// PanelBorder is the default rounded border for content panels.
var PanelBorder = lipgloss.RoundedBorder()

// ── Base Styles ────────────────────────────────────────────────────────────

var (
	// Base is the full-screen background style.
	Base = lipgloss.NewStyle().
		Background(BgBase).
		Foreground(TextPrimary)

	// Panel wraps a content area with a rounded border.
	Panel = lipgloss.NewStyle().
		Border(PanelBorder).
		BorderForeground(BgBorder).
		Background(BgSurface).
		Padding(0, 1)

	// PanelFocused highlights the active panel.
	PanelFocused = lipgloss.NewStyle().
		Border(PanelBorder).
		BorderForeground(KaliPurple).
		Background(BgSurface).
		Padding(0, 1)

	// Header is the top bar style.
	Header = lipgloss.NewStyle().
		Background(BgBase).
		Foreground(TextSecondary).
		Padding(0, 1)

	// StatusBar is the one-line status strip above the input.
	StatusBar = lipgloss.NewStyle().
		Background(BgSurface).
		Foreground(TextSecondary).
		Padding(0, 1)

	// InputBar styles the goal input field.
	InputBar = lipgloss.NewStyle().
		Background(BgBase).
		Foreground(TextPrimary).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(KaliPurpleDim).
		Padding(0, 1)

	// Prompt is the K-0> prompt prefix.
	Prompt = lipgloss.NewStyle().
		Foreground(KaliPurple).
		Bold(true)

	// AgentLabel styles [K-0], [Recon-01], etc. prefixes in output.
	AgentLabel = lipgloss.NewStyle().
		Foreground(KaliPurpleBright).
		Bold(true)

	// ToolLine styles a tool execution line (dimmer, monospaced feel).
	ToolLine = lipgloss.NewStyle().
		Foreground(TextSecondary)

	// FindingCritical / High / Medium / Low / Info
	FindingCritical = lipgloss.NewStyle().Foreground(SevCritical).Bold(true)
	FindingHigh     = lipgloss.NewStyle().Foreground(SevHigh).Bold(true)
	FindingMedium   = lipgloss.NewStyle().Foreground(SevMedium)
	FindingLow      = lipgloss.NewStyle().Foreground(SevLow)
	FindingInfo     = lipgloss.NewStyle().Foreground(SevInfo)

	// SectionTitle is a purple underlined section heading.
	SectionTitle = lipgloss.NewStyle().
		Foreground(KaliPurple).
		Bold(true).
		Underline(true)

	// Divider is a subtle horizontal rule.
	Divider = lipgloss.NewStyle().
		Foreground(BgBorder)

	// KeyHint styles keyboard shortcut hints.
	KeyHint = lipgloss.NewStyle().
		Foreground(KaliPurpleDim).
		Background(BgSurface).
		Padding(0, 1)
)
