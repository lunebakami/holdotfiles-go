package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "tester")
	got, err := ExpandPath("~/.config/app", home)
	if err != nil {
		t.Fatalf("ExpandPath retornou erro: %v", err)
	}
	want := filepath.Join(home, ".config", "app")
	if got != want {
		t.Fatalf("ExpandPath = %q; esperado %q", got, want)
	}

	if _, err := ExpandPath("~outra-pessoa/config", home); err == nil {
		t.Fatal("ExpandPath deveria rejeitar outro usuário")
	}
}

func TestReadPathsIgnoresCommentsBlankLinesAndDuplicates(t *testing.T) {
	home := t.TempDir()
	configFile := filepath.Join(t.TempDir(), "hdtconfig")
	contents := strings.Join([]string{
		"# comentário",
		"",
		" ~/.zshrc ",
		"~/.zshrc",
		"~/.config/app",
	}, "\n")
	if err := os.WriteFile(configFile, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ReadPaths(configFile, home)
	if err != nil {
		t.Fatalf("ReadPaths retornou erro: %v", err)
	}
	want := []string{filepath.Join(home, ".zshrc"), filepath.Join(home, ".config", "app")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadPaths = %#v; esperado %#v", got, want)
	}
}

func TestReadPathsRejectsEmptyConfig(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "hdtconfig")
	if err := os.WriteFile(configFile, []byte("# apenas comentário\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPaths(configFile, t.TempDir()); err == nil {
		t.Fatal("ReadPaths deveria rejeitar configuração vazia")
	}
}

func TestValidateR2(t *testing.T) {
	valid := R2{
		Endpoint:        "https://account.r2.cloudflarestorage.com",
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		Bucket:          "bucket",
	}
	if err := validateR2(valid); err != nil {
		t.Fatalf("configuração válida foi rejeitada: %v", err)
	}

	valid.Endpoint = "http://inseguro.example.com"
	if err := validateR2(valid); err == nil {
		t.Fatal("endpoint sem HTTPS deveria ser rejeitado")
	}
}

func TestGlobalConfigAndEnvironmentPrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("R2_ENDPOINT", "https://test.r2.cloudflarestorage.com")
	t.Setenv("R2_ACCESS_KEY_ID", "test")
	t.Setenv("R2_SECRET_ACCESS_KEY", "test")
	t.Setenv("R2_BUCKET", "environment-bucket")
	t.Setenv("R2_PREFIX", "test")
	filename := filepath.Join(dir, "holdotfiles", ".env")
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte("R2_BUCKET=global-bucket\nHDT_GLOBAL_TEST=loaded\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// godotenv must not replace values already exported.
	t.Setenv("HDT_GLOBAL_TEST", "")
	os.Unsetenv("HDT_GLOBAL_TEST")
	cfg, err := LoadForBackup(false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.R2.Bucket != "environment-bucket" || os.Getenv("HDT_GLOBAL_TEST") != "loaded" {
		t.Fatal("precedência da configuração incorreta")
	}
}
