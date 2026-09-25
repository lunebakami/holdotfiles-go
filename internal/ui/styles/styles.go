package styles

import "github.com/charmbracelet/lipgloss"

var (
	primaryColor   = lipgloss.Color("#A78BFA")
	secondaryColor = lipgloss.Color("#202838")
	accentColor    = lipgloss.Color("#67E8F9")
	textColor      = lipgloss.Color("#E2E8F0")
	errorColor     = lipgloss.Color("#FDA4AF")
	successColor   = lipgloss.Color("#6EE7B7")

	Muted = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	Panel = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#475569")).Padding(1, 2)
	Tab       = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Padding(0, 1)
	ActiveTab = lipgloss.NewStyle().Foreground(lipgloss.Color("#111827")).
			Background(primaryColor).Bold(true).Padding(0, 1)
	Selected = lipgloss.NewStyle().Foreground(accentColor).Background(secondaryColor).Bold(true)
	Badge    = lipgloss.NewStyle().Foreground(accentColor).Background(secondaryColor).Padding(0, 1)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Padding(0, 1)

	TitleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(1)

	TextStyle = lipgloss.NewStyle().
			Foreground(textColor)

	StatusStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(secondaryColor).
			Padding(0, 1)

	FooterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8"))

	FileStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	TipStyle = lipgloss.NewStyle().
			Foreground(successColor)
)
