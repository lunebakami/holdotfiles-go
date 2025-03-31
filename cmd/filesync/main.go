package main

import (
  "fmt"
  "os"

  tea "github.com/charmbracelet/bubbletea"
  "github.com/lunebakami/holdotfiles-go/internal/ui/models"
)

func main() {
  p := tea.NewProgram(models.NewAppModel(), tea.WithAltScreen())

  if _, err := p.Run(); err != nil {
    fmt.Printf("Error running program: %v\n", err)
    os.Exit(1)
  }
}
