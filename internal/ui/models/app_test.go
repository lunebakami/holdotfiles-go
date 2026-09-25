package models

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lunebakami/holdotfiles-go/internal/storage"
)

type fakeSyncer struct{}

func (fakeSyncer) Sync(context.Context, []string) (storage.Result, error) {
	return storage.Result{Uploaded: 1}, nil
}

func (fakeSyncer) Target() string { return "r2://bucket/host" }

func TestSyncFinishedUpdatesStatus(t *testing.T) {
	model := NewAppModel(fakeSyncer{}, []string{"/tmp/file"}, "/tmp/config")
	updated, _ := model.Update(syncFinishedMsg{result: storage.Result{Uploaded: 1, Skipped: 2}})
	got := updated.(AppModel)
	if !strings.Contains(got.syncStatus, "1 enviados, 2 inalterados, 0 falhas") {
		t.Fatalf("status inesperado: %q", got.syncStatus)
	}
	if got.syncing {
		t.Fatal("modelo continuou marcado como sincronizando")
	}
}

func TestSyncErrorIsVisible(t *testing.T) {
	model := NewAppModel(fakeSyncer{}, []string{"/tmp/file"}, "/tmp/config")
	model.state = stateSync
	updated, _ := model.Update(syncFinishedMsg{
		result: storage.Result{Failed: 1},
		err:    errors.New("credenciais inválidas"),
	})
	got := updated.(AppModel)
	if !strings.Contains(got.View(), "credenciais inválidas") {
		t.Fatalf("erro não apareceu na interface: %q", got.View())
	}
}

func TestShortenUsesRuneLimit(t *testing.T) {
	if got := shorten("configuração", 7); got != "config…" {
		t.Fatalf("shorten = %q", got)
	}
}
