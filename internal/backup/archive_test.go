package backup

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".config/nvim/empty"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{".config/nvim/init.lua": "return {}", "kitty.conf": "font_size 12"} {
		if err := os.WriteFile(filepath.Join(home, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	archive, err := Pack(context.Background(), home, []string{filepath.Join(home, ".config/nvim"), filepath.Join(home, "kitty.conf")})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(archive)
	dest := t.TempDir()
	os.WriteFile(filepath.Join(dest, "kitty.conf"), []byte("old"), 0600)
	plan, err := Preview(archive, dest)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range plan.Entries {
		if e.Name == "kitty.conf" && e.Replace {
			found = true
		}
	}
	if !found {
		t.Fatal("prévia não indicou substituição")
	}
	recovery, err := Install(context.Background(), archive, dest)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{".config/nvim/init.lua": "return {}", "kitty.conf": "font_size 12"} {
		got, err := os.ReadFile(filepath.Join(dest, name))
		if err != nil || string(got) != want {
			t.Fatalf("restauração de %s: %v", name, err)
		}
	}
	old, err := os.ReadFile(filepath.Join(recovery, "kitty.conf"))
	if err != nil || string(old) != "old" {
		t.Fatal("cópia anterior perdida")
	}
	if _, err := os.Stat(filepath.Join(dest, ".config/nvim/empty")); err != nil {
		t.Fatal(err)
	}
}

func TestRejectTraversalAndSymlinks(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", ".config/../../escape", "a\\b"} {
		t.Run(name, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "evil.zip")
			out, _ := os.Create(filename)
			zw := zip.NewWriter(out)
			w, _ := zw.Create(name)
			w.Write([]byte("bad"))
			zw.Close()
			out.Close()
			if _, err := Install(context.Background(), filename, t.TempDir()); err == nil {
				t.Fatal("caminho perigoso aceito")
			}
		})
	}
	home := t.TempDir()
	outside := t.TempDir()
	os.WriteFile(filepath.Join(home, "file"), []byte("ok"), 0600)
	archive, err := Pack(context.Background(), home, []string{filepath.Join(home, "file")})
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(archive)
	dest := t.TempDir()
	os.Symlink(filepath.Join(outside, "file"), filepath.Join(dest, "file"))
	if _, err := Install(context.Background(), archive, dest); err == nil {
		t.Fatal("link de destino aceito")
	}
	if _, err := Pack(context.Background(), home, []string{outside}); err == nil {
		t.Fatal("origem externa aceita")
	}
}

func TestPackFileSymlinks(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run(fmt.Sprint("direct=", direct), func(t *testing.T) {
			home := t.TempDir()
			folder := filepath.Join(home, ".config", "nvim", "lua", "plugins")
			if err := os.MkdirAll(folder, 0700); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(t.TempDir(), "neovim.lua")
			if err := os.WriteFile(target, []byte("return { theme = 'test' }"), 0640); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(folder, "theme.lua")
			relative, err := filepath.Rel(folder, target)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.Symlink(relative, link); err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(home, ".config")
			if direct {
				root = link
			}
			archive, err := Pack(context.Background(), home, []string{root})
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(archive)
			dest := t.TempDir()
			if _, err = Install(context.Background(), archive, dest); err != nil {
				t.Fatal(err)
			}
			installed := filepath.Join(dest, ".config", "nvim", "lua", "plugins", "theme.lua")
			info, err := os.Lstat(installed)
			if err != nil {
				t.Fatal(err)
			}
			if !info.Mode().IsRegular() || info.Mode().Perm() != 0640 {
				t.Fatalf("modo incorreto: %v", info.Mode())
			}
			got, err := os.ReadFile(installed)
			if err != nil || string(got) != "return { theme = 'test' }" {
				t.Fatalf("conteúdo incorreto: %q %v", got, err)
			}
		})
	}
}

func TestPackRejectsBrokenAndDirectoryLinks(t *testing.T) {
	for _, kind := range []string{"broken", "directory", "cycle"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			link := filepath.Join(home, "link")
			target := filepath.Join(home, "missing")
			if kind == "directory" {
				target = t.TempDir()
			}
			if kind == "cycle" {
				target = link
			}
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			archive, err := Pack(context.Background(), home, []string{link})
			if err == nil {
				os.Remove(archive)
				t.Fatal("link inválido aceito")
			}
		})
	}
}
