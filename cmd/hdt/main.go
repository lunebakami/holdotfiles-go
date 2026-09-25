package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lunebakami/holdotfiles-go/internal/backup"
	"github.com/lunebakami/holdotfiles-go/internal/config"
	"github.com/lunebakami/holdotfiles-go/internal/storage"
	"github.com/lunebakami/holdotfiles-go/internal/ui/models"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	list := flag.Bool("list", false, "listar computadores com backup ZIP")
	restore := flag.String("restore", "", "computador cujo backup será instalado")
	apply := flag.Bool("apply", false, "instalar após revisar a prévia")
	home, _ := os.UserHomeDir()
	dest := flag.String("dest", home, "diretório de destino da restauração")
	send := flag.Bool("backup", false, "enviar ZIP sem abrir a interface")
	recover := flag.String("recover", "", "pasta local de recuperação (prévia; use --apply para recuperar)")
	recoveries := flag.Bool("recoveries", false, "listar cópias locais de recuperação")
	flag.Parse()
	modes := 0
	for _, enabled := range []bool{*list, *send, *restore != "", *recover != "", *recoveries} {
		if enabled {
			modes++
		}
	}
	if modes > 1 {
		return fmt.Errorf("use apenas um modo: --list, --backup, --restore, --recover ou --recoveries")
	}
	if *apply && *restore == "" && *recover == "" {
		return fmt.Errorf("--apply requer --restore ou --recover")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	// Local recovery must work offline and without R2 credentials or .hdtconfig.
	if *recoveries {
		names, err := backup.ListRecoveries(*dest)
		if err != nil {
			return err
		}
		for _, name := range names {
			fmt.Println(name)
		}
		if len(names) == 0 {
			fmt.Println("Nenhuma cópia local de recuperação encontrada")
		}
		return nil
	}
	if *recover != "" {
		archive, err := backup.RecoveryArchive(ctx, *recover, *dest)
		if err != nil {
			return err
		}
		defer os.Remove(archive)
		return installArchive(ctx, archive, *dest, *apply, "Recuperação concluída.")
	}
	cfg, err := config.LoadForBackup(*send)
	if err != nil {
		return fmt.Errorf("configuração: %w", err)
	}
	r2, err := storage.NewR2(ctx, cfg.R2)
	if err != nil {
		return fmt.Errorf("iniciar R2: %w", err)
	}
	if *list {
		backups, err := r2.ListBackupDetails(ctx)
		if err != nil {
			return err
		}
		for _, item := range backups {
			fmt.Printf("%s\t%s\t%d bytes\n", item.Computer, item.Modified.Local().Format("02/01/2006 15:04:05 MST"), item.Size)
		}
		if len(backups) == 0 {
			fmt.Println("Nenhum backup ZIP encontrado")
		}
		return nil
	}
	if *restore != "" {
		archive, err := r2.Download(ctx, *restore)
		if err != nil {
			return err
		}
		defer os.Remove(archive)
		return installArchive(ctx, archive, *dest, *apply, "Instalação concluída.")
	}
	if *send {
		result, err := r2.Sync(ctx, cfg.Paths)
		fmt.Printf("ZIPs enviados: %d; inalterados: %d; falhas: %d\n", result.Uploaded, result.Skipped, result.Failed)
		return err
	}

	paths, pathsErr := config.ReadPaths(cfg.ConfigFile, home)
	model := models.NewAppModel(r2, paths, cfg.ConfigFile)
	model.SetRestoreDestination(*dest)
	if pathsErr != nil {
		model.SetConfigError(pathsErr)
	}
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func installArchive(ctx context.Context, archive, dest string, apply bool, success string) error {
	plan, err := backup.Preview(archive, dest)
	if err != nil {
		return err
	}
	for _, entry := range plan.Entries {
		action := "criar"
		if entry.Replace {
			action = "substituir (com cópia)"
		}
		fmt.Printf("%s: %s\n", action, entry.Name)
	}
	if !apply {
		fmt.Println("Prévia concluída. Use --apply para aplicar.")
		return nil
	}
	recovery, err := backup.Install(ctx, archive, dest)
	if recovery != "" {
		fmt.Println("Arquivos anteriores preservados em:", recovery)
	}
	if err != nil {
		return err
	}
	fmt.Println(success)
	return nil
}
