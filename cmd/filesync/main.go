package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lunebakami/holdotfiles-go/cmd/lib"
	"github.com/lunebakami/holdotfiles-go/internal/ui/models"
)

func main() {
	appwrite, err := lib.InitAppwrite()
	if err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(models.NewAppModel(appwrite), tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
