package models

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lunebakami/holdotfiles-go/internal/backup"
	"github.com/lunebakami/holdotfiles-go/internal/storage"
	"github.com/lunebakami/holdotfiles-go/internal/ui/styles"
)

type backupBrowser interface {
	ListBackupDetails(context.Context) ([]storage.BackupInfo, error)
	Download(context.Context, string) (string, error)
}

type restoreState struct {
	destination string
	items       []storage.BackupInfo
	selected    int
	offset      int
	loaded      bool
	archive     string
	plan        backup.Plan
	computer    string
	recovery    string
}

func newRestoreState() restoreState {
	home, _ := os.UserHomeDir()
	return restoreState{destination: home}
}
func (m *AppModel) SetRestoreDestination(dest string) { m.restore.destination = dest }
func (m *AppModel) SetConfigError(err error)          { m.configError = err.Error() }

type backupsListedMsg struct {
	items []storage.BackupInfo
	err   error
}
type previewReadyMsg struct {
	archive  string
	plan     backup.Plan
	computer string
	err      error
}
type installedMsg struct {
	recovery string
	err      error
}

func (m *AppModel) clearPreview() {
	if m.restore.archive != "" {
		os.Remove(m.restore.archive)
	}
	m.restore.archive = ""
	m.restore.plan = backup.Plan{}
	m.restore.offset = 0
}

func (m *AppModel) beginRestore(status string) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.syncing = true
	m.lastError = ""
	m.syncStatus = status
	m.state = stateRestore
	return ctx
}

func (m *AppModel) finishRestore(err error) {
	m.syncing = false
	if m.cancel != nil {
		m.cancel()
	}
	m.cancel = nil
	if err != nil {
		if errors.Is(err, context.Canceled) {
			m.syncStatus = "Operação cancelada"
		} else {
			m.syncStatus = "Falha na restauração"
			m.lastError = shorten(err.Error(), 240)
		}
	}
}

func (m AppModel) startListing() (AppModel, tea.Cmd) {
	remote, ok := m.syncer.(backupBrowser)
	if !ok {
		m.lastError = "Cliente não suporta restauração"
		return m, nil
	}
	m.clearPreview()
	ctx := m.beginRestore("Consultando backups no R2...")
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		items, err := remote.ListBackupDetails(ctx)
		if err == nil {
			err = ctx.Err()
		}
		return backupsListedMsg{items, err}
	})
}

func (m AppModel) updateRestore(msg tea.Msg) (AppModel, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case backupsListedMsg:
		m.finishRestore(msg.err)
		if msg.err == nil {
			m.restore.items = msg.items
			m.restore.selected = 0
			m.restore.loaded = true
			m.syncStatus = fmt.Sprintf("%d backups disponíveis", len(msg.items))
		}
		if m.quitting {
			m.clearPreview()
			return m, tea.Quit, true
		}
		return m, nil, true
	case previewReadyMsg:
		m.finishRestore(msg.err)
		if msg.err != nil || m.quitting {
			if msg.archive != "" {
				os.Remove(msg.archive)
			}
		} else {
			m.restore.archive = msg.archive
			m.restore.plan = msg.plan
			m.restore.computer = msg.computer
			m.restore.offset = 0
			m.syncStatus = "Prévia pronta — pressione i para confirmar a instalação"
		}
		if m.quitting {
			return m, tea.Quit, true
		}
		return m, nil, true
	case installedMsg:
		m.finishRestore(msg.err)
		m.clearPreview()
		m.restore.recovery = msg.recovery
		if msg.err == nil {
			m.syncStatus = "Instalação concluída"
		}
		if m.quitting {
			return m, tea.Quit, true
		}
		return m, nil, true
	case tea.KeyMsg:
		k := msg.String()
		if k == "r" {
			if m.syncing {
				return m, nil, true
			}
			next, cmd := m.startListing()
			return next, cmd, true
		}
		if m.state != stateRestore {
			return m, nil, false
		}
		if m.syncing {
			return m, nil, false
		}
		switch k {
		case "esc":
			m.clearPreview()
			m.lastError = ""
			m.syncStatus = "Selecione um backup"
			return m, nil, true
		case "up", "k", "down", "j":
			delta := 1
			if k == "up" || k == "k" {
				delta = -1
			}
			if m.restore.archive != "" {
				m.restore.offset = max(0, min(max(0, len(m.restore.plan.Entries)-m.restoreRows()), m.restore.offset+delta))
			} else {
				m.restore.selected = max(0, min(max(0, len(m.restore.items)-1), m.restore.selected+delta))
			}
			return m, nil, true
		case "enter":
			if m.restore.archive != "" || len(m.restore.items) == 0 {
				return m, nil, true
			}
			remote, ok := m.syncer.(backupBrowser)
			if !ok {
				return m, nil, true
			}
			computer := m.restore.items[m.restore.selected].Computer
			key := m.restore.items[m.restore.selected].Key
			if key == "" {
				key = computer
			}
			destination := m.restore.destination
			ctx := m.beginRestore("Baixando e verificando o ZIP...")
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				archive, err := remote.Download(ctx, key)
				var plan backup.Plan
				if err == nil {
					plan, err = backup.Preview(archive, destination)
				}
				if err == nil {
					err = ctx.Err()
				}
				return previewReadyMsg{archive, plan, key, err}
			}), true
		case "i":
			if m.restore.archive == "" {
				return m, nil, true
			}
			archive, dest := m.restore.archive, m.restore.destination
			ctx := m.beginRestore("Instalando dotfiles...")
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				recovery, err := backup.Install(ctx, archive, dest)
				return installedMsg{recovery, err}
			}), true
		}
	}
	return m, nil, false
}

