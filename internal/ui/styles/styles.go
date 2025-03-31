package styles

import "github.com/charmbracelet/lipgloss"

var (
	primaryColor   = lipgloss.Color("#7D56F4")
	secondaryColor = lipgloss.Color("#2D3748")
	accentColor    = lipgloss.Color("#F472B6")
	textColor      = lipgloss.Color("#E2E8F0")
	errorColor     = lipgloss.Color("#FC8181")
	successColor   = lipgloss.Color("#68D391")

	HeaderStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(primaryColor).
			Bold(true).
			Padding(0, 1).
			Width(100).
			Align(lipgloss.Center)

	TitleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginTop(1).
			MarginBottom(1)

	TextStyle = lipgloss.NewStyle().
			Foreground(textColor)

	StatusStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(secondaryColor).
			Padding(0, 1)

	FooterStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Italic(true)

	FileStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	TipStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Italic(true).
			MarginTop(1)
)
