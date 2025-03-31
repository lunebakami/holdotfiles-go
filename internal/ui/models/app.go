package models

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lunebakami/holdotfiles-go/internal/ui/styles"
)

type keyMap struct {
	Help      key.Binding
	Quit      key.Binding
	Tab       key.Binding
	StartSync key.Binding
	StopSync  key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit, k.Tab}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Help, k.Quit},
		{k.Tab, k.StartSync, k.StopSync},
	}
}

var keys = keyMap{
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "ajuda"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "sair"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "alternar visões"),
	),
	StartSync: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "iniciar sincronização"),
	),
	StopSync: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "parar sincronização"),
	),
}

type appState int

const (
	stateConfig appState = iota
	stateMonitor
	stateSync
)

type AppModel struct {
	state       appState
	width       int
	height      int
	help        help.Model
	showHelp    bool
	sourceDir   string
	targetDir   string
	syncStatus  string
	activeFiles []string
}

func NewAppModel() AppModel {
	return AppModel{
		state:       stateConfig,
		help:        help.New(),
		showHelp:    false,
		sourceDir:   "",
		targetDir:   "",
		syncStatus:  "Pronto pra configurar",
		activeFiles: []string{},
	}
}

func (m AppModel) Init() tea.Cmd {
	return nil
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Help):
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(msg, keys.Tab):
			m.state = (m.state + 1) % 3
			return m, nil
		case key.Matches(msg, keys.StartSync):
			if m.sourceDir != "" && m.targetDir != "" {
				m.syncStatus = "Sincronizando..."
				// call sync
				return m, nil
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
	}
	return m, nil
}

func (m AppModel) View() string {
	var content string

	header := styles.HeaderStyle.Render("Holdotfiles - Sync your dotfiles")

	switch m.state {
	case stateConfig:
		content = renderConfigView(m)
	case stateMonitor:
		content = renderMonitorView(m)
	case stateSync:
		content = renderSyncView(m)
	}

	status := styles.StatusStyle.Render(fmt.Sprintf("Status: %s", m.syncStatus))
	helpView := ""
	if m.showHelp {
		helpView = m.help.View(keys)
	} else {
		helpView = styles.FooterStyle.Render("? para ajuda • q para sair")
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		content,
		status,
		helpView,
	)
}

func renderConfigView(m AppModel) string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		styles.TitleStyle.Render("Configuração"),
		styles.TextStyle.Render("Origem: "+m.sourceDir),
		styles.TextStyle.Render("Destino: "+m.targetDir),
		styles.TipStyle.Render("Presione 'o' para selecionar origem, 'd' para destino"),
	)
}

func renderMonitorView(m AppModel) string {
	fileList := "Nenhum arquivo monitorado"
	if len(m.activeFiles) > 0 {
		fileList = ""
		for _, f := range m.activeFiles {
			fileList += styles.FileStyle.Render(f) + "\n"
		}
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		styles.TitleStyle.Render("Monitoramento"),
		styles.TextStyle.Render(fileList),
	)
}

func renderSyncView(m AppModel) string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		styles.TitleStyle.Render("Sincronização"),
		styles.TextStyle.Render("Status: "+m.syncStatus),
		styles.TipStyle.Render("Presione 's' para iniciar, 'x' para parar"),
	)
}
