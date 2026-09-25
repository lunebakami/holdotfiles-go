package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const recoveryPrefix = ".holdotfiles-recovery-"

// ListRecoveries lists local recovery directories without following symlinks.
func ListRecoveries(dest string) ([]string, error) {
	dest, err := filepath.Abs(dest)
	if err != nil {
		return nil, err
	}
	if err = safeParents(dest); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), recoveryPrefix) {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

// RecoveryArchive snapshots the saved files, leaving the original copy intact.
// Only a direct recovery directory inside dest may be selected.
func RecoveryArchive(ctx context.Context, selected, dest string) (string, error) {
	dest, err := filepath.Abs(dest)
	if err != nil {
		return "", err
	}
	source := selected
	if !filepath.IsAbs(source) {
		source = filepath.Join(dest, source)
	}
	source = filepath.Clean(source)
	if filepath.Dir(source) != dest || !strings.HasPrefix(filepath.Base(source), recoveryPrefix) {
		return "", fmt.Errorf("selecione uma pasta %s* diretamente dentro de %s", recoveryPrefix, dest)
	}
	if err = safeParents(source); err != nil {
		return "", err
	}
	info, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("a recuperação selecionada não é um diretório")
	}
	files := 0
	err = filepath.Walk(source, func(p string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if e = ctx.Err(); e != nil {
			return e
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("link não permitido na cópia de recuperação: %s", p)
		}
		if info.Mode().IsRegular() {
			files++
		}
		// Recovery files must never overwrite another recovery directory.
		rel, e := filepath.Rel(source, p)
		if e != nil {
			return e
		}
		first := strings.Split(rel, string(filepath.Separator))[0]
		if strings.HasPrefix(first, recoveryPrefix) {
			return fmt.Errorf("pasta de recuperação aninhada: %s", rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if files == 0 {
		return "", fmt.Errorf("a cópia selecionada não contém arquivos para recuperar")
	}
	return Pack(ctx, source, []string{source})
}
