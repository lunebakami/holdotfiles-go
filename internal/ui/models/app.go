package models

import (
	"bufio"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lunebakami/holdotfiles-go/cmd/lib"
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
	appwrite    *lib.AppwriteClient
}

func NewAppModel(appwrite *lib.AppwriteClient) AppModel {
	m := AppModel{
		state:       stateConfig,
		help:        help.New(),
		showHelp:    false,
		sourceDir:   "",
		targetDir:   "",
		syncStatus:  "Pronto pra configurar",
		activeFiles: []string{},
		appwrite:    appwrite,
	}

	m.LoadConfig()

	return m
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
				m.appwrite.Sync(m.activeFiles)
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
		styles.TextStyle.Render("Origens: "+m.sourceDir),
		styles.TextStyle.Render("Destino: "+m.targetDir),
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

func (m *AppModel) LoadConfig() {
	defaultFilePath, err := lib.ExpandPath("~/.hdtconfig")
	if err != nil {
		m.syncStatus = "Erro ao expandir o caminho do arquivo de configuração"
		return
	}

	file, err := os.Open(defaultFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open file: %v", err)
		m.syncStatus = "Erro ao abrir o arquivo de configuração"
		return 
	}
	defer file.Close()

	paths := []string{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		expandedPath, err := lib.ExpandPath(line)
		if err != nil {
			m.syncStatus = "Erro ao expandir o caminho do arquivo de configuração"
		}
		paths = append(paths, expandedPath)
	}

	if err := scanner.Err(); err != nil {
		m.syncStatus = "Erro ao ler o arquivo de configuração"
		return
	}

	m.activeFiles = paths
	m.sourceDir = defaultFilePath
	m.targetDir = "appwrite"
}