func (m AppModel) restoreRows() int {
	if m.height == 0 {
		return 10
	}
	return max(1, m.height-21)
}

func (m AppModel) renderRestoreView() string {
	lines := []string{styles.TitleStyle.Render("Escolha de onde continuar"),
		styles.Muted.Render("DESTINO") + "  " + styles.TextStyle.Render(m.restore.destination),
		styles.Muted.Render("Histórico de backups • horário local • mais recentes primeiro"), ""}
	rows := m.restoreRows()
	if m.restore.archive != "" {
		lines = append(lines, styles.Badge.Render("PRÉVIA")+"  "+styles.FileStyle.Render(m.restore.computer))
		start := min(m.restore.offset, max(0, len(m.restore.plan.Entries)-rows))
		for _, entry := range m.restore.plan.Entries[start:min(len(m.restore.plan.Entries), start+rows)] {
			action := "criar"
			if entry.Replace {
				action = "substituir (guardar cópia)"
			}
			lines = append(lines, styles.FileStyle.Render(entry.Name)+"  "+styles.Muted.Render(action))
		}
		lines = append(lines, fmt.Sprintf("%d entradas • ↑/↓ rolar", len(m.restore.plan.Entries)),
			styles.TipStyle.Render("i confirmar instalação")+"  "+styles.Muted.Render("esc voltar"))
	} else {
		if len(m.restore.items) == 0 && !m.syncing {
			lines = append(lines, styles.Muted.Render("Nenhum backup por aqui ainda."), styles.TipStyle.Render("s enviar o primeiro backup • r atualizar"))
		}
		start := max(0, m.restore.selected-rows+1)
		for index := start; index < min(len(m.restore.items), start+rows); index++ {
			item := m.restore.items[index]
			cursor := "  "
			if index == m.restore.selected {
				cursor = "› "
			}
			date := "data indisponível"
			if !item.Modified.IsZero() {
				date = item.Modified.Local().Format("02/01/2006 15:04:05 MST")
			}
			row := fmt.Sprintf("%s%s  ·  %s  ·  %.1f KiB", cursor, item.Computer, date, float64(item.Size)/1024)
			if index == m.restore.selected {
				row = styles.Selected.Render(row)
			} else {
				row = styles.TextStyle.Render(row)
			}
			lines = append(lines, row)
		}
		lines = append(lines, "", styles.TipStyle.Render("enter ver prévia")+"  "+styles.Muted.Render("↑/↓ selecionar • r atualizar"))
	}
	if m.restore.recovery != "" {
		lines = append(lines, styles.TipStyle.Render("Cópia preservada")+"\n"+styles.Muted.Render(m.restore.recovery))
	}
	if m.lastError != "" {
		lines = append(lines, styles.ErrorStyle.Render("Erro: "+m.lastError))
	}
	return strings.Join(lines, "\n") + "\n"
}
