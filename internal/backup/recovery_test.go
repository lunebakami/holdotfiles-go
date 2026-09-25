package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoveryPreservesBothVersions(t *testing.T) {
	ctx := context.Background()
	dest := t.TempDir()
	saved := filepath.Join(dest, recoveryPrefix+"test")
	name := filepath.Join(".config", "nvim", "init.lua")
	write := func(p, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	check := func(p, want string) {
		t.Helper()
		got, err := os.ReadFile(p)
		if err != nil || string(got) != want {
			t.Fatalf("%s: %q, %v", p, got, err)
		}
	}
	write(filepath.Join(saved, name), "anterior")
	write(filepath.Join(dest, name), "atual")
	write(filepath.Join(dest, "novo"), "manter")
	names, err := ListRecoveries(dest)
	if err != nil || len(names) != 1 || names[0] != filepath.Base(saved) {
		t.Fatalf("listagem: %v %v", names, err)
	}
	archive, err := RecoveryArchive(ctx, filepath.Base(saved), dest)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(archive)
	plan, err := Preview(archive, dest)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range plan.Entries {
		if e.Name == filepath.ToSlash(name) && e.Replace {
			found = true
		}
	}
	if !found {
		t.Fatal("prévia sem substituição")
	}
	check(filepath.Join(dest, name), "atual")
	recovery, err := Install(ctx, archive, dest)
	if err != nil {
		t.Fatal(err)
	}
	check(filepath.Join(dest, name), "anterior")
	check(filepath.Join(saved, name), "anterior")
	check(filepath.Join(recovery, name), "atual")
	check(filepath.Join(dest, "novo"), "manter")
	reverse, err := RecoveryArchive(ctx, recovery, dest)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(reverse)
	if _, err = Install(ctx, reverse, dest); err != nil {
		t.Fatal(err)
	}
	check(filepath.Join(dest, name), "atual")
}

func TestRecoveryRejectsInvalidSources(t *testing.T) {
	dest := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(dest, recoveryPrefix+"link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dest, recoveryPrefix+"empty")
	if err := os.Mkdir(empty, 0700); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"../outside", outside, link, empty, dest} {
		if archive, err := RecoveryArchive(context.Background(), source, dest); err == nil {
			os.Remove(archive)
			t.Fatalf("origem inválida aceita: %s", source)
		}
	}
}
