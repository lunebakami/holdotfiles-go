package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
	"github.com/lunebakami/holdotfiles-go/internal/backup"
	"github.com/lunebakami/holdotfiles-go/internal/config"
)

func TestR2Live(t *testing.T) {
	if os.Getenv("HOLDOTFILES_LIVE_TEST") != "1" {
		t.Skip("teste remoto opt-in")
	}
	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatal("não foi possível carregar .env")
	}
	dir := t.TempDir()
	filename := filepath.Join(dir, "probe.txt")
	const body = "Holdotfiles R2 integration probe\n"
	if err := os.WriteFile(filename, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(dir, "config")
	if err := os.WriteFile(list, []byte(filename+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOLDOTFILES_CONFIG", list)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("configuração R2 inválida; revise os campos obrigatórios")
	}
	cfg.R2.Prefix = "holdotfiles-integration-test/" + filepath.Base(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	remote, err := NewR2(ctx, cfg.R2)
	if err != nil {
		t.Fatal("falha ao iniciar cliente")
	}
	client := remote.client.(*s3.Client)
	remote.home = dir
	key := remote.ArchiveKey()
	// Remove exclusivamente o objeto sintético criado por este teste.
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		if _, err := client.DeleteObject(cleanupCtx, &s3.DeleteObjectInput{Bucket: aws.String(cfg.R2.Bucket), Key: aws.String(key)}); err != nil {
			t.Error("falha ao remover objeto temporário")
		} else {
			t.Log("objeto temporário removido")
		}
	}()
	first, err := remote.Sync(ctx, []string{filename})
	if err != nil {
		t.Fatalf("upload falhou: %v", err)
	}
	if first.Uploaded != 1 || first.Failed != 0 {
		t.Fatalf("resultado de upload inesperado: %+v", first)
	}
	t.Log("upload confirmado")
	second, err := remote.Sync(ctx, []string{filename})
	if err != nil || second.Skipped != 1 || second.Uploaded != 0 {
		t.Fatal("verificação incremental falhou")
	}
	t.Log("arquivo inalterado reconhecido")
	archive, err := remote.Download(ctx, cfg.R2.Prefix)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(archive)
	dest := t.TempDir()
	if _, err = backup.Install(ctx, archive, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "probe.txt"))
	if err != nil || string(got) != body {
		t.Fatal("conteúdo remoto divergente")
	}
	t.Log("download e integridade confirmados")
}
