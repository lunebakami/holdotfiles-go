package models

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lunebakami/holdotfiles-go/internal/backup"
	"github.com/lunebakami/holdotfiles-go/internal/storage"
)

type restoreFake struct {
	fakeSyncer
	archive string
	wantKey string
}

func (f restoreFake) ListBackupDetails(ctx context.Context) ([]storage.BackupInfo, error) {
	return []storage.BackupInfo{{Computer: "laptop", Modified: time.Date(2026, 9, 24, 12, 30, 0, 0, time.UTC), Size: 123}}, ctx.Err()
}
func (f restoreFake) Download(ctx context.Context, computer string) (string, error) {
	if f.wantKey != "" && computer != f.wantKey {
		return "", fmt.Errorf("versão incorreta: %s", computer)
	}
	return f.archive, ctx.Err()
}

func TestRestoreUsesSelectedVersionKey(t *testing.T) {
	const key = "laptop/backup-2026-09-25T12-00-00.000000001Z.zip"
	m := NewAppModel(restoreFake{wantKey: key}, nil, "missing")
	m.state = stateRestore
	m.restore.items = []storage.BackupInfo{{Computer: "laptop", Key: key}}
	m, cmd := press(m, "enter")
	batch := cmd().(tea.BatchMsg)
	msg := batch[1]().(previewReadyMsg)
	if msg.computer != key || (msg.err != nil && strings.Contains(msg.err.Error(), "versão incorreta")) {
		t.Fatalf("versão errada: %+v", msg)
	}
}

func press(m AppModel, key string) (AppModel, tea.Cmd) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	if key == "enter" {
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	}
	model, cmd := m.Update(msg)
	return model.(AppModel), cmd
}
func completeOperation(t *testing.T, m AppModel, cmd tea.Cmd) AppModel {
	t.Helper()
	if cmd == nil {
		t.Fatal("comando assíncrono ausente")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatal("operação deve rodar fora do Update")
	}
	updated, _ := m.Update(batch[1]())
	return updated.(AppModel)
}

func TestRestoreSelectionPreviewAndConfirmation(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	filename := filepath.Join(source, "kitty.conf")
	if err := os.WriteFile(filename, []byte("novo"), 0600); err != nil {
		t.Fatal(err)
	}
	archive, err := backup.Pack(context.Background(), source, []string{filename})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(archive)
	target := filepath.Join(dest, "kitty.conf")
	if err = os.WriteFile(target, []byte("anterior"), 0600); err != nil {
		t.Fatal(err)
	}
	m := NewAppModel(restoreFake{archive: archive}, nil, "missing")
	m.SetRestoreDestination(dest)
	m, cmd := press(m, "r")
	m = completeOperation(t, m, cmd)
	if !strings.Contains(m.View(), "laptop") || !strings.Contains(m.View(), "24/09/2026") {
		t.Fatal("lista sem nome e data")
	}
	if _, cmd := press(m, "i"); cmd != nil {
		t.Fatal("instalação permitida sem prévia")
	}
	m, cmd = press(m, "enter")
	m = completeOperation(t, m, cmd)
	if !strings.Contains(m.View(), "substituir (guardar cópia)") {
		t.Fatal("prévia ausente")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "anterior" {
		t.Fatal("prévia alterou o destino")
	}
	m, cmd = press(m, "i")
	m = completeOperation(t, m, cmd)
	got, _ = os.ReadFile(target)
	if string(got) != "novo" {
		t.Fatal("instalação falhou")
	}
	got, err = os.ReadFile(filepath.Join(m.restore.recovery, "kitty.conf"))
	if err != nil || string(got) != "anterior" {
		t.Fatal("cópia anterior não preservada")
	}
	if _, err = os.Stat(archive); !os.IsNotExist(err) {
		t.Fatal("ZIP temporário não foi removido")
	}
	if m.syncing || m.restore.archive != "" {
		t.Fatal("estado não finalizado")
	}
}

func TestQuitDuringDownloadCleansArchive(t *testing.T) {
	archive, err := os.CreateTemp(t.TempDir(), "download-")
	if err != nil {
		t.Fatal(err)
	}
	archive.Close()
	m := NewAppModel(restoreFake{}, nil, "")
	m.beginRestore("download")
	m, cmd := press(m, "q")
	if cmd != nil || !m.quitting {
		t.Fatal("deve aguardar cancelamento antes de sair")
	}
	updated, cmd := m.Update(previewReadyMsg{archive: archive.Name(), err: context.Canceled})
	m = updated.(AppModel)
	if cmd == nil || m.syncing {
		t.Fatal("não encerrou após cancelar")
	}
	if _, err = os.Stat(archive.Name()); !os.IsNotExist(err) {
		t.Fatal("arquivo temporário vazou")
	}
}
