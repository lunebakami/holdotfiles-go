package models

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lunebakami/holdotfiles-go/internal/storage"
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
	return []key.Binding{k.Help, k.Quit, k.Tab, k.StartSync}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Help, k.Quit}, {k.Tab, k.StartSync, k.StopSync}}
}

var keys = keyMap{
	Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "ajuda")),
	Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "sair")),
	Tab:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "alternar visão")),
	StartSync: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sincronizar")),
	StopSync:  key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "cancelar")),
}

type appState int

const (
	stateConfig appState = iota
	stateMonitor
	stateSync
	stateRestore
)

type syncFinishedMsg struct {
	result storage.Result
	err    error
}

type AppModel struct {
	state       appState
	width       int
	height      int
	help        help.Model
	spinner     spinner.Model
	showHelp    bool
	configFile  string
	paths       []string
	syncStatus  string
	lastError   string
	syncing     bool
	cancel      context.CancelFunc
	syncer      storage.Syncer
	restore     restoreState
	quitting    bool
	configError string
}

func NewAppModel(syncer storage.Syncer, paths []string, configFile string) AppModel {
	indicator := spinner.New()
	indicator.Spinner = spinner.Dot
	indicator.Style = styles.TipStyle
	return AppModel{
		state:      stateConfig,
		help:       help.New(),
		spinner:    indicator,
		configFile: configFile,
		paths:      append([]string(nil), paths...),
		syncStatus: "Pronto para sincronizar",
		syncer:     syncer,
		restore:    newRestoreState(),
	}
}

func (m AppModel) Init() tea.Cmd { return nil }

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if updated, cmd, handled := m.updateRestore(msg); handled {
		return updated, cmd
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			if m.cancel != nil {
				m.cancel()
			}
			if m.syncing {
				m.quitting = true
				m.syncStatus = "Encerrando operação..."
				return m, nil
			}
			m.clearPreview()
			return m, tea.Quit
		case key.Matches(msg, keys.Help):
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(msg, keys.Tab):
			if m.syncing {
				return m, nil
			}
			m.state = (m.state + 1) % 4
			if m.state == stateRestore && !m.restore.loaded {
				return m.startListing()
			}
			return m, nil
		case key.Matches(msg, keys.StartSync):
			if m.syncing {
				return m, nil
			}
			if len(m.paths) == 0 {
				m.state = stateSync
				m.syncStatus = "Configure os caminhos antes de enviar"
				m.lastError = m.configError
				return m, nil
			}
			m.clearPreview()
			ctx, cancel := context.WithCancel(context.Background())
			m.cancel = cancel
			m.syncing = true
			m.lastError = ""
			m.syncStatus = "Compactando e enviando backup.zip..."
			m.state = stateSync
			return m, tea.Batch(m.spinner.Tick, syncCmd(ctx, m.syncer, m.paths))
		case key.Matches(msg, keys.StopSync):
			if m.syncing && m.cancel != nil {
				m.cancel()
				m.syncStatus = "Cancelando..."
			}
			return m, nil
		}
	case spinner.TickMsg:
		if !m.syncing {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case syncFinishedMsg:
		m.syncing = false
		if m.cancel != nil {
			m.cancel()
		}
		m.cancel = nil
		if m.quitting {
			m.clearPreview()
			return m, tea.Quit
		}
		m.lastError = ""
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			m.lastError = shorten(msg.err.Error(), 240)
		}
		summary := fmt.Sprintf("%d enviados, %d inalterados, %d falhas (ZIP)", msg.result.Uploaded, msg.result.Skipped, msg.result.Failed)
		switch {
		case errors.Is(msg.err, context.Canceled):
			m.syncStatus = "Sincronização cancelada — " + summary
		case msg.err != nil:
			m.syncStatus = "Falha na sincronização — " + summary
		case msg.result.Uploaded == 0 && msg.result.Skipped == 0:
			m.syncStatus = "Nenhum arquivo encontrado"
		default:
			m.syncStatus = "Sincronização concluída — " + summary
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
	}
	return m, nil
}

func syncCmd(ctx context.Context, syncer storage.Syncer, paths []string) tea.Cmd {
	return func() tea.Msg {
		result, err := syncer.Sync(ctx, paths)
		return syncFinishedMsg{result: result, err: err}
	}
}

func (m AppModel) View() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	if width < 36 {
		return "Holdotfiles\nAmplie o terminal (mín. 36 colunas).\nq sair"
	}
	frameWidth := min(width-2, 104)
	header := styles.HeaderStyle.Render("◈  Holdotfiles") + "  " + styles.Muted.Render("seus dotfiles, em qualquer lugar")
	var tabs []string
	labels := []string{"Visão geral", "Arquivos", "Backup", "Restaurar"}
	if width < 65 {
		labels = []string{"Geral", "Arquivos", "Backup", "Restaurar"}
	}
	for i, label := range labels {
		style := styles.Tab
		if int(m.state) == i {
			style = styles.ActiveTab
		}
		tabs = append(tabs, style.Render(label))
	}
	navigation := strings.Join(tabs, " ")

	var content string
	switch m.state {
	case stateConfig:
		content = m.renderConfigView()
	case stateMonitor:
		content = m.renderMonitorView()
	case stateSync:
		content = m.renderSyncView()
	case stateRestore:
		content = m.renderRestoreView()
	}

	statusText := "●  " + m.syncStatus
	if m.syncing {
		statusText = m.spinner.View() + " " + statusText
	}
	statusStyle := styles.StatusStyle
	if m.lastError != "" {
		statusStyle = statusStyle.Foreground(lipgloss.Color("#FDA4AF"))
	}
	status := statusStyle.Width(frameWidth).Render(statusText)
	helpView := styles.FooterStyle.Render("tab navegar   s enviar   r restaurar   ? ajuda   q sair")
	if m.syncing {
		helpView = styles.FooterStyle.Render("x cancelar operação   q cancelar e sair")
	}
	if m.showHelp {
		helpView = styles.FooterStyle.Render("tab trocar tela • s enviar ZIP • r listar backups\n↑/↓ selecionar • enter prévia • i instalar • esc voltar\nx cancelar operação • q sair • ? fechar ajuda")
	}
	panel := styles.Panel.Width(max(1, frameWidth-2)).Render(content)
	layout := lipgloss.JoinVertical(lipgloss.Left, header, "", navigation, panel, status, "", helpView)
	return lipgloss.NewStyle().Margin(0, 1).MaxWidth(width).Render(layout)
}

func (m AppModel) renderConfigView() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		styles.TitleStyle.Render("Tudo pronto para levar seus dotfiles"),
		styles.Badge.Render("CLOUDFLARE R2")+"  "+styles.Badge.Render("ZIP"),
		"",
		styles.Muted.Render("DESTINO"),
		styles.TextStyle.Render(m.syncer.Target()),
		"",
		styles.Muted.Render("CONFIGURAÇÃO"),
		styles.TextStyle.Render(m.configFile),
		styles.Muted.Render(fmt.Sprintf("%d caminhos selecionados", len(m.paths))),
		"",
		styles.TipStyle.Render("s  Criar backup")+"    "+styles.TipStyle.Render("r  Restaurar backup"),
	)
}

func (m AppModel) renderMonitorView() string {
	lines := make([]string, 0, len(m.paths))
	for _, filename := range m.paths {
		lines = append(lines, styles.Muted.Render("  › ")+styles.FileStyle.Render(filename))
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		styles.TitleStyle.Render("Arquivos configurados"),
		styles.Muted.Render("Pastas incluem seus arquivos e subpastas.")+"\n",
		styles.TextStyle.Render(strings.Join(lines, "\n")),
	)
}

func (m AppModel) renderSyncView() string {
	tip := "s  Compactar e enviar"
	if m.syncing {
		tip = "x  Cancelar operação"
	}
	content := []string{
		styles.TitleStyle.Render("Seu próximo backup"),
		styles.Muted.Render("COMPACTAR  →  VERIFICAR  →  ENVIAR"),
		"",
		styles.TextStyle.Render(m.syncer.Target()),
		styles.Muted.Render("backup.zip • caminhos relativos à sua pasta pessoal"),
		"",
		styles.TipStyle.Render(tip),
	}
	if m.lastError != "" {
		content = append(content, styles.ErrorStyle.Render("Erro: "+m.lastError))
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		content...,
	)
}

func shorten(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}
